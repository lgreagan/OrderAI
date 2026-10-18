package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/repository"
)

// PurchaseOrderService 负责持久化采购订单的业务逻辑。
type PurchaseOrderService struct {
	Repo         *repository.PurchaseOrderRepository
	Demand       *repository.PurchaseDemandRepository
	Inventory    *InventoryService
	Requirements *MaterialRequirementService
}

func NewPurchaseOrderService(
	repo *repository.PurchaseOrderRepository,
	demand *repository.PurchaseDemandRepository,
	inventory *InventoryService,
	requirements *MaterialRequirementService,
) *PurchaseOrderService {
	return &PurchaseOrderService{
		Repo:         repo,
		Demand:       demand,
		Inventory:    inventory,
		Requirements: requirements,
	}
}

func (s *PurchaseOrderService) List(
	ctx context.Context,
	keyword, status string,
) ([]models.PurchaseOrderListItem, error) {
	return s.Repo.List(ctx, keyword, status)
}

// ItemSummary 采购报表：按物品汇总采购量、到货量与金额。
func (s *PurchaseOrderService) ItemSummary(
	ctx context.Context,
	keyword, status string,
) ([]models.PurchaseItemSummary, error) {
	return s.Repo.ItemSummary(ctx, keyword, status)
}

// MaterialNeeds 「采购材料」：带出纸箱等材料用量，便于与产品一起采购。
//
// 纸箱数量**按源订单的需求量一对一折算**，与采购单里买多少无关：
//   - 单品直配行：订单需求量 ÷ SKU 每箱数量（SKU 自己的纸箱）
//   - 组合套装行：订单套数 ÷ 组合每箱套数（组合纸箱，如「6厘米两件套纸箱」一箱 72 套）
//
// 客人要 2 箱西瓜球7厘米薄荷绿（共 160 个）、库存已有 100 个只采购 60 个时，
// 纸箱默认仍然是 2 箱——要买的纸箱数由订单决定，实际买几只由操作人员改。
func (s *PurchaseOrderService) MaterialNeeds(
	ctx context.Context,
	req *models.PurchaseMaterialRequest,
) ([]models.MaterialRequirement, error) {
	if s.Requirements == nil {
		return nil, fmt.Errorf("材料需求服务未初始化")
	}
	if req == nil || len(req.Items) == 0 {
		return nil, fmt.Errorf("请先载入产品明细并填写采购量")
	}

	products, fromOrder, err := s.materialProducts(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(products) == 0 {
		return nil, fmt.Errorf("没有可核算材料的产品明细（请先填采购量）")
	}

	needs, err := s.Requirements.Needs(ctx, products)
	if err != nil {
		return nil, err
	}
	// 标记折算依据：界面据此说明"按源订单需求（不随采购量变化）"还是"按采购量"。
	basis := models.MaterialBasisPurchase
	if fromOrder {
		basis = models.MaterialBasisOrder
	}
	for i := range needs {
		needs[i].Basis = basis
	}

	return needs, nil
}

// materialProducts 折算参与材料核算的产品行，返回是否取自源订单需求。
//
// 采购单关联了来源订单时，直接按订单行一对一取需求量（组合行=套数、单品行=个数），
// 完全不看采购量；只保留采购单里确实在买的那些 SKU（产品行被删掉的不带纸箱）。
// 没有来源订单时才退回"按采购量 ÷ SKU 每箱数量"，避免凭空猜测。
func (s *PurchaseOrderService) materialProducts(
	ctx context.Context,
	req *models.PurchaseMaterialRequest,
) ([]MaterialProduct, bool, error) {
	// 采购单里在买的 SKU（数量填 0 视为不买）。
	requested := make(map[uint64]bool, len(req.Items))
	skuQty := make(map[uint64]float64)
	for i := range req.Items {
		it := &req.Items[i]
		if it.ItemType == models.InventoryItemMaterial || it.ItemID == 0 {
			continue
		}
		if it.Quantity > 0 {
			requested[it.ItemID] = true
		}
		skuQty[it.ItemID] += it.Quantity
	}
	if len(requested) == 0 {
		return nil, false, nil
	}

	if s.Demand != nil && req.SourceOrderID != 0 {
		lines, err := s.Demand.ListOrderLineDemand(ctx, req.SourceOrderID)
		if err != nil {
			return nil, false, err
		}
		if products := orderLineProducts(lines, requested); len(products) > 0 {
			return products, true, nil
		}
	}

	// 回退：没有来源订单（或订单里查不到这些 SKU），按采购量 × 单品箱规。
	products := make([]MaterialProduct, 0, len(skuQty))
	for i := range req.Items {
		it := &req.Items[i]
		if it.ItemType == models.InventoryItemMaterial || it.ItemID == 0 || it.Quantity <= 0 {
			continue
		}
		products = append(products, MaterialProduct{
			ProductType: models.ProductionProductSku,
			RefID:       it.ItemID,
			Quantity:    round3(it.Quantity),
		})
	}

	return products, false, nil
}

// orderLineProducts 把订单行需求量折算成产品行：组合行按规则（套数）、单品行按 SKU（个数）。
//
// 组合行在库里按成员单品展开成多条，这里按订单行去重，只记一次套数。
func orderLineProducts(
	lines []models.PurchaseDemandLine,
	requested map[uint64]bool,
) []MaterialProduct {
	type productKey struct {
		kind string
		ref  uint64
	}
	// 同一订单行的组合行会展开多条，去重后再累加。
	type lineKey struct {
		itemID uint64
		kind   string
		ref    uint64
	}

	qty := make(map[productKey]float64)
	name := make(map[uint64]string)
	seq := make([]productKey, 0, len(lines))
	added := make(map[lineKey]bool, len(lines))

	for _, l := range lines {
		if l.SkuID == 0 || !requested[l.SkuID] {
			continue
		}

		// 单品行：需求量就是个数；组合行：需求量 ÷ 每套用量 = 套数。
		need := l.RequiredQuantity
		kind, ref := models.ProductionProductSku, l.SkuID
		if l.RuleID != 0 {
			kind, ref = models.ProductionProductRule, l.RuleID
			qps := l.QuantityPerSet
			if qps <= 0 {
				qps = 1
			}
			if need > 0 {
				need /= qps
			} else {
				need = l.Quantity
			}
			if name[ref] == "" {
				name[ref] = l.RuleNameZH
			}
		}
		if need <= 0 {
			continue
		}

		lk := lineKey{itemID: l.OrderItemID, kind: kind, ref: ref}
		if added[lk] {
			continue
		}
		added[lk] = true

		pk := productKey{kind: kind, ref: ref}
		if _, ok := qty[pk]; !ok {
			seq = append(seq, pk)
		}
		qty[pk] += need
	}

	products := make([]MaterialProduct, 0, len(seq))
	for _, pk := range seq {
		if qty[pk] <= 0 {
			continue
		}
		products = append(products, MaterialProduct{
			ProductType: pk.kind,
			RefID:       pk.ref,
			ProductName: name[pk.ref],
			Quantity:    round3(qty[pk]),
		})
	}

	return products
}

func (s *PurchaseOrderService) Get(
	ctx context.Context,
	id uint64,
) (*models.PurchaseOrder, error) {
	return s.Repo.GetByID(ctx, id)
}

// Delete 删除采购单，并撤销其产生的入库流水。
func (s *PurchaseOrderService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("采购单 ID 无效")
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.Inventory != nil {
		return s.Inventory.Repo.DeleteByRef(ctx, "purchase_order", id)
	}
	return nil
}

// Suggestion 生成"从订单创建采购单"的建议：订单需求 + 当前库存 + 在途采购 + 建议采购量。
func (s *PurchaseOrderService) Suggestion(
	ctx context.Context,
	orderID uint64,
) (*models.PurchaseOrderSuggestion, error) {
	if orderID == 0 {
		return nil, fmt.Errorf("订单 ID 无效")
	}

	order, err := s.Demand.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	items, err := s.Demand.ListOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	return &models.PurchaseOrderSuggestion{
		OrderID:     order.ID,
		OrderNumber: order.OrderNumber,
		Customer:    order.Customer,
		Country:     order.Country,
		Currency:    "CNY",
		Items:       items,
	}, nil
}

// MarkOrderPurchased 把客户订单标记为已采购。
func (s *PurchaseOrderService) MarkOrderPurchased(
	ctx context.Context,
	orderID uint64,
) error {
	if orderID == 0 {
		return fmt.Errorf("订单 ID 无效")
	}
	return s.Demand.UpdateOrderStatus(ctx, orderID, models.OrderStatusPurchased)
}

// Save 新增(ID=0)或更新采购单：校验、计算金额并落库。
func (s *PurchaseOrderService) Save(
	ctx context.Context,
	po *models.PurchaseOrder,
) error {
	if po.Currency == "" {
		po.Currency = "CNY"
	}
	if po.Status == "" {
		po.Status = models.PurchaseStatusDraft
	}
	switch po.Status {
	case models.PurchaseStatusDraft, models.PurchaseStatusOrdered,
		models.PurchaseStatusPartial, models.PurchaseStatusReceived,
		models.PurchaseStatusCancelled:
	default:
		return fmt.Errorf("采购单状态无效")
	}
	if po.OrderDate.Time.IsZero() {
		po.OrderDate = models.Date{Time: time.Now()}
	}
	if len(po.Items) == 0 {
		return fmt.Errorf("采购单至少需要一条明细")
	}

	for i := range po.Items {
		it := &po.Items[i]
		if it.ItemType == "" {
			it.ItemType = models.InventoryItemSku
		}
		if it.ItemType != models.InventoryItemSku &&
			it.ItemType != models.InventoryItemMaterial {
			return fmt.Errorf("第 %d 行物品类型无效", i+1)
		}
		if it.Quantity < 0 {
			return fmt.Errorf("第 %d 行采购数量不能小于0", i+1)
		}
		if it.UnitPrice < 0 {
			return fmt.Errorf("第 %d 行单价不能小于0", i+1)
		}
		if strings.TrimSpace(it.ItemCode) == "" && strings.TrimSpace(it.ItemName) == "" {
			return fmt.Errorf("第 %d 行物品不能为空", i+1)
		}
		it.Amount = round2(it.UnitPrice * it.Quantity)
	}

	// 编辑时先读原单：前端未回传的单号沿用原值，同时留底用于冲销被删除的行。
	var previous *models.PurchaseOrder
	if po.ID != 0 {
		existing, err := s.Repo.GetByID(ctx, po.ID)
		if err != nil {
			return err
		}
		previous = existing
		if po.PONumber == "" {
			po.PONumber = existing.PONumber
		}
	}

	if po.PONumber == "" {
		number, err := s.Repo.NextNumber(ctx)
		if err != nil {
			return err
		}
		po.PONumber = number
	}

	s.recalcAmounts(po)

	if po.ID == 0 {
		if err := s.Repo.Create(ctx, po); err != nil {
			return err
		}
	} else {
		if err := s.Repo.Update(ctx, po); err != nil {
			return err
		}
		// 明细行被删除时，其已入库的货不会跟着消失，必须补一条冲销流水，
		// 否则会出现「单据显示未到货、库存却已增加」的脱节。
		if err := s.revokeRemovedReceipts(ctx, previous, po); err != nil {
			return err
		}
	}

	// 回读一次，返回带真实明细 ID 的完整单据。
	return s.reload(ctx, po)
}

// revokeRemovedReceipts 冲销被删除明细行已入库的部分。
//
// 采购单编辑会整体重写明细：旧行删除、新行由数据库重新分配 ID，
// 只有「行 ID 仍被提交」的行才会保留已入库数量。被删掉的行若已到过货，
// 其 purchase_in 流水仍留在台账里，库存会凭空多出来，所以在此补反向流水。
func (s *PurchaseOrderService) revokeRemovedReceipts(
	ctx context.Context,
	previous *models.PurchaseOrder,
	current *models.PurchaseOrder,
) error {
	if s.Inventory == nil || previous == nil || current == nil {
		return nil
	}

	// 前端回传的行 ID 即为保留集合（行 ID 在重写后由数据库重新分配）。
	kept := make(map[uint64]bool, len(current.Items))
	for i := range current.Items {
		if id := current.Items[i].ID; id != 0 {
			kept[id] = true
		}
	}

	warehouse := s.Inventory.Warehouse(ctx)
	for i := range previous.Items {
		it := &previous.Items[i]
		if kept[it.ID] || it.ItemID == nil || it.ReceivedQuantity <= 0 {
			continue
		}

		if err := s.Inventory.RecordOut(
			ctx, it.ItemType, *it.ItemID, models.InventoryBizAdjust,
			"purchase_order", previous.ID, it.ReceivedQuantity,
			it.UnitPrice, previous.Currency, warehouse,
			fmt.Sprintf(
				"删除采购明细 %s 冲销已入库",
				firstNonEmpty(it.ItemCode, it.ItemName),
			),
		); err != nil {
			return err
		}
	}

	return nil
}

// reload 用数据库中的最新数据覆盖入参，便于接口返回真实明细 ID 与金额。
func (s *PurchaseOrderService) reload(ctx context.Context, po *models.PurchaseOrder) error {
	fresh, err := s.Repo.GetByID(ctx, po.ID)
	if err != nil {
		return err
	}
	*po = *fresh
	return nil
}

// PushDown 由客户订单下推生成采购单草稿：保存原始需求快照，供采购员按实际情况修改。
//
// 同一订单若已有下推草稿则直接返回该草稿，避免重复下推。
func (s *PurchaseOrderService) PushDown(
	ctx context.Context,
	orderID uint64,
) (*models.PurchaseOrder, bool, error) {
	if orderID == 0 {
		return nil, false, fmt.Errorf("订单 ID 无效")
	}

	if id, err := s.Repo.FindDraftBySourceOrder(ctx, orderID); err != nil {
		return nil, false, err
	} else if id != 0 {
		po, err := s.Repo.GetByID(ctx, id)
		if err != nil {
			return nil, false, err
		}
		return po, false, nil
	}

	sug, err := s.Suggestion(ctx, orderID)
	if err != nil {
		return nil, false, err
	}

	items := make([]models.PurchaseOrderItem, 0, len(sug.Items))
	for i := range sug.Items {
		d := &sug.Items[i]
		if d.RequiredQuantity <= 0 {
			continue
		}

		qty := d.SuggestedQuantity
		if qty < 0 {
			qty = 0
		}

		skuID := d.SkuID
		items = append(items, models.PurchaseOrderItem{
			ItemType:         models.InventoryItemSku,
			ItemID:           &skuID,
			ItemCode:         d.SkuCode,
			ItemName:         firstNonEmpty(d.ProductNameZH, d.ProductName),
			ShortName:        d.ShortName,
			Spec:             d.Model,
			Unit:             firstNonEmpty(d.Unit, "pcs"),
			RequiredQuantity: d.RequiredQuantity,
			Quantity:         qty,
			// 单价关联 SKU 的成本价，采购员可再按实际修改。
			UnitPrice: d.Price,
		})
	}

	if len(items) == 0 {
		return nil, false, fmt.Errorf("该订单没有可下推的采购需求")
	}

	po := &models.PurchaseOrder{
		SourceOrderID: &orderID,
		Status:        models.PurchaseStatusDraft,
		Currency:      "CNY",
		Remark:        fmt.Sprintf("由订单 %s 下推，需求数量已保存，请按实际情况修改", sug.OrderNumber),
		Items:         items,
	}

	if err := s.Save(ctx, po); err != nil {
		return nil, false, err
	}

	return po, true, nil
}

// recalcAmounts 计算货款与合计。
func (s *PurchaseOrderService) recalcAmounts(po *models.PurchaseOrder) {
	var goods float64
	for i := range po.Items {
		goods += po.Items[i].Amount
	}
	po.GoodsAmount = round2(goods)
	po.FreightAmount = round2(po.FreightAmount)
	po.TotalAmount = round2(po.GoodsAmount + po.FreightAmount)
}

// Receive 到货登记：按行更新已入库数量并写入入库流水，随后刷新单据状态。
func (s *PurchaseOrderService) Receive(
	ctx context.Context,
	id uint64,
	req *models.PurchaseOrderReceiveRequest,
) error {
	if id == 0 {
		return fmt.Errorf("采购单 ID 无效")
	}
	if req == nil || len(req.Items) == 0 {
		return fmt.Errorf("没有需要登记的到货明细")
	}

	po, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	byID := make(map[uint64]models.PurchaseOrderItem, len(po.Items))
	for _, it := range po.Items {
		byID[it.ID] = it
	}

	warehouse := "default"
	if s.Inventory != nil {
		warehouse = s.Inventory.Warehouse(ctx)
	}

	for _, row := range req.Items {
		it, ok := byID[row.ID]
		if !ok {
			continue
		}

		target := row.ReceivedQuantity
		if target < 0 {
			target = 0
		}
		delta := target - it.ReceivedQuantity

		if delta != 0 && it.ItemID != nil && s.Inventory != nil {
			if delta > 0 {
				if err := s.Inventory.RecordIn(
					ctx, it.ItemType, *it.ItemID, models.InventoryBizPurchaseIn,
					"purchase_order", po.ID, delta, it.UnitPrice, po.Currency,
					warehouse, "采购到货入库",
				); err != nil {
					return err
				}
			} else {
				if err := s.Inventory.RecordOut(
					ctx, it.ItemType, *it.ItemID, models.InventoryBizAdjust,
					"purchase_order", po.ID, -delta, it.UnitPrice, po.Currency,
					warehouse, "采购到货更正",
				); err != nil {
					return err
				}
			}
		}

		if err := s.Repo.SetItemReceived(ctx, it.ID, target); err != nil {
			return err
		}
		it.ReceivedQuantity = target
		byID[row.ID] = it
	}

	anyReceived, allReceived := summarizeReceipt(po.Items, byID)

	switch {
	case allReceived && len(po.Items) > 0:
		return s.Repo.UpdateStatus(ctx, id, models.PurchaseStatusReceived)
	case anyReceived:
		return s.Repo.UpdateStatus(ctx, id, models.PurchaseStatusPartial)
	default:
		// 尚未到货：保留单据原有状态（如已下单）。
		return nil
	}
}

// summarizeReceipt 统计到货情况：是否有到货、是否全部到齐。
func summarizeReceipt(
	items []models.PurchaseOrderItem,
	current map[uint64]models.PurchaseOrderItem,
) (anyReceived, allReceived bool) {
	allReceived = true

	for _, it := range items {
		received := it.ReceivedQuantity
		if row, ok := current[it.ID]; ok {
			received = row.ReceivedQuantity
		}

		if received > 0 {
			anyReceived = true
		}
		if it.Quantity > 0 && received < it.Quantity {
			allReceived = false
		}
	}

	return anyReceived, allReceived
}
