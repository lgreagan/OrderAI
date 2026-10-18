package services

import (
	"context"
	"errors"
	"fmt"
	"math"

	"OrderAI/models"
	"OrderAI/repository"
)

// MaterialRequirementService 计算"产品 → 材料"的需求。
//
// 依据产品用料清单(BOM)按产品数量折算；纸箱用料缺失时按装箱信息自动补全
// （与生产下推纸箱用料的口径一致）。后续扩展包材时只需补充 BOM，无需改动调用方。
type MaterialRequirementService struct {
	Bom       *repository.ProductBomRepository
	Materials *repository.MaterialRepository
	Packs     *repository.PackInfoRepository
}

func NewMaterialRequirementService(
	bom *repository.ProductBomRepository,
	materials *repository.MaterialRepository,
	packs *repository.PackInfoRepository,
) *MaterialRequirementService {
	return &MaterialRequirementService{Bom: bom, Materials: materials, Packs: packs}
}

// MaterialProduct 参与材料核算的产品及其数量。
type MaterialProduct struct {
	ProductType string
	RefID       uint64
	ProductName string
	Quantity    float64
}

// Needs 计算产品材料需求（按材料汇总），并确保纸箱用料已建立。
func (s *MaterialRequirementService) Needs(
	ctx context.Context,
	products []MaterialProduct,
) ([]models.MaterialRequirement, error) {
	if s.Bom == nil {
		return []models.MaterialRequirement{}, nil
	}

	// 1) 纸箱用料缺失时按装箱信息补全，保证 BOM 完整。
	for i := range products {
		p := &products[i]
		if p.Quantity <= 0 {
			continue
		}
		if _, err := s.EnsureCartonBom(ctx, p.ProductType, p.RefID, p.ProductName); err != nil {
			return nil, err
		}
	}

	// 2) 批量读取涉及产品的 BOM。
	refs := make([]repository.ProductRef, 0, len(products))
	seen := make(map[repository.ProductRef]bool, len(products))
	for i := range products {
		p := &products[i]
		if p.RefID == 0 {
			continue
		}
		ref := repository.ProductRef{Type: p.ProductType, RefID: p.RefID}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return []models.MaterialRequirement{}, nil
	}

	boms, err := s.Bom.ListByProducts(ctx, refs)
	if err != nil {
		return nil, err
	}

	// 3) 按材料汇总用量与涉及产品数。
	result := make([]models.MaterialRequirement, 0)
	index := make(map[uint64]int)
	productSets := make(map[uint64]map[repository.ProductRef]bool)
	// 每单位用量：用于反推"每箱数量"并展示折算依据（1 / 每单位用量 = 每箱数量）。
	perUnit := make(map[uint64]float64)
	mixedPerUnit := make(map[uint64]bool)
	// 产品数量（个 / 套）按材料汇总：已知整数每箱数量时用它做除法，比乘法准（见下）。
	productQty := make(map[uint64]float64)
	// 每箱数量的单位：组合纸箱按"套"、单品纸箱按"个"计（例如"每箱 72 套"）。
	packUnit := make(map[uint64]string)
	mixedPackUnit := make(map[uint64]bool)
	// 包材（纸箱等）必须按整只计：7.999 箱要记 8 箱，不能出现小数箱。
	isPackaging := make(map[uint64]bool)

	for i := range products {
		p := &products[i]
		if p.RefID == 0 || p.Quantity <= 0 {
			continue
		}
		ref := repository.ProductRef{Type: p.ProductType, RefID: p.RefID}
		for _, b := range boms[ref] {
			pos, ok := index[b.MaterialID]
			if !ok {
				result = append(result, models.MaterialRequirement{
					MaterialID:   b.MaterialID,
					MaterialCode: b.MaterialCode,
					MaterialName: b.MaterialName,
					Spec:         b.MaterialSpec,
					Unit:         firstNonEmpty(b.Unit, "pcs"),
				})
				pos = len(result) - 1
				index[b.MaterialID] = pos
				productSets[b.MaterialID] = make(map[repository.ProductRef]bool)
			}
			if b.MaterialCategory == models.MaterialCategoryPackaging {
				isPackaging[b.MaterialID] = true
			}
			if v, seen := perUnit[b.MaterialID]; !seen {
				perUnit[b.MaterialID] = b.QuantityPerUnit
			} else if v != b.QuantityPerUnit {
				// 同一材料口径不一，无法用单一每箱数量表达。
				mixedPerUnit[b.MaterialID] = true
			}
			unit := "个"
			if p.ProductType == models.ProductionProductRule {
				// 组合规则行的"数量"是套数，每箱数量自然按套计。
				unit = "套"
			}
			if v, seen := packUnit[b.MaterialID]; !seen {
				packUnit[b.MaterialID] = unit
			} else if v != unit {
				mixedPackUnit[b.MaterialID] = true
			}
			result[pos].Quantity += p.Quantity * b.QuantityPerUnit
			productQty[b.MaterialID] += p.Quantity
			productSets[b.MaterialID][ref] = true
		}
	}

	ids := make([]uint64, 0, len(result))
	for i := range result {
		mid := result[i].MaterialID

		// 每单位用量 = 1 / 每箱数量，反推回去即用户维护的每箱数量。
		// 用料清单里的用量是 6 位小数（1/144 → 0.006944），反推会带出 144.009
		// 这类毛刺，接近整数时还原成整数，便于界面显示；
		// 同时把这个整数记下来，下面直接用整数做除法，避免"乘回去"放大毛刺
		// （1/72 存成 0.013889，72 × 0.013889 = 1.000008，向上取整会凭空多出一只纸箱）。
		packPerCarton := 0.0
		if !mixedPerUnit[mid] && perUnit[mid] > 0 {
			v := 1 / perUnit[mid]
			if r := math.Round(v); r > 0 && math.Abs(v-r) < r*0.01 {
				v = r
				packPerCarton = r
			}
			result[i].PackPerCarton = round3(v)
			if !mixedPackUnit[mid] {
				result[i].PackUnit = packUnit[mid]
			}
		}

		q := result[i].Quantity
		if isPackaging[mid] {
			// 纸箱按整只采购：1152 ÷ 144 = 7.999 也要记 8 箱。
			//
			// 已知整数每箱数量时改用"产品数量 ÷ 每箱数量"，比"数量 × 1/每箱数量"准：
			// 用料清单的每单位用量只存 6 位小数（1/72 → 0.013889），
			// 72 套乘回去是 1.000008，向上取整会凭空多出一只纸箱。
			if packPerCarton > 0 {
				q = productQty[mid] / packPerCarton
			}
			q = math.Ceil(q)
		}
		result[i].Quantity = round3(q)
		result[i].ProductCount = len(productSets[mid])
		result[i].DemandQuantity = round3(productQty[mid])
		ids = append(ids, mid)
	}

	// 4) 带出材料单价，便于采购单直接生成采购金额。
	if s.Materials != nil && len(ids) > 0 {
		prices, err := s.Materials.PricesByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for i := range result {
			result[i].Price = prices[result[i].MaterialID]
		}
	}

	return result, nil
}

// PackagingNeeds 计算产品的包装材料需求（只含包材类），用于发货时自动核销。
//
// 口径与 Needs 一致：纸箱用料缺失时先按装箱信息补全，再按产品数量折算。
// 只统计 category = packaging 的材料，避免把原材料也当成"随货消耗"。
func (s *MaterialRequirementService) PackagingNeeds(
	ctx context.Context,
	products []MaterialProduct,
) ([]models.MaterialRequirement, error) {
	if s.Bom == nil {
		return []models.MaterialRequirement{}, nil
	}

	for i := range products {
		p := &products[i]
		if p.Quantity <= 0 {
			continue
		}
		if _, err := s.EnsureCartonBom(ctx, p.ProductType, p.RefID, p.ProductName); err != nil {
			return nil, err
		}
	}

	refs := make([]repository.ProductRef, 0, len(products))
	seen := make(map[repository.ProductRef]bool, len(products))
	for i := range products {
		p := &products[i]
		if p.RefID == 0 || p.Quantity <= 0 {
			continue
		}
		ref := repository.ProductRef{Type: p.ProductType, RefID: p.RefID}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return []models.MaterialRequirement{}, nil
	}

	boms, err := s.Bom.ListPackagingByProducts(ctx, refs)
	if err != nil {
		return nil, err
	}

	result := make([]models.MaterialRequirement, 0)
	index := make(map[uint64]int)
	// 与 Needs 同口径：记下每单位用量与产品数量，用于把「数量 × 1/每箱」换成整数除法。
	// 用料清单只存 6 位小数（1/642 → 0.001558），642 乘回去是 1.0002，
	// 向上取整就凭空多出一只纸箱 —— 必须直接用「数量 ÷ 整数每箱数量」。
	perUnit := make(map[uint64]float64)
	mixedPerUnit := make(map[uint64]bool)
	productQty := make(map[uint64]float64)
	packUnit := make(map[uint64]string)
	mixedPackUnit := make(map[uint64]bool)

	for i := range products {
		p := &products[i]
		if p.RefID == 0 || p.Quantity <= 0 {
			continue
		}
		ref := repository.ProductRef{Type: p.ProductType, RefID: p.RefID}
		for _, b := range boms[ref] {
			pos, ok := index[b.MaterialID]
			if !ok {
				result = append(result, models.MaterialRequirement{
					MaterialID:   b.MaterialID,
					MaterialCode: b.MaterialCode,
					MaterialName: b.MaterialName,
					Spec:         b.MaterialSpec,
					Unit:         firstNonEmpty(b.Unit, "pcs"),
				})
				pos = len(result) - 1
				index[b.MaterialID] = pos
			}
			if v, seen := perUnit[b.MaterialID]; !seen {
				perUnit[b.MaterialID] = b.QuantityPerUnit
			} else if v != b.QuantityPerUnit {
				mixedPerUnit[b.MaterialID] = true
			}
			// 组合纸箱按"套"、单品纸箱按"个"计（例如"每箱 72 套"）。
			unit := "个"
			if p.ProductType == models.ProductionProductRule {
				unit = "套"
			}
			if v, seen := packUnit[b.MaterialID]; !seen {
				packUnit[b.MaterialID] = unit
			} else if v != unit {
				mixedPackUnit[b.MaterialID] = true
			}
			result[pos].Quantity += p.Quantity * b.QuantityPerUnit
			productQty[b.MaterialID] += p.Quantity
		}
	}

	for i := range result {
		mid := result[i].MaterialID

		packPerCarton := 0.0
		if !mixedPerUnit[mid] && perUnit[mid] > 0 {
			v := 1 / perUnit[mid]
			if r := math.Round(v); r > 0 && math.Abs(v-r) < r*0.01 {
				v = r
				packPerCarton = r
			}
			result[i].PackPerCarton = round3(v)
			if !mixedPackUnit[mid] {
				result[i].PackUnit = packUnit[mid]
			}
		}

		// 包材按整只核销：发货实际用掉的纸箱不可能是 7.999 个。
		q := result[i].Quantity
		if packPerCarton > 0 {
			q = productQty[mid] / packPerCarton
		}
		result[i].Quantity = round3(math.Ceil(q))
		result[i].DemandQuantity = round3(productQty[mid])
	}

	return result, nil
}

// EnsureCartonBom 依据装箱信息补全纸箱用料（材料 + BOM），返回是否新建了纸箱材料。
func (s *MaterialRequirementService) EnsureCartonBom(
	ctx context.Context,
	productType string,
	refID uint64,
	productName string,
) (bool, error) {
	if s.Packs == nil || s.Bom == nil || s.Materials == nil || refID == 0 {
		return false, nil
	}

	var ruleIDs, skuIDs []uint64
	switch productType {
	case models.ProductionProductRule:
		ruleIDs = append(ruleIDs, refID)
	case models.ProductionProductSku:
		skuIDs = append(skuIDs, refID)
	default:
		return false, nil
	}

	packInfo, err := s.Packs.ListPackInfo(ctx, ruleIDs, skuIDs)
	if err != nil {
		return false, err
	}

	info, ok := packInfo[models.PackKey(productType, refID)]
	if !ok || info.PackPerCarton <= 0 {
		return false, nil
	}

	material, isNew, err := s.ensureCartonMaterial(ctx, info, productName)
	if err != nil {
		return isNew, err
	}

	// 每箱一个纸箱：用量 = 1 / 每箱数量，完工时 已生产数量 × 用量 = 箱数。
	if err := s.Bom.Upsert(ctx, &models.ProductBom{
		ProductType:     productType,
		ProductRefID:    refID,
		MaterialID:      material.ID,
		QuantityPerUnit: 1 / info.PackPerCarton,
		Unit:            "pcs",
		Remark:          "由装箱信息下推",
		Status:          "active",
	}); err != nil {
		return isNew, err
	}

	return isNew, nil
}

// ensureCartonMaterial 取得或创建纸箱材料（编码确定，便于重复下推时复用）。
func (s *MaterialRequirementService) ensureCartonMaterial(
	ctx context.Context,
	info models.PackInfo,
	productName string,
) (*models.Material, bool, error) {
	code := "CTN-" + info.Code
	if info.Code == "" {
		code = fmt.Sprintf("CTN-%s-%d", info.ProductType, info.ProductRefID)
	}

	spec := info.CartonSpec()

	existing, err := s.Materials.GetByCode(ctx, code)
	if err == nil {
		// 已存在：仅补全缺失的规格，不覆盖人工维护的内容。
		if existing.Spec == "" && spec != "" {
			existing.Spec = spec
			if uerr := s.Materials.Update(ctx, existing); uerr != nil {
				return nil, false, uerr
			}
		}
		return existing, false, nil
	}
	if !errors.Is(err, models.ErrMaterialNotFound) {
		return nil, false, err
	}

	name := productName
	if name == "" {
		name = info.Name
	}
	if name == "" {
		name = code
	}

	material := &models.Material{
		Code:      code,
		Name:      name + " carton",
		NameZH:    name + "纸箱",
		ShortName: name + "纸箱",
		Category:  models.MaterialCategoryPackaging,
		Spec:      spec,
		Unit:      "pcs",
		Price:     0,
		Currency:  "CNY",
		Status:    "active",
		Remark:    "由 SKU/组合规则装箱信息自动下推",
	}

	if err := s.Materials.Create(ctx, material); err != nil {
		// 并发下可能已被创建，回读一次。
		if again, e2 := s.Materials.GetByCode(ctx, code); e2 == nil {
			return again, false, nil
		}
		return nil, false, err
	}

	return material, true, nil
}
