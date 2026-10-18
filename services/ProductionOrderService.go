package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/repository"
)

// ProductionOrderService 负责持久化生产订单的业务逻辑。
type ProductionOrderService struct {
	Repo         *repository.ProductionOrderRepository
	Demand       *repository.ProductionDemandRepository
	Requirements *MaterialRequirementService
	Inventory    *InventoryService
}

func NewProductionOrderService(
	repo *repository.ProductionOrderRepository,
	demand *repository.ProductionDemandRepository,
	requirements *MaterialRequirementService,
	inventory *InventoryService,
) *ProductionOrderService {
	return &ProductionOrderService{
		Repo:         repo,
		Demand:       demand,
		Requirements: requirements,
		Inventory:    inventory,
	}
}

// PushDown 由客户订单下推生成生产单草稿：保存原始需求快照（供车间按实际情况修改），
// 并按 SKU / 组合规则的装箱信息下推纸箱用料（产品 → 纸箱，用量 = 1 / 每箱数量）。
//
// 同一订单若已有下推草稿则直接返回该草稿，避免重复下推。
func (s *ProductionOrderService) PushDown(
	ctx context.Context,
	orderID uint64,
) (*models.ProductionOrder, int, bool, error) {
	if orderID == 0 {
		return nil, 0, false, fmt.Errorf("订单 ID 无效")
	}

	if id, err := s.Repo.FindDraftBySourceOrder(ctx, orderID); err != nil {
		return nil, 0, false, err
	} else if id != 0 {
		po, err := s.Repo.GetByID(ctx, id)
		if err != nil {
			return nil, 0, false, err
		}
		return po, 0, false, nil
	}

	sug, err := s.Suggestion(ctx, orderID)
	if err != nil {
		return nil, 0, false, err
	}
	if len(sug.Items) == 0 {
		return nil, 0, false, fmt.Errorf("该订单没有可下推的生产需求")
	}

	// 先下推纸箱用料，成功后再落库生产单，避免半成品数据。
	cartons, err := s.pushCartonBom(ctx, sug.Items)
	if err != nil {
		return nil, 0, false, err
	}

	po := &models.ProductionOrder{
		SourceOrderID: &orderID,
		Status:        models.ProductionStatusDraft,
		// 订单下推默认是投料生产：领原料、完工入成品库。
		WorkType: models.ProductionWorkProduce,
		PlanDate: models.Date{Time: time.Now()},
		Remark:   fmt.Sprintf("由订单 %s 下推，需求数量已保存，请按实际情况修改", sug.OrderNumber),
		Items:    sug.Items,
	}

	if err := s.Save(ctx, po); err != nil {
		return nil, cartons, false, err
	}

	return po, cartons, true, nil
}

// fillCartonInfo 按装箱信息补全每箱数量（用于显示箱数）。
func (s *ProductionOrderService) fillCartonInfo(
	ctx context.Context,
	items []models.ProductionOrderItem,
) {
	if s.Requirements == nil || s.Requirements.Packs == nil {
		return
	}

	var ruleIDs, skuIDs []uint64
	for i := range items {
		if items[i].ProductRefID == nil {
			continue
		}
		switch items[i].ProductType {
		case models.ProductionProductRule:
			ruleIDs = append(ruleIDs, *items[i].ProductRefID)
		case models.ProductionProductSku:
			skuIDs = append(skuIDs, *items[i].ProductRefID)
		}
	}

	packInfo, err := s.Requirements.Packs.ListPackInfo(ctx, ruleIDs, skuIDs)
	if err != nil {
		return
	}

	for i := range items {
		it := &items[i]
		if it.ProductRefID == nil {
			continue
		}
		if info, ok := packInfo[models.PackKey(it.ProductType, *it.ProductRefID)]; ok {
			it.SetsPerCarton = info.PackPerCarton
		}
	}
}

// pushCartonBom 依据装箱信息下推纸箱用料，返回新建的纸箱材料数量。
func (s *ProductionOrderService) pushCartonBom(
	ctx context.Context,
	items []models.ProductionOrderItem,
) (int, error) {
	if s.Requirements == nil {
		return 0, nil
	}

	created := 0
	for i := range items {
		it := &items[i]
		if it.ProductRefID == nil {
			continue
		}
		isNew, err := s.Requirements.EnsureCartonBom(
			ctx, it.ProductType, *it.ProductRefID, it.ProductName,
		)
		if err != nil {
			return created, err
		}
		if isNew {
			created++
		}
	}

	return created, nil
}

func (s *ProductionOrderService) List(
	ctx context.Context,
	keyword, status string,
) ([]models.ProductionOrderListItem, error) {
	return s.Repo.List(ctx, keyword, status)
}

// MaterialSummary 生产报表：汇总生产单所需材料（计划量 × 单位用量）。
func (s *ProductionOrderService) MaterialSummary(
	ctx context.Context,
	keyword, status string,
) ([]models.ProductionMaterialSummary, error) {
	return s.Repo.MaterialSummary(ctx, keyword, status)
}

func (s *ProductionOrderService) Get(
	ctx context.Context,
	id uint64,
) (*models.ProductionOrder, error) {
	return s.Repo.GetByID(ctx, id)
}

// MaterialNeeds 汇总某生产单的材料需求（BOM 理论用量），用于领料预填。
func (s *ProductionOrderService) MaterialNeeds(
	ctx context.Context,
	id uint64,
) ([]models.ProductionMaterialSummary, error) {
	if id == 0 {
		return nil, fmt.Errorf("生产单 ID 无效")
	}
	return s.Repo.MaterialNeeds(ctx, id)
}

// Delete 删除生产单。
//
// 只撤销本单自动生成的成品入库流水。领料出库是人工登记的真实消耗，
// 必须保留（ref_type 仍指向本单便于追溯），否则删单会让材料库存凭空回涨。
func (s *ProductionOrderService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("生产单 ID 无效")
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.Inventory != nil {
		return s.Inventory.Repo.DeleteByRefBiz(
			ctx, "production_order", id,
			models.InventoryBizProductionIn, models.InventoryBizAdjust,
		)
	}
	return nil
}

// MarkOrderProduced 把客户订单标记为已生产。
func (s *ProductionOrderService) MarkOrderProduced(
	ctx context.Context,
	orderID uint64,
) error {
	if orderID == 0 {
		return fmt.Errorf("订单 ID 无效")
	}
	return s.Demand.UpdateOrderStatus(ctx, orderID, models.OrderStatusProduced)
}

// Suggestion 由订单生成生产单的建议：组合规则按"套"、直配 SKU 按"个"汇总。
func (s *ProductionOrderService) Suggestion(
	ctx context.Context,
	orderID uint64,
) (*models.ProductionOrderSuggestion, error) {
	if orderID == 0 {
		return nil, fmt.Errorf("订单 ID 无效")
	}

	order, err := s.Demand.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	tasks, err := s.Demand.ListOrderTasks(ctx, orderID)
	if err != nil {
		return nil, err
	}

	direct, err := s.Demand.ListDirectItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	type key struct {
		Type  string
		RefID uint64
	}

	items := make([]models.ProductionOrderItem, 0)
	index := make(map[key]int)

	addItem := func(k key, build func() models.ProductionOrderItem, addQty func(*models.ProductionOrderItem)) {
		pos, ok := index[k]
		if !ok {
			item := build()
			items = append(items, item)
			pos = len(items) - 1
			index[k] = pos
		}
		addQty(&items[pos])
	}

	// 组合规则任务：以"套"为生产单位。
	for i := range tasks {
		task := &tasks[i]
		if task.RuleID == nil {
			continue
		}
		refID := *task.RuleID
		k := key{Type: models.ProductionProductRule, RefID: refID}
		name := firstNonEmpty(task.RuleProductNameZH, firstNonEmpty(task.RuleProductName, task.ProductName))
		unit := firstNonEmpty(task.OrderUnit, "套")
		setsPerCarton := task.SetsPerCarton
		sets := task.Sets

		addItem(k, func() models.ProductionOrderItem {
			return models.ProductionOrderItem{
				ProductType:   models.ProductionProductRule,
				ProductRefID:  &refID,
				ProductName:   name,
				Unit:          unit,
				SetsPerCarton: setsPerCarton,
			}
		}, func(it *models.ProductionOrderItem) {
			it.RequiredQuantity += sets
			it.PlannedQuantity += sets
		})
	}

	// 直配 SKU：以"个"为生产单位。
	for i := range direct {
		d := &direct[i]
		refID := d.SkuID
		k := key{Type: models.ProductionProductSku, RefID: refID}
		name := firstNonEmpty(d.ProductNameZH, d.ProductName)
		shortName := d.ShortName
		spec := d.Model
		pack := d.PackQuantity
		qty := d.RequiredQuantity

		addItem(k, func() models.ProductionOrderItem {
			return models.ProductionOrderItem{
				ProductType:   models.ProductionProductSku,
				ProductRefID:  &refID,
				ProductName:   name,
				ShortName:     shortName,
				Spec:          spec,
				Unit:          "pcs",
				SetsPerCarton: pack,
			}
		}, func(it *models.ProductionOrderItem) {
			it.RequiredQuantity += qty
			it.PlannedQuantity += qty
		})
	}

	return &models.ProductionOrderSuggestion{
		OrderID:     order.ID,
		OrderNumber: order.OrderNumber,
		Customer:    order.Customer,
		Country:     order.Country,
		Items:       items,
	}, nil
}

// Save 新增(ID=0)或更新生产单：校验、计算箱数并落库。
func (s *ProductionOrderService) Save(
	ctx context.Context,
	po *models.ProductionOrder,
) error {
	if po.Status == "" {
		po.Status = models.ProductionStatusDraft
	}
	switch po.Status {
	case models.ProductionStatusDraft, models.ProductionStatusProducing,
		models.ProductionStatusCompleted, models.ProductionStatusCancelled:
	default:
		return fmt.Errorf("生产单状态无效")
	}
	if po.PlanDate.Time.IsZero() {
		po.PlanDate = models.Date{Time: time.Now()}
	}
	if len(po.Items) == 0 {
		return fmt.Errorf("生产单至少需要一条明细")
	}

	for i := range po.Items {
		it := &po.Items[i]
		switch it.ProductType {
		case models.ProductionProductRule, models.ProductionProductSku:
		case "":
			it.ProductType = models.ProductionProductRule
		default:
			return fmt.Errorf("第 %d 行生产类型无效", i+1)
		}
		if it.PlannedQuantity < 0 {
			return fmt.Errorf("第 %d 行计划生产量不能小于0", i+1)
		}
		if it.Unit == "" {
			it.Unit = "套"
		}
	}

	// 编辑时先读原单：前端不提交的字段（来源单、作业类型、单号）一律沿用旧值，
	// 避免被清空后误导完工入库口径与后续对账。
	var previous *models.ProductionOrder
	if po.ID != 0 {
		existing, err := s.Repo.GetByID(ctx, po.ID)
		if err != nil {
			return err
		}
		previous = existing

		if po.SourcePurchaseOrderID == nil {
			po.SourcePurchaseOrderID = existing.SourcePurchaseOrderID
		}
		if strings.TrimSpace(po.WorkType) == "" {
			po.WorkType = existing.WorkType
		}
		if po.PONumber == "" {
			po.PONumber = existing.PONumber
		}
	}
	po.WorkType = models.NormalizeWorkType(po.WorkType)

	if po.PONumber == "" {
		number, err := s.Repo.NextNumber(ctx)
		if err != nil {
			return err
		}
		po.PONumber = number
	}

	if po.ID == 0 {
		if err := s.Repo.Create(ctx, po); err != nil {
			return err
		}
	} else {
		if err := s.Repo.Update(ctx, po); err != nil {
			return err
		}
	}

	fresh, err := s.Repo.GetByID(ctx, po.ID)
	if err != nil {
		return err
	}
	*po = *fresh

	// 作业类型变更会改变成品入库口径（生产 ↔ 包装），立即重算一次清掉残留流水。
	if previous != nil &&
		models.NormalizeWorkType(previous.WorkType) != models.NormalizeWorkType(po.WorkType) {
		produced := make(map[uint64]models.ProductionOrderItem, len(po.Items))
		for _, it := range po.Items {
			produced[it.ID] = it
		}
		if err := s.syncFinishedGoods(ctx, po, produced); err != nil {
			return err
		}
	}

	return nil
}

// Complete 完工登记：按行更新已生产数量，并按作业类型重算成品入库，随后刷新状态。
//
// 材料消耗不在此自动扣减，改由「领料」页面按实际领用量（含损耗）出库。
func (s *ProductionOrderService) Complete(
	ctx context.Context,
	id uint64,
	req *models.ProductionCompleteRequest,
) error {
	if id == 0 {
		return fmt.Errorf("生产单 ID 无效")
	}
	if req == nil || len(req.Items) == 0 {
		return fmt.Errorf("没有需要登记的完工明细")
	}

	po, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	byID := make(map[uint64]models.ProductionOrderItem, len(po.Items))
	for _, it := range po.Items {
		byID[it.ID] = it
	}

	for _, row := range req.Items {
		it, ok := byID[row.ID]
		if !ok {
			continue
		}

		target := row.ProducedQuantity
		if target < 0 {
			target = 0
		}

		if err := s.Repo.SetItemProduced(ctx, it.ID, target); err != nil {
			return err
		}
		it.ProducedQuantity = target
		byID[row.ID] = it
	}

	// 成品入库按作业类型重算：包装/只记录类型不动成品，避免与采购入库重复。
	if err := s.syncFinishedGoods(ctx, po, byID); err != nil {
		return err
	}

	anyProduced, allProduced := summarizeProduction(po.Items, byID)

	switch {
	case allProduced && len(po.Items) > 0:
		return s.Repo.UpdateStatus(ctx, id, models.ProductionStatusCompleted)
	case anyProduced:
		return s.Repo.UpdateStatus(ctx, id, models.ProductionStatusProducing)
	default:
		return nil
	}
}

// syncFinishedGoods 重算生产单的成品入库流水。
//
// 只有「投料生产」产生成品入库：包装装箱 / 只记录类型的货已在采购环节入库，
// 再入一次即为重复记账。
//
// 记账方式与发货单一致——先清掉本单自动生成的成品流水，再按当前已生产量重写。
// 因此反复修改完工数量不会残留一串调整流水，重复提交也天然幂等。
func (s *ProductionOrderService) syncFinishedGoods(
	ctx context.Context,
	po *models.ProductionOrder,
	current map[uint64]models.ProductionOrderItem,
) error {
	if s.Inventory == nil || po == nil || po.ID == 0 {
		return nil
	}

	// 只清本单自动生成的流水；人工领料出库（同为 production_order 来源）必须保留。
	if err := s.Inventory.Repo.DeleteByRefBiz(
		ctx, "production_order", po.ID,
		models.InventoryBizProductionIn, models.InventoryBizAdjust,
	); err != nil {
		return err
	}

	if !models.WorkTypeInboundsFinished(po.WorkType) {
		return nil
	}

	// 组合规则按套展开成组成单品，单品直接累加。
	ruleIDs := make([]uint64, 0)
	for i := range po.Items {
		if po.Items[i].ProductType == models.ProductionProductRule &&
			po.Items[i].ProductRefID != nil {
			ruleIDs = append(ruleIDs, *po.Items[i].ProductRefID)
		}
	}
	ruleComponents, err := s.Repo.ListRuleComponents(ctx, ruleIDs)
	if err != nil {
		return err
	}

	goods := make(map[uint64]float64)
	for i := range po.Items {
		it := &po.Items[i]
		if it.ProductRefID == nil {
			continue
		}

		produced := it.ProducedQuantity
		if row, ok := current[it.ID]; ok {
			produced = row.ProducedQuantity
		}
		if produced <= 0 {
			continue
		}

		switch it.ProductType {
		case models.ProductionProductRule:
			for _, c := range ruleComponents[*it.ProductRefID] {
				goods[c.SkuID] += produced * c.QuantityPerSet
			}
		case models.ProductionProductSku:
			goods[*it.ProductRefID] += produced
		}
	}

	if len(goods) == 0 {
		return nil
	}

	warehouse := s.Inventory.Warehouse(ctx)
	for skuID, qty := range goods {
		if qty <= 0 {
			continue
		}
		if err := s.Inventory.RecordIn(
			ctx, models.InventoryItemSku, skuID, models.InventoryBizProductionIn,
			"production_order", po.ID, qty, 0, "", warehouse, "生产完工入库",
		); err != nil {
			return err
		}
	}

	return nil
}

// summarizeProduction 统计完工情况：是否有产出、是否全部完成。
func summarizeProduction(
	items []models.ProductionOrderItem,
	current map[uint64]models.ProductionOrderItem,
) (anyProduced, allProduced bool) {
	allProduced = true

	for _, it := range items {
		produced := it.ProducedQuantity
		if row, ok := current[it.ID]; ok {
			produced = row.ProducedQuantity
		}

		if produced > 0 {
			anyProduced = true
		}
		if it.PlannedQuantity > 0 && produced < it.PlannedQuantity {
			allProduced = false
		}
	}

	return anyProduced, allProduced
}
