package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/repository"
)

// ProformaInvoiceService 负责按系统数据装配并保存形式发票（PI）。
type ProformaInvoiceService struct {
	Repo           *repository.ProformaInvoiceRepository
	Shipments      *repository.ShipmentRepository
	Orders         *repository.OrderRepository
	PurchaseOrders *repository.PurchaseOrderRepository
	Payments       *repository.PaymentRepository
	Inventory      *repository.InventoryRepository
	SKUs           *repository.SKURepository
	Materials      *repository.MaterialRepository
	Settings       *SettingsService
}

func NewProformaInvoiceService(
	repo *repository.ProformaInvoiceRepository,
	shipments *repository.ShipmentRepository,
	orders *repository.OrderRepository,
	purchaseOrders *repository.PurchaseOrderRepository,
	payments *repository.PaymentRepository,
	inventory *repository.InventoryRepository,
	skus *repository.SKURepository,
	materials *repository.MaterialRepository,
	settings *SettingsService,
) *ProformaInvoiceService {
	return &ProformaInvoiceService{
		Repo:           repo,
		Shipments:      shipments,
		Orders:         orders,
		PurchaseOrders: purchaseOrders,
		Payments:       payments,
		Inventory:      inventory,
		SKUs:           skus,
		Materials:      materials,
		Settings:       settings,
	}
}

func (s *ProformaInvoiceService) List(
	ctx context.Context,
) ([]models.ProformaInvoiceListItem, error) {
	return s.Repo.List(ctx)
}

func (s *ProformaInvoiceService) Get(
	ctx context.Context,
	id uint64,
) (*models.ProformaInvoice, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *ProformaInvoiceService) Delete(
	ctx context.Context,
	id uint64,
) error {
	if id == 0 {
		return fmt.Errorf("PI ID 无效")
	}
	return s.Repo.Delete(ctx, id)
}

// Generate 按订单/发货单装配一张 PI 草稿（不落库），供前端预览与编辑。
//
// 规则：
//   - 一发货单 = 一组；订单尚无发货单时，用订单匹配明细按需求数量兜底成一组。
//   - 分组标题 = {单号} ({箱数}ctns) {运输方式}。
//   - 发货单上的运费/保险各生成一条费用行。
func (s *ProformaInvoiceService) Generate(
	ctx context.Context,
	req *models.ProformaInvoiceGenerateRequest,
) (*models.ProformaInvoice, error) {
	orders, err := s.resolveOrders(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, fmt.Errorf("没有找到可生成 PI 的订单")
	}

	orderIDs := make([]uint64, 0, len(orders))
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}

	shipments, err := s.Shipments.ListByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}

	shipmentsByOrder := make(map[uint64][]models.Shipment)
	for _, sh := range shipments {
		shipmentsByOrder[sh.OrderID] = append(shipmentsByOrder[sh.OrderID], sh)
	}

	pi := &models.ProformaInvoice{
		CustomerName: req.CustomerName,
		Currency:     s.defaultCurrency(ctx),
		Status:       models.PIStatusDraft,
		IssuedDate:   models.Date{Time: time.Now()},
		Items:        make([]models.ProformaInvoiceItem, 0),
	}
	if pi.CustomerName == "" {
		pi.CustomerName = orders[0].Customer
	}

	number, err := s.Repo.NextNumber(ctx)
	if err != nil {
		return nil, err
	}
	pi.PINumber = number

	sortOrder := 0
	groupNo := 0

	for _, order := range orders {
		orderShipments := shipmentsByOrder[order.ID]

		if len(orderShipments) > 0 {
			for i := range orderShipments {
				sh := &orderShipments[i]
				groupNo++
				sortOrder++

				pi.Items = append(pi.Items, models.ProformaInvoiceItem{
					ItemType:   models.PIItemGroup,
					GroupNo:    groupNo,
					GroupTitle: shipmentGroupTitle(sh),
					OrderID:    uint64Ptr(order.ID),
					ShipmentID: uint64Ptr(sh.ID),
					SortOrder:  sortOrder,
				})

				for j := range sh.Items {
					it := &sh.Items[j]
					sortOrder++
					pi.Items = append(pi.Items, goodsFromShipmentItem(it, groupNo, order.ID, sh.ID, sortOrder))
				}

				sortOrder = s.appendShipmentCharges(ctx, pi, sh, order, sortOrder)
			}
			continue
		}

		// 兜底：订单还没有发货单时，用匹配明细按需求数量装配一组。
		lines, err := s.Shipments.ListOrderLines(ctx, order.ID)
		if err != nil {
			return nil, err
		}
		if len(lines) == 0 {
			continue
		}

		groupNo++
		sortOrder++

		pi.Items = append(pi.Items, models.ProformaInvoiceItem{
			ItemType:   models.PIItemGroup,
			GroupNo:    groupNo,
			GroupTitle: fallbackGroupTitle(order, lines),
			OrderID:    uint64Ptr(order.ID),
			SortOrder:  sortOrder,
		})

		for i := range lines {
			l := &lines[i]
			sortOrder++
			pi.Items = append(pi.Items, models.ProformaInvoiceItem{
				ItemType:    models.PIItemGoods,
				GroupNo:     groupNo,
				OrderID:     uint64Ptr(order.ID),
				SkuID:       l.SkuID,
				ShortName:   l.ShortName,
				Description: BuildShipmentDescription(l.ShortName, l.RequiredQuantity, l.PackQuantity),
				Quantity:    l.RequiredQuantity,
				Unit:        l.Unit,
				UnitPrice:   l.UnitPrice,
				Amount:      round2(l.UnitPrice * l.RequiredQuantity),
				SortOrder:   sortOrder,
			})
		}
	}

	if len(pi.Items) == 0 {
		return nil, fmt.Errorf("没有找到可生成 PI 的数据")
	}

	// Box orders：已到货的材料采购（关联到所选订单）自动带出箱费行。
	if s.PurchaseOrders != nil {
		boxItems, err := s.PurchaseOrders.ListReceivedMaterialItems(ctx, orderIDs)
		if err != nil {
			return nil, err
		}
		for i := range boxItems {
			it := &boxItems[i]
			sortOrder++
			pi.Items = append(pi.Items, models.ProformaInvoiceItem{
				ItemType:    models.PIItemBox,
				OrderID:     it.SourceOrderID,
				MaterialID:  it.ItemID,
				ShortName:   it.ShortName,
				Description: firstNonEmpty(it.ItemName, it.ShortName),
				Quantity:    it.ReceivedQuantity,
				Unit:        it.Unit,
				UnitPrice:   it.UnitPrice,
				Amount:      round2(it.UnitPrice * it.ReceivedQuantity),
				SortOrder:   sortOrder,
			})
		}
	}

	// 单价兜底：发货/采购明细没带价格时，按产品主数据补全（货款取 skus.price_usd）。
	s.fillItemPrices(ctx, pi)

	// 箱费：材料单价是人民币，按签发月份汇率折算成 PI 外币单价。
	s.convertBoxCostToUSD(ctx, pi)

	s.recalcTotals(pi)

	// 历史欠款 = 该客户历史 PI 本期合计 − 历史收款（收款超过开票额时按 0 计）。
	if pi.CustomerName != "" && s.Payments != nil {
		billed, err := s.Repo.SumAmountByCustomer(ctx, pi.CustomerName, 0)
		if err != nil {
			return nil, err
		}
		paid, err := s.Payments.SumByCustomer(ctx, pi.CustomerName)
		if err != nil {
			return nil, err
		}
		pi.CarryoverAmount = carryover(billed, paid)
	}
	pi.TotalUnpaid = round2(pi.Amount + pi.CarryoverAmount)

	// 库存说明草稿（系统里有库存的才写出，其余留空由业务员补填）。
	note, err := s.buildStockNote(ctx, pi)
	if err != nil {
		return nil, err
	}
	pi.StockNote = note

	return pi, nil
}

// defaultCurrency 默认币种（来自系统设置）。
func (s *ProformaInvoiceService) defaultCurrency(ctx context.Context) string {
	if s.Settings != nil {
		return s.Settings.DefaultCurrency(ctx)
	}
	return "USD"
}

// buildStockNote 依据库存结存生成 PI 的库存说明草稿。
func (s *ProformaInvoiceService) buildStockNote(
	ctx context.Context,
	pi *models.ProformaInvoice,
) (string, error) {
	if s.Inventory == nil {
		return "", nil
	}

	skuIDs := make([]uint64, 0)
	materialIDs := make([]uint64, 0)
	for i := range pi.Items {
		it := &pi.Items[i]
		if it.ItemType == models.PIItemGoods && it.SkuID != nil {
			skuIDs = append(skuIDs, *it.SkuID)
		}
		if it.ItemType == models.PIItemBox && it.MaterialID != nil {
			materialIDs = append(materialIDs, *it.MaterialID)
		}
	}

	skuStocks, err := s.Inventory.StockByIDs(ctx, models.InventoryItemSku, dedupeIDs(skuIDs))
	if err != nil {
		return "", err
	}
	materialStocks, err := s.Inventory.StockByIDs(ctx, models.InventoryItemMaterial, dedupeIDs(materialIDs))
	if err != nil {
		return "", err
	}

	type stockInfo struct {
		Name  string
		Stock float64
	}

	skuMap := make(map[uint64]stockInfo, len(skuStocks))
	for _, it := range skuStocks {
		skuMap[it.ItemID] = stockInfo{Name: firstNonEmpty(it.ItemName, it.ItemCode), Stock: it.StockQuantity}
	}
	materialMap := make(map[uint64]stockInfo, len(materialStocks))
	for _, it := range materialStocks {
		materialMap[it.ItemID] = stockInfo{Name: firstNonEmpty(it.ItemName, it.ItemCode), Stock: it.StockQuantity}
	}

	header := "当前库存"
	if s.Settings != nil {
		if h := strings.TrimSpace(s.Settings.StockNoteHeader(ctx)); h != "" {
			header = h
		}
	}

	var b strings.Builder
	b.WriteString(header)

	seen := make(map[string]bool)
	for i := range pi.Items {
		it := &pi.Items[i]

		var info stockInfo
		var ok bool

		switch it.ItemType {
		case models.PIItemGoods:
			if it.SkuID == nil {
				continue
			}
			info, ok = skuMap[*it.SkuID]
		case models.PIItemBox:
			if it.MaterialID == nil {
				continue
			}
			info, ok = materialMap[*it.MaterialID]
		default:
			continue
		}

		name := strings.TrimSpace(info.Name)
		if !ok {
			name = strings.TrimSpace(firstNonEmpty(it.ShortName, it.Description))
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		b.WriteString(fmt.Sprintf(
			"\n%s: %s %s",
			name, formatQty(round3(info.Stock)), it.Unit,
		))
	}

	return b.String(), nil
}

// RefreshStockNote 重新生成并保存某 PI 的库存说明。
func (s *ProformaInvoiceService) RefreshStockNote(
	ctx context.Context,
	id uint64,
) (string, error) {
	pi, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	note, err := s.buildStockNote(ctx, pi)
	if err != nil {
		return "", err
	}
	if err := s.Repo.SetStockNote(ctx, id, note); err != nil {
		return "", err
	}
	return note, nil
}

func dedupeIDs(ids []uint64) []uint64 {
	seen := make(map[uint64]bool, len(ids))
	result := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func (s *ProformaInvoiceService) resolveOrders(
	ctx context.Context,
	req *models.ProformaInvoiceGenerateRequest,
) ([]models.Order, error) {
	all, err := s.Orders.List(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[uint64]models.Order, len(all))
	for _, o := range all {
		byID[o.ID] = o
	}

	// 指定订单优先。
	if len(req.OrderIDs) > 0 {
		result := make([]models.Order, 0, len(req.OrderIDs))
		for _, id := range req.OrderIDs {
			if o, ok := byID[id]; ok {
				result = append(result, o)
			} else {
				return nil, fmt.Errorf("订单 %d 不存在", id)
			}
		}
		return result, nil
	}

	from, _ := parseDate(req.DateFrom)
	to, _ := parseDate(req.DateTo)
	customer := strings.TrimSpace(req.CustomerName)

	result := make([]models.Order, 0, len(all))
	for _, o := range all {
		if customer != "" && !strings.Contains(o.Customer, customer) {
			continue
		}
		if !from.IsZero() && o.CreatedAt.Before(from) {
			continue
		}
		if !to.IsZero() && o.CreatedAt.After(to.AddDate(0, 0, 1)) {
			continue
		}
		result = append(result, o)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// appendShipmentCharges 为发货单追加运费/保险费用行，返回新的 sortOrder。
//
// 发货单上的运费/保险按录入时的币种保存：币种为 CNY 时按 PI 签发月份汇率折算成
// PI 币种（与箱费口径一致），其余币种视为已与 PI 同币种，原样带入。
func (s *ProformaInvoiceService) appendShipmentCharges(
	ctx context.Context,
	pi *models.ProformaInvoice,
	sh *models.Shipment,
	order models.Order,
	sortOrder int,
) int {
	if sh.FreightCost > 0 {
		sortOrder++
		method := strings.TrimSpace(sh.ShippingMethod)
		if method == "" {
			method = "truck"
		}
		pi.Items = append(pi.Items, models.ProformaInvoiceItem{
			ItemType:   models.PIItemCharge,
			ChargeType: models.PIChargeShipping,
			Title: fmt.Sprintf(
				"Shipping cost by %s for %s for %sctns",
				method, order.OrderNumber, formatQty(sh.CartonCount),
			),
			Amount:     s.chargeAmountInPI(ctx, pi, sh, sh.FreightCost),
			OrderID:    uint64Ptr(order.ID),
			ShipmentID: uint64Ptr(sh.ID),
			SortOrder:  sortOrder,
		})
	}

	if sh.InsuranceCost > 0 {
		sortOrder++
		pi.Items = append(pi.Items, models.ProformaInvoiceItem{
			ItemType:   models.PIItemCharge,
			ChargeType: models.PIChargeInsurance,
			Title:      fmt.Sprintf("Insurance Cost for %s", order.OrderNumber),
			Amount:     s.chargeAmountInPI(ctx, pi, sh, sh.InsuranceCost),
			OrderID:    uint64Ptr(order.ID),
			ShipmentID: uint64Ptr(sh.ID),
			SortOrder:  sortOrder,
		})
	}

	return sortOrder
}

// chargeAmountInPI 把发货单的费用金额换算成 PI 币种。
//
// 只有发货单币种为 CNY 时才做换算（÷ 汇率，保留 2 位小数）；USD 等其它币种
// 视为与 PI 同币种，原样返回。汇率取不到时同样原样返回，避免把金额算成 0。
func (s *ProformaInvoiceService) chargeAmountInPI(
	ctx context.Context,
	pi *models.ProformaInvoice,
	sh *models.Shipment,
	amount float64,
) float64 {
	amount = round2(amount)
	if amount <= 0 {
		return 0
	}
	if !strings.EqualFold(strings.TrimSpace(sh.Currency), "CNY") {
		return amount
	}

	rate := s.piExchangeRate(ctx, pi.IssuedDate)
	if rate <= 0 {
		return amount
	}
	return round2(amount / rate)
}

// Save 保存 PI：分配单号、重算合计并落库。
func (s *ProformaInvoiceService) Save(
	ctx context.Context,
	p *models.ProformaInvoice,
) error {
	if p.Currency == "" {
		p.Currency = "USD"
	}
	if p.Status == "" {
		p.Status = models.PIStatusDraft
	}
	switch p.Status {
	case models.PIStatusDraft, models.PIStatusIssued, models.PIStatusClosed:
	default:
		return fmt.Errorf("PI 状态无效")
	}
	if p.IssuedDate.Time.IsZero() {
		p.IssuedDate = models.Date{Time: time.Now()}
	}
	if len(p.Items) == 0 {
		return fmt.Errorf("PI 至少需要一条明细")
	}

	if p.PINumber == "" {
		number, err := s.Repo.NextNumber(ctx)
		if err != nil {
			return err
		}
		p.PINumber = number
	}

	for i := range p.Items {
		if p.Items[i].SortOrder == 0 {
			p.Items[i].SortOrder = i + 1
		}
	}

	// 箱费 / 运费等手工行常常没带订单号，先按 PI 的唯一订单补齐，
	// 否则财务利润明细会把它们算成「未关联订单」。
	assignMissingOrderIDs(p)

	s.recalcTotals(p)

	// 历史欠款不允许为负，避免抵减本期应收。
	if p.CarryoverAmount < 0 {
		p.CarryoverAmount = 0
	}

	// 累计未收 = 本期合计 + 历史欠款 − 已收款。

	// 更新时按收款记录重算已收款，避免被前端覆盖。
	if p.ID != 0 && s.Payments != nil {
		paid, err := s.Payments.SumByPI(ctx, p.ID)
		if err != nil {
			return err
		}
		p.PaidAmount = round2(paid)
	}

	p.TotalUnpaid = round2(p.Amount + p.CarryoverAmount - p.PaidAmount)

	if p.ID == 0 {
		return s.Repo.Create(ctx, p)
	}
	return s.Repo.Update(ctx, p)
}

// assignMissingOrderIDs 给缺订单号的明细行补上归属。
//
// 财务利润明细按 order_id 分组，箱费、运费、手工加的明细行缺 order_id 就会单独
// 落到「未关联订单」。PI 只对应一个客户订单时（绝大多数情况），缺省行一律归到该订单；
// 跨多个订单的 PI 不猜，保持为空。
func assignMissingOrderIDs(p *models.ProformaInvoice) {
	var orderID uint64
	multiple := false

	for i := range p.Items {
		it := &p.Items[i]
		if it.OrderID == nil || *it.OrderID == 0 {
			continue
		}
		if orderID == 0 {
			orderID = *it.OrderID
			continue
		}
		if *it.OrderID != orderID {
			multiple = true
			break
		}
	}

	if orderID == 0 || multiple {
		return
	}

	for i := range p.Items {
		it := &p.Items[i]
		if it.OrderID == nil || *it.OrderID == 0 {
			it.OrderID = uint64Ptr(orderID)
		}
	}
}

// recalcTotals 依据明细重算各项合计（Paid 行不计入金额）。
func (s *ProformaInvoiceService) recalcTotals(p *models.ProformaInvoice) {
	var itemAmount, boxAmount, insuranceAmount, freightAmount float64

	for i := range p.Items {
		it := &p.Items[i]

		// 货款行单价大于 0 时按 数量 × 单价 归一金额，避免前端漏算。
		// 箱费行允许业务员手工改写金额，仅在未填金额时按 数量 × 单价 兜底。
		if it.ItemType == models.PIItemGoods && it.UnitPrice > 0 {
			it.Amount = round2(it.Quantity * it.UnitPrice)
		} else if it.ItemType == models.PIItemBox && it.UnitPrice > 0 && it.Amount == 0 {
			it.Amount = round2(it.Quantity * it.UnitPrice)
		}

		switch it.ItemType {
		case models.PIItemGoods:
			if !it.IsPaid {
				itemAmount += it.Amount
			}
		case models.PIItemBox:
			if !it.IsPaid {
				boxAmount += it.Amount
			}
		case models.PIItemCharge:
			switch it.ChargeType {
			case models.PIChargeInsurance:
				insuranceAmount += it.Amount
			case models.PIChargeShipping:
				freightAmount += it.Amount
			}
		}
	}

	p.ItemAmount = round2(itemAmount)
	p.BoxAmount = round2(boxAmount)
	p.InsuranceAmount = round2(insuranceAmount)
	p.FreightAmount = round2(freightAmount)
	p.Amount = round2(p.ItemAmount + p.BoxAmount + p.InsuranceAmount + p.FreightAmount)
}

func uint64Ptr(v uint64) *uint64 { return &v }

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", value)
}

// shipmentGroupTitle 生成分组标题：{单号} ({箱数}ctns) {运输方式}。
func shipmentGroupTitle(sh *models.Shipment) string {
	title := sh.OrderNumber
	if sh.CartonCount > 0 {
		title += fmt.Sprintf(" (%sctns)", formatQty(sh.CartonCount))
	}
	if method := strings.TrimSpace(sh.ShippingMethod); method != "" {
		title += " " + method
	}
	return strings.TrimSpace(title)
}

// fallbackGroupTitle 订单无发货单时的分组标题，箱数由明细换算。
//
// 箱数按**订单行分组**统计：组合套装的组成单品装在同一箱里，
// 逐行累加会把同一箱重复计入（1 套 = 1 个 A + 1 个 B 时，A、B 行各算一箱 → 多算一倍）。
func fallbackGroupTitle(order models.Order, lines []models.OrderShipmentLine) string {
	groupCartons := make(map[uint64]float64)
	var cartons float64
	for i := range lines {
		l := &lines[i]
		if l.PackQuantity <= 0 {
			continue
		}
		rowCartons := l.RequiredQuantity / l.PackQuantity
		if cur, ok := groupCartons[l.OrderItemID]; !ok || rowCartons > cur {
			groupCartons[l.OrderItemID] = rowCartons
		}
	}
	for _, c := range groupCartons {
		cartons += c
	}

	title := order.OrderNumber
	if cartons > 0 {
		title += fmt.Sprintf(" (%sctns)", formatQty(round3(cartons)))
	}
	if method := strings.TrimSpace(order.ShippingMethod); method != "" {
		title += " " + method
	}
	return strings.TrimSpace(title)
}

// goodsFromShipmentItem 把发货明细转换为 PI 货款行（材料转箱费行）。
func goodsFromShipmentItem(
	it *models.ShipmentItem,
	groupNo int,
	orderID, shipmentID uint64,
	sortOrder int,
) models.ProformaInvoiceItem {
	row := models.ProformaInvoiceItem{
		GroupNo:     groupNo,
		OrderID:     uint64Ptr(orderID),
		ShipmentID:  uint64Ptr(shipmentID),
		SkuID:       it.SkuID,
		MaterialID:  it.MaterialID,
		ShortName:   it.ShortName,
		Description: it.Description,
		Quantity:    it.Quantity,
		Unit:        it.Unit,
		UnitPrice:   it.UnitPrice,
		Amount:      it.Amount,
		IsPaid:      it.IsPaid,
		SortOrder:   sortOrder,
	}

	if it.ItemType == models.InventoryItemMaterial {
		row.ItemType = models.PIItemBox
	} else {
		row.ItemType = models.PIItemGoods
	}

	if row.Description == "" {
		row.Description = BuildShipmentDescription(it.ShortName, it.Quantity, it.PackQuantity)
	}
	if row.Amount == 0 {
		row.Amount = round2(it.UnitPrice * it.Quantity)
	}

	return row
}
