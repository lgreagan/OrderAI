package services

import (
	"context"
	"sort"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// FinanceService 生成财务报表（收入、成本、利润）。
type FinanceService struct {
	Repo     *repository.FinanceRepository
	Settings *SettingsService
}

func NewFinanceService(
	repo *repository.FinanceRepository,
	settings *SettingsService,
) *FinanceService {
	return &FinanceService{Repo: repo, Settings: settings}
}

// Report 汇总订单利润、月度利润与 PI 应收。
//
// 收入按 PI 明细（货款 + 箱费）归集到客户订单，按 PI 签发月份的汇率折算成人民币
// （月度汇率来自系统设置，未维护的月份回落到默认汇率并在报表中标注）。
// 成本同时给出两个口径：
//   - 标准成本：数量 × 主数据成本价（skus.price / materials.price）
//   - 实际成本：该订单采购单合计（purchase_orders.total_amount，不含已取消）
func (s *FinanceService) Report(
	ctx context.Context,
	filter models.FinanceFilter,
) (*models.FinanceReport, error) {
	defaultRate := DefaultExchangeRate
	monthRates := map[string]float64{}

	if s.Settings != nil {
		defaultRate = s.Settings.ExchangeRate(ctx)

		rates, err := s.Settings.MonthlyExchangeRates(ctx)
		if err != nil {
			return nil, err
		}
		monthRates = rates
	}

	orderRows, err := s.Repo.OrderRows(ctx, filter)
	if err != nil {
		return nil, err
	}
	unlinked, err := s.Repo.UnlinkedRows(ctx, filter)
	if err != nil {
		return nil, err
	}
	purchases, err := s.Repo.PurchaseCosts(ctx)
	if err != nil {
		return nil, err
	}
	shipmentCharges, err := s.Repo.ShipmentCharges(ctx)
	if err != nil {
		return nil, err
	}
	shipmentCosts := convertShipmentCharges(shipmentCharges, defaultRate, monthRates)
	piSummary, err := s.Repo.PISummary(ctx, filter)
	if err != nil {
		return nil, err
	}

	shipped, err := s.Repo.ShippedItemByOrder(ctx, filter)
	if err != nil {
		return nil, err
	}
	purchased, err := s.Repo.PurchasedItemByOrder(ctx)
	if err != nil {
		return nil, err
	}

	rows := append(orderRows, unlinked...)
	fillProfit(rows, defaultRate, monthRates, purchases, shipmentCosts, shipped, purchased)
	rows = dropEmptyRows(rows)

	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i].IssuedDate.Time, rows[j].IssuedDate.Time
		if left.Equal(right) {
			return rows[i].OrderID > rows[j].OrderID
		}
		return left.After(right)
	})

	return &models.FinanceReport{
		Rows:      rows,
		Monthly:   monthlyRows(rows),
		Summary:   summarize(rows, defaultRate, piSummary),
		Shortages: buildShortages(rows, shipped, purchased),
	}, nil
}

// convertShipmentCharges 把各发货单录入的运费/保险折算成美元并按订单汇总。
//
// 与 PI 侧口径对齐：PI 里的运费/保险行是「发货单金额 ÷ 签发月汇率」带过来的，
// 归属栏里的「发货实付」必须同样折成美元，否则币种是人民币的发货单会凭空产生差额。
// 折算汇率取发货日期所在月份（该月未维护则回落到默认汇率），空币种按美元处理。
func convertShipmentCharges(
	charges []models.FinanceShipmentCharge,
	defaultRate float64,
	monthRates map[string]float64,
) map[uint64]models.FinanceShipmentCost {
	result := map[uint64]models.FinanceShipmentCost{}

	for _, c := range charges {
		rate := 1.0
		if isCNY(c.Currency) {
			if r, _ := resolveRate(c.ShipDate, monthRates, defaultRate); r > 0 {
				rate = r
			}
		}

		it := result[c.OrderID]
		it.OrderID = c.OrderID
		it.FreightUSD += c.FreightCost / rate
		it.InsuranceUSD += c.InsuranceCost / rate
		result[c.OrderID] = it
	}

	for id, it := range result {
		it.FreightUSD = round2(it.FreightUSD)
		it.InsuranceUSD = round2(it.InsuranceUSD)
		result[id] = it
	}

	return result
}

// isCNY 判断发货单币种是否为人民币（录入值可能是 CNY / RMB / ¥ 等写法）。
func isCNY(currency string) bool {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "CNY", "RMB", "CNH", "¥", "￥", "人民币":
		return true
	}
	return false
}

// resolveRate 取某行签发月份的汇率；该月未维护月度汇率时回落到默认汇率并标记。
func resolveRate(
	issued models.Date,
	monthRates map[string]float64,
	defaultRate float64,
) (float64, bool) {
	if issued.Time.IsZero() {
		return defaultRate, true
	}

	month := issued.Time.Format("2006-01")
	if rate, ok := monthRates[month]; ok {
		return rate, false
	}

	return defaultRate, true
}

// hasActualCost 表示该行的实际采购成本口径可用：
// 已关联客户订单、有采购单，且采购单数量覆盖了 PI 出货数量。
func hasActualCost(it *models.FinanceOrderRow) bool {
	return it.OrderID != 0 && !it.NoPurchase && !it.PurchaseIncomplete
}

// purchaseCovered 判断该订单的采购单是否覆盖 PI 出货的每个 SKU。
//
// 发货可能动用历史库存，或采购单只填了系统建议数量的一部分，
// 这两种情况下「采购单合计」都不是该订单的真实成本，需要排除在实际口径外。
func purchaseCovered(
	orderID uint64,
	shipped map[uint64]models.FinanceShippedItem,
	purchased map[uint64]map[uint64]models.FinancePurchasedItem,
) bool {
	if len(shipped) == 0 {
		return true
	}

	bought := purchased[orderID]
	for skuID, item := range shipped {
		if bought[skuID].Quantity+1e-6 < item.Quantity {
			return false
		}
	}

	return true
}

// buildShortages 列出实际成本口径无法覆盖的订单（未采购 / 采购未齐）缺哪些 SKU、差多少数量。
//
// 缺口 = PI 出货数量 − 已采购数量，采购照着这张表补采购单数量即可。
func buildShortages(
	rows []models.FinanceOrderRow,
	shipped map[uint64]map[uint64]models.FinanceShippedItem,
	purchased map[uint64]map[uint64]models.FinancePurchasedItem,
) []models.FinanceShortageRow {
	result := make([]models.FinanceShortageRow, 0)

	for i := range rows {
		it := &rows[i]
		if it.OrderID == 0 || hasActualCost(it) {
			continue
		}

		orderShipped := shipped[it.OrderID]
		orderPurchased := purchased[it.OrderID]

		gaps := make([]models.FinanceShortageRow, 0, len(orderShipped))
		for skuID, item := range orderShipped {
			bought := orderPurchased[skuID]
			shortage := round2(item.Quantity - bought.Quantity)
			if shortage <= 0 {
				continue
			}

			gaps = append(gaps, models.FinanceShortageRow{
				OrderID:      it.OrderID,
				OrderNumber:  it.OrderNumber,
				Customer:     it.Customer,
				PINumbers:    it.PINumbers,
				SkuID:        skuID,
				SkuCode:      item.SkuCode,
				SkuName:      item.SkuName,
				Spec:         item.Spec,
				Unit:         item.Unit,
				ShippedQty:   round2(item.Quantity),
				PurchasedQty: round2(bought.Quantity),
				ShortageQty:  shortage,
				PONumbers:    bought.PONumbers,
			})
		}

		// 同一订单内缺口大的排前面，便于先补主要商品。
		sort.Slice(gaps, func(a, b int) bool {
			return gaps[a].ShortageQty > gaps[b].ShortageQty
		})
		result = append(result, gaps...)
	}

	return result
}

// dropEmptyRows 去掉没有任何金额的行（例如金额为 0 的手工费用行）。
func dropEmptyRows(rows []models.FinanceOrderRow) []models.FinanceOrderRow {
	result := make([]models.FinanceOrderRow, 0, len(rows))
	for _, it := range rows {
		if it.GoodsUSD == 0 && it.BoxUSD == 0 && it.FreightUSD == 0 &&
			it.InsuranceUSD == 0 && it.StdCostCNY == 0 && it.ActualCostCNY == 0 {
			continue
		}
		result = append(result, it)
	}

	return result
}

// fillProfit 计算每行的折算汇率、收入、毛利与毛利率。
//
// 未下采购单的订单不给出实际口径毛利，避免把「成本为 0」当成 100% 毛利。
func fillProfit(
	rows []models.FinanceOrderRow,
	defaultRate float64,
	monthRates map[string]float64,
	purchases map[uint64]models.FinancePurchaseCost,
	shipmentCosts map[uint64]models.FinanceShipmentCost,
	shipped map[uint64]map[uint64]models.FinanceShippedItem,
	purchased map[uint64]map[uint64]models.FinancePurchasedItem,
) {
	for i := range rows {
		it := &rows[i]

		it.ExchangeRate, it.RateFallback = resolveRate(it.IssuedDate, monthRates, defaultRate)
		it.RevenueUSD = round2(it.GoodsUSD + it.BoxUSD)
		it.RevenueCNY = round2(it.RevenueUSD * it.ExchangeRate)
		it.StdCostCNY = round2(it.StdCostCNY)
		it.GrossStdCNY = round2(it.RevenueCNY - it.StdCostCNY)

		if cost, ok := purchases[it.OrderID]; ok && it.OrderID != 0 {
			it.ActualCostCNY = round2(cost.AmountCNY)
			it.PurchaseCount = cost.Count
			it.PurchaseOrders = cost.PONumbers

			if !purchaseCovered(it.OrderID, shipped[it.OrderID], purchased) {
				it.PurchaseIncomplete = true
			}
		} else if it.OrderID != 0 {
			it.NoPurchase = true
		}

		if hasActualCost(it) {
			it.GrossActualCNY = round2(it.RevenueCNY - it.ActualCostCNY)
		}

		if it.RevenueCNY != 0 {
			it.MarginStd = round4(it.GrossStdCNY / it.RevenueCNY)
			if hasActualCost(it) {
				it.MarginActual = round4(it.GrossActualCNY / it.RevenueCNY)
			}
		}

		if cost, ok := shipmentCosts[it.OrderID]; ok && it.OrderID != 0 {
			it.ShipFreightUSD = round2(cost.FreightUSD)
			it.ShipInsuranceUSD = round2(cost.InsuranceUSD)
		}
	}
}

// monthlyRows 按 PI 签发月份汇总。签发日期为空（例如未填日期的草稿）归入空月份。
//
// 每行已按各自月份的汇率折算，因此月度收入之和与汇总口径一致。
func monthlyRows(rows []models.FinanceOrderRow) []models.FinanceMonthlyRow {
	byMonth := map[string]*models.FinanceMonthlyRow{}

	for i := range rows {
		it := &rows[i]

		month := ""
		if !it.IssuedDate.Time.IsZero() {
			month = it.IssuedDate.Time.Format("2006-01")
		}

		item, ok := byMonth[month]
		if !ok {
			item = &models.FinanceMonthlyRow{
				Month:        month,
				ExchangeRate: it.ExchangeRate,
				RateFallback: it.RateFallback,
			}
			byMonth[month] = item
		}

		// 订单数只统计已关联客户订单的行，收入仍含未关联订单的 PI 明细，
		// 保证月度收入之和与汇总口径一致。
		if it.OrderID != 0 {
			item.OrderCount++
		}

		item.RevenueCNY += it.RevenueCNY
		item.StdCostCNY += it.StdCostCNY
		item.GrossStdCNY += it.GrossStdCNY

		if hasActualCost(it) {
			item.ActualOrderCount++
			item.ActualRevenueCNY += it.RevenueCNY
			item.ActualCostCNY += it.ActualCostCNY
			item.GrossActualCNY += it.GrossActualCNY
		}
	}

	result := make([]models.FinanceMonthlyRow, 0, len(byMonth))
	for _, item := range byMonth {
		item.RevenueCNY = round2(item.RevenueCNY)
		item.StdCostCNY = round2(item.StdCostCNY)
		item.ActualCostCNY = round2(item.ActualCostCNY)
		item.GrossStdCNY = round2(item.GrossStdCNY)
		item.GrossActualCNY = round2(item.GrossActualCNY)
		item.ActualRevenueCNY = round2(item.ActualRevenueCNY)

		if item.RevenueCNY != 0 {
			item.MarginStd = round4(item.GrossStdCNY / item.RevenueCNY)
		}
		if item.ActualRevenueCNY != 0 {
			item.MarginActual = round4(item.GrossActualCNY / item.ActualRevenueCNY)
		}
		result = append(result, *item)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Month == "" {
			return false
		}
		if result[j].Month == "" {
			return true
		}
		return result[i].Month > result[j].Month
	})

	return result
}

// summarize 汇总报表合计。OrderCount 只统计已关联客户订单的行。
//
// 各月已按各自汇率折算，所以这里直接相加；EffectiveRate 是反算出的加权平均汇率，
// FallbackMonths 列出本期未维护月度汇率、按默认汇率折算的月份。
func summarize(
	rows []models.FinanceOrderRow,
	defaultRate float64,
	piSummary *models.FinancePISummary,
) models.FinanceSummary {
	summary := models.FinanceSummary{DefaultExchangeRate: defaultRate}
	if piSummary != nil {
		summary.PI = *piSummary
	}

	fallbackMonths := map[string]bool{}

	for i := range rows {
		it := &rows[i]

		if it.OrderID != 0 {
			summary.OrderCount++
		}
		if it.NoPurchase {
			summary.NoPurchaseCount++
		}
		if it.PurchaseIncomplete {
			summary.IncompletePurchaseCount++
		}
		if it.RateFallback {
			month := ""
			if !it.IssuedDate.Time.IsZero() {
				month = it.IssuedDate.Time.Format("2006-01")
			}
			fallbackMonths[month] = true
		}

		summary.RevenueUSD += it.RevenueUSD
		summary.RevenueCNY += it.RevenueCNY
		summary.StdCostCNY += it.StdCostCNY
		summary.GrossStdCNY += it.GrossStdCNY
		summary.FreightUSD += it.FreightUSD
		summary.InsuranceUSD += it.InsuranceUSD
		summary.ShipFreightUSD += it.ShipFreightUSD
		summary.ShipInsuranceUSD += it.ShipInsuranceUSD
		summary.MissingGoodsCost += it.MissingGoodsCost
		summary.MissingBoxCost += it.MissingBoxCost

		if hasActualCost(it) {
			summary.ActualOrderCount++
			summary.ActualRevenueCNY += it.RevenueCNY
			summary.ActualCostCNY += it.ActualCostCNY
			summary.GrossActualCNY += it.GrossActualCNY
		}
	}

	summary.RevenueUSD = round2(summary.RevenueUSD)
	summary.RevenueCNY = round2(summary.RevenueCNY)
	summary.StdCostCNY = round2(summary.StdCostCNY)
	summary.ActualCostCNY = round2(summary.ActualCostCNY)
	summary.GrossStdCNY = round2(summary.GrossStdCNY)
	summary.GrossActualCNY = round2(summary.GrossActualCNY)
	summary.FreightUSD = round2(summary.FreightUSD)
	summary.InsuranceUSD = round2(summary.InsuranceUSD)
	summary.ShipFreightUSD = round2(summary.ShipFreightUSD)
	summary.ShipInsuranceUSD = round2(summary.ShipInsuranceUSD)
	summary.ActualRevenueCNY = round2(summary.ActualRevenueCNY)

	if summary.RevenueCNY != 0 {
		summary.MarginStd = round4(summary.GrossStdCNY / summary.RevenueCNY)
	}
	if summary.ActualRevenueCNY != 0 {
		summary.MarginActual = round4(summary.GrossActualCNY / summary.ActualRevenueCNY)
	}
	if summary.RevenueUSD != 0 {
		summary.EffectiveRate = round4(summary.RevenueCNY / summary.RevenueUSD)
	}

	summary.FallbackMonths = make([]string, 0, len(fallbackMonths))
	for month := range fallbackMonths {
		summary.FallbackMonths = append(summary.FallbackMonths, month)
	}
	sort.Strings(summary.FallbackMonths)

	return summary
}
