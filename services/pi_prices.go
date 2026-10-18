package services

import (
	"context"

	"OrderAI/models"
)

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
