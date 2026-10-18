package services

import (
	"context"

	"OrderAI/models"
)

// convertBoxCostToUSD 把箱费行的人民币单价按汇率折算成 PI 外币单价。
//
// 纸箱单价维护在 materials.price 里，单位是人民币；PI 是外币报价，
// 直接用人民币单价会让箱费高出汇率倍数。折算汇率取 PI 签发月份的月度汇率，
// 该月未维护时回落到系统设置的默认汇率。
func (s *ProformaInvoiceService) convertBoxCostToUSD(
	ctx context.Context,
	pi *models.ProformaInvoice,
) {
	rate := s.piExchangeRate(ctx, pi.IssuedDate)
	if rate <= 0 {
		return
	}

	for i := range pi.Items {
		it := &pi.Items[i]
		if it.ItemType != models.PIItemBox || it.UnitPrice <= 0 {
			continue
		}
		// 单价按 2 位小数折算，保证「数量 × 单价 = 金额」在发票上自洽。
		it.UnitPrice = round2(it.UnitPrice / rate)
		it.Amount = round2(it.Quantity * it.UnitPrice)
	}
}

// piExchangeRate 返回 1 外币 = N CNY 的折算汇率。
func (s *ProformaInvoiceService) piExchangeRate(
	ctx context.Context,
	issued models.Date,
) float64 {
	rate := DefaultExchangeRate
	if s.Settings == nil {
		return rate
	}

	rate = s.Settings.ExchangeRate(ctx)
	rates, err := s.Settings.MonthlyExchangeRates(ctx)
	if err != nil {
		return rate
	}

	resolved, _ := resolveRate(issued, rates, rate)
	if resolved <= 0 {
		return rate
	}
	return resolved
}

// fillItemPrices 用产品/材料主数据补全 PI 明细的单价。
//
// 发货明细与采购明细里的单价可能为 0（例如发货单建立时产品还没维护 PI 售价），
// 这里按主数据兜底：货款行取 skus.price_usd（PI 美金售价），箱费行取 materials.price。
// 只补 0 值，已有单价的明细保持原样。
func (s *ProformaInvoiceService) fillItemPrices(
	ctx context.Context,
	pi *models.ProformaInvoice,
) {
	skuPrices := map[uint64]float64{}
	if s.SKUs != nil {
		ids := make([]uint64, 0)
		for i := range pi.Items {
			it := &pi.Items[i]
			if it.ItemType == models.PIItemGoods && it.SkuID != nil && it.UnitPrice <= 0 {
				ids = append(ids, *it.SkuID)
			}
		}
		if prices, err := s.SKUs.PriceUSDByIDs(ctx, dedupeIDs(ids)); err == nil {
			skuPrices = prices
		}
	}

	materialPrices := map[uint64]float64{}
	if s.Materials != nil {
		ids := make([]uint64, 0)
		for i := range pi.Items {
			it := &pi.Items[i]
			if it.ItemType == models.PIItemBox && it.MaterialID != nil && it.UnitPrice <= 0 {
				ids = append(ids, *it.MaterialID)
			}
		}
		if prices, err := s.Materials.PricesByIDs(ctx, dedupeIDs(ids)); err == nil {
			materialPrices = prices
		}
	}

	for i := range pi.Items {
		it := &pi.Items[i]
		if it.UnitPrice > 0 {
			continue
		}

		switch it.ItemType {
		case models.PIItemGoods:
			if it.SkuID != nil {
				it.UnitPrice = skuPrices[*it.SkuID]
			}
		case models.PIItemBox:
			if it.MaterialID != nil {
				it.UnitPrice = materialPrices[*it.MaterialID]
			}
		}
	}
}
