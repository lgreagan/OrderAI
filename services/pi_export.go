package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"OrderAI/models"
)

// goodsQtySuffixRe 匹配 PI 描述里的 "x<数量>pcs"（含之后的装箱信息），
// 用于只替换产品名部分，保留数量/装箱描述。
var goodsQtySuffixRe = regexp.MustCompile(`(?i)x\s*[0-9]+(\.[0-9]+)?\s*pcs`)

// GetForExport 返回用于导出 Excel 的 PI：
//   - 货款行的产品名改为 order_items.product_name + specification 生成的英文名；
//   - 箱费行改用材料对应的英文名；
//   - 整份 PI（含库存说明）不含任何中文。
func (s *ProformaInvoiceService) GetForExport(
	ctx context.Context,
	id uint64,
) (*models.ProformaInvoice, error) {
	pi, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	skuNames, materialNames := s.exportNames(ctx, pi.Items)
	s.applyExportNames(ctx, pi, skuNames, materialNames)
	return pi, nil
}

// SKUImages 取 PI 货款行用到的产品图片（sku_id -> 图片路径），供 Excel 导出插入。
func (s *ProformaInvoiceService) SKUImages(
	ctx context.Context,
	pi *models.ProformaInvoice,
) map[uint64]string {
	if s.SKUs == nil {
		return nil
	}

	ids := make([]uint64, 0)
	for i := range pi.Items {
		it := &pi.Items[i]
		if it.ItemType == models.PIItemGoods && it.SkuID != nil {
			ids = append(ids, *it.SkuID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	images, err := s.SKUs.ImagesByIDs(ctx, dedupeIDs(ids))
	if err != nil {
		return nil
	}
	return images
}

// OrderImages 取 PI 各分组对应订单的客户原始订单截图（order_id -> 图片路径）。
//
// 截图取上传给 AI 识别的那张图（ai_results.image_path），为空时回退
// orders.source_image；都没有的订单不出现在结果里，导出时该分组留空。
func (s *ProformaInvoiceService) OrderImages(
	ctx context.Context,
	pi *models.ProformaInvoice,
) map[uint64]string {
	if s.Orders == nil {
		return nil
	}

	ids := make([]uint64, 0)
	for i := range pi.Items {
		it := &pi.Items[i]
		if it.ItemType == models.PIItemGroup && it.OrderID != nil {
			ids = append(ids, *it.OrderID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	images, err := s.Orders.SourceImagesByIDs(ctx, dedupeIDs(ids))
	if err != nil {
		return nil
	}
	return images
}

// exportNames 批量取 PI 明细用到的 SKU / 材料英文名。
func (s *ProformaInvoiceService) exportNames(
	ctx context.Context,
	items []models.ProformaInvoiceItem,
) (map[uint64]string, map[uint64]string) {
	skuIDs := make([]uint64, 0)
	materialIDs := make([]uint64, 0)
	for i := range items {
		it := &items[i]
		if it.ItemType == models.PIItemGoods && it.SkuID != nil {
			skuIDs = append(skuIDs, *it.SkuID)
		}
		if it.ItemType == models.PIItemBox && it.MaterialID != nil {
			materialIDs = append(materialIDs, *it.MaterialID)
		}
	}

	skuNames := map[uint64]string{}
	if s.SKUs != nil {
		if m, err := s.SKUs.ProductNamesByIDs(ctx, dedupeIDs(skuIDs)); err == nil {
			skuNames = m
		}
	}

	materialNames := map[uint64]string{}
	if s.Materials == nil {
		return skuNames, materialNames
	}

	materials := make(map[uint64]*models.Material)
	codes := make([]string, 0)
	for _, id := range dedupeIDs(materialIDs) {
		m, err := s.Materials.GetByID(ctx, id)
		if err != nil || m == nil {
			continue
		}
		materials[id] = m
		if code := materialSKUCode(m.Code); code != "" {
			codes = append(codes, code)
		}
	}

	codeNames := map[string]string{}
	if s.SKUs != nil && len(codes) > 0 {
		if m, err := s.SKUs.ProductNamesByCodes(ctx, codes); err == nil {
			codeNames = m
		}
	}

	for id, m := range materials {
		name := ""
		if code := materialSKUCode(m.Code); code != "" {
			name = buildPIProductName(codeNames[code], "")
		}
		if name == "" {
			name = sanitizePIText(firstNonEmpty(m.ShortName, firstNonEmpty(m.Name, m.NameZH)))
		} else {
			name += " carton"
		}
		materialNames[id] = name
	}

	return skuNames, materialNames
}

// materialSKUCode 取材料编码里的 SKU 编码（自动建的纸箱编码形如 CTN-<sku_code>）。
func materialSKUCode(code string) string {
	code = strings.TrimSpace(code)
	if len(code) > 4 && strings.EqualFold(code[:4], "CTN-") {
		return code[4:]
	}
	return ""
}

// applyExportNames 把 PI 明细与库存说明替换成英文。
func (s *ProformaInvoiceService) applyExportNames(
	ctx context.Context,
	pi *models.ProformaInvoice,
	skuNames, materialNames map[uint64]string,
) {
	for i := range pi.Items {
		it := &pi.Items[i]

		switch it.ItemType {
		case models.PIItemGoods:
			name := s.goodsEnglishName(ctx, it, skuNames)
			it.ShortName = name
			it.Description = rebuildGoodsDescription(it.Description, name, it.Quantity)
		case models.PIItemBox:
			name := ""
			if it.MaterialID != nil {
				name = materialNames[*it.MaterialID]
			}
			if name == "" {
				name = sanitizePIText(firstNonEmpty(it.ShortName, it.Description))
			}
			it.ShortName = name
			it.Description = name
		case models.PIItemGroup:
			it.GroupTitle = sanitizePIText(it.GroupTitle)
		case models.PIItemCharge:
			it.Title = sanitizePIText(it.Title)
		case models.PIItemNote:
			it.Description = sanitizePILines(it.Description)
		}

		it.Model = sanitizePIText(it.Model)
		it.Remark = sanitizePIText(it.Remark)
	}

	if hasCJK(pi.StockNote) {
		pi.StockNote = s.englishStockNote(ctx, pi, skuNames, materialNames)
	}
	pi.StockNote = sanitizePILines(pi.StockNote)

	pi.CustomerName = sanitizePIText(pi.CustomerName)
	pi.PINumber = sanitizePIText(pi.PINumber)
	pi.Currency = sanitizePIText(pi.Currency)
}

// goodsEnglishName 取货款行的英文产品名（order_items.product_name + specification）。
func (s *ProformaInvoiceService) goodsEnglishName(
	ctx context.Context,
	it *models.ProformaInvoiceItem,
	skuNames map[uint64]string,
) string {
	if s.Orders != nil && it.OrderID != nil && it.SkuID != nil {
		productName, specification, err := s.Orders.OrderItemProductName(
			ctx, *it.OrderID, *it.SkuID,
		)
		if err == nil {
			if name := buildPIProductName(productName, specification); name != "" {
				return name
			}
		}
	}

	if it.SkuID != nil {
		if name := buildPIProductName(skuNames[*it.SkuID], ""); name != "" {
			return name
		}
	}

	return sanitizePIText(firstNonEmpty(it.ShortName, it.Description))
}

// rebuildGoodsDescription 保留原描述里的数量/装箱信息，只把产品名换成英文名。
func rebuildGoodsDescription(description, name string, quantity float64) string {
	suffix := ""
	if loc := goodsQtySuffixRe.FindStringIndex(description); loc != nil {
		suffix = description[loc[0]:]
	}
	if suffix == "" && quantity > 0 {
		suffix = "x" + formatQty(quantity) + "pcs"
	}
	return sanitizePIText(name + suffix)
}

// englishStockNote 用英文产品名重建库存说明（原说明含中文时才调用）。
func (s *ProformaInvoiceService) englishStockNote(
	ctx context.Context,
	pi *models.ProformaInvoice,
	skuNames, materialNames map[uint64]string,
) string {
	header := "Current Stock"
	if s.Settings != nil {
		raw := strings.TrimSpace(s.Settings.StockNoteHeader(ctx))
		if raw != "" && !hasCJK(raw) {
			header = sanitizePIText(raw)
		}
	}

	stock := map[string]float64{}
	if s.Inventory != nil {
		if rows, err := s.Inventory.StockByIDs(
			ctx, models.InventoryItemSku, dedupeIDs(piSKUIds(pi)),
		); err == nil {
			for _, r := range rows {
				stock[fmt.Sprintf("sku:%d", r.ItemID)] = r.StockQuantity
			}
		}
		if rows, err := s.Inventory.StockByIDs(
			ctx, models.InventoryItemMaterial, dedupeIDs(piMaterialIds(pi)),
		); err == nil {
			for _, r := range rows {
				stock[fmt.Sprintf("material:%d", r.ItemID)] = r.StockQuantity
			}
		}
	}

	lines := []string{header}
	seen := map[string]bool{}
	for i := range pi.Items {
		it := &pi.Items[i]

		var key, name, unit string
		switch it.ItemType {
		case models.PIItemGoods:
			if it.SkuID == nil {
				continue
			}
			key = fmt.Sprintf("sku:%d", *it.SkuID)
			name = buildPIProductName(skuNames[*it.SkuID], "")
		case models.PIItemBox:
			if it.MaterialID == nil {
				continue
			}
			key = fmt.Sprintf("material:%d", *it.MaterialID)
			name = materialNames[*it.MaterialID]
		default:
			continue
		}
		unit = sanitizePIText(it.Unit)

		qty, ok := stock[key]
		if !ok {
			continue
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		lines = append(lines, fmt.Sprintf("%s: %s %s", name, formatQty(round3(qty)), unit))
	}

	return strings.Join(lines, "\n")
}

func piSKUIds(pi *models.ProformaInvoice) []uint64 {
	ids := make([]uint64, 0)
	for i := range pi.Items {
		if pi.Items[i].ItemType == models.PIItemGoods && pi.Items[i].SkuID != nil {
			ids = append(ids, *pi.Items[i].SkuID)
		}
	}
	return ids
}

func piMaterialIds(pi *models.ProformaInvoice) []uint64 {
	ids := make([]uint64, 0)
	for i := range pi.Items {
		if pi.Items[i].ItemType == models.PIItemBox && pi.Items[i].MaterialID != nil {
			ids = append(ids, *pi.Items[i].MaterialID)
		}
	}
	return ids
}
