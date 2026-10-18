package services

import (
	"context"
	"fmt"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// InventoryService 负责库存台账与结存。
type InventoryService struct {
	Repo     *repository.InventoryRepository
	Settings *SettingsService
}

func NewInventoryService(
	repo *repository.InventoryRepository,
	settings *SettingsService,
) *InventoryService {
	return &InventoryService{Repo: repo, Settings: settings}
}

// Warehouse 返回默认仓库（来自系统设置，缺省 default）。
func (s *InventoryService) Warehouse(ctx context.Context) string {
	if s != nil && s.Settings != nil {
		if w := s.Settings.DefaultWarehouse(ctx); w != "" {
			return w
		}
	}
	return "default"
}

func (s *InventoryService) ListStock(
	ctx context.Context,
	itemType, keyword string,
) ([]models.InventoryStock, error) {
	return s.Repo.ListStock(ctx, itemType, keyword)
}

func (s *InventoryService) ListTransactions(
	ctx context.Context,
	itemType, keyword, bizType string,
	limit int,
) ([]models.InventoryTransaction, error) {
	return s.Repo.ListTransactions(ctx, itemType, keyword, bizType, limit)
}

// Adjust 手工盘点调整：按 direction 写入一条 adjust 流水。
func (s *InventoryService) Adjust(
	ctx context.Context,
	req *models.InventoryAdjustRequest,
) error {
	if req.ItemID == 0 {
		return fmt.Errorf("请选择物品")
	}
	if req.ItemType != models.InventoryItemSku &&
		req.ItemType != models.InventoryItemMaterial {
		return fmt.Errorf("物品类型无效")
	}
	if req.Direction != 1 && req.Direction != -1 {
		return fmt.Errorf("调整方向无效")
	}
	if req.Quantity <= 0 {
		return fmt.Errorf("调整数量必须大于0")
	}
	if strings.TrimSpace(req.Warehouse) == "" {
		req.Warehouse = s.Warehouse(ctx)
	}

	return s.Repo.Create(ctx, &models.InventoryTransaction{
		ItemType:  req.ItemType,
		ItemID:    req.ItemID,
		Direction: req.Direction,
		BizType:   models.InventoryBizAdjust,
		Quantity:  req.Quantity,
		UnitCost:  req.UnitCost,
		Currency:  "CNY",
		Warehouse: req.Warehouse,
		RefType:   "manual",
		Remark:    req.Remark,
	})
}

// ManualIn 手工入库：写入一条 manual_in 流水（产品/材料均可，用于临时补录/初始库存）。
func (s *InventoryService) ManualIn(
	ctx context.Context,
	req *models.InventoryInRequest,
) error {
	if req.ItemID == 0 {
		return fmt.Errorf("请选择物品")
	}
	if req.ItemType != models.InventoryItemSku &&
		req.ItemType != models.InventoryItemMaterial {
		return fmt.Errorf("物品类型无效")
	}
	if req.Quantity <= 0 {
		return fmt.Errorf("入库数量必须大于0")
	}
	if strings.TrimSpace(req.Warehouse) == "" {
		req.Warehouse = s.Warehouse(ctx)
	}

	return s.Repo.Create(ctx, &models.InventoryTransaction{
		ItemType:  req.ItemType,
		ItemID:    req.ItemID,
		Direction: 1,
		BizType:   models.InventoryBizManualIn,
		Quantity:  req.Quantity,
		UnitCost:  req.UnitCost,
		Currency:  "CNY",
		Warehouse: req.Warehouse,
		RefType:   "manual",
		Remark:    req.Remark,
	})
}

// IssueMaterials 领料出库：按行写入 requisition 流水（direction=-1）。
//
// 领料数量由用户按实际填写（可高于 BOM 理论用量，用于覆盖生产损耗），
// 但不允许把任何材料的结存领成负数——负库存意味着账实已经对不上，事后极难追溯。
//
// 注意：包装材料（纸箱等）随货发出时由发货单按实际发货量自动核销，
// 不需要也不应该在领料页面重复出库，否则同一只纸箱会被扣两次。
func (s *InventoryService) IssueMaterials(
	ctx context.Context,
	req *models.MaterialIssueRequest,
) error {
	if req == nil || len(req.Items) == 0 {
		return fmt.Errorf("没有需要领料的材料")
	}

	warehouse := strings.TrimSpace(req.Warehouse)
	if warehouse == "" {
		warehouse = s.Warehouse(ctx)
	}

	refType := strings.TrimSpace(req.RefType)
	if refType == "" {
		refType = "manual"
	}

	var refID *uint64
	if req.RefID != 0 {
		id := req.RefID
		refID = &id
	}

	// 先按材料汇总本次领用量，再与结存核对：同一材料拆成多行也要合并判断。
	need := make(map[uint64]float64)
	ids := make([]uint64, 0, len(req.Items))
	for _, it := range req.Items {
		if it.MaterialID == 0 || it.Quantity <= 0 {
			continue
		}
		if _, ok := need[it.MaterialID]; !ok {
			ids = append(ids, it.MaterialID)
		}
		need[it.MaterialID] += it.Quantity
	}
	if len(ids) == 0 {
		return fmt.Errorf("没有有效的领料明细")
	}

	if err := s.ensureStockEnough(ctx, ids, need); err != nil {
		return err
	}

	issued := 0
	for _, it := range req.Items {
		if it.MaterialID == 0 || it.Quantity <= 0 {
			continue
		}
		remark := strings.TrimSpace(it.Remark)
		if remark == "" {
			remark = req.Remark
		}
		if err := s.Repo.Create(ctx, &models.InventoryTransaction{
			ItemType:  models.InventoryItemMaterial,
			ItemID:    it.MaterialID,
			Direction: -1,
			BizType:   models.InventoryBizRequisition,
			Quantity:  it.Quantity,
			UnitCost:  it.UnitCost,
			Currency:  "CNY",
			Warehouse: warehouse,
			RefType:   refType,
			RefID:     refID,
			Remark:    remark,
		}); err != nil {
			return err
		}
		issued++
	}

	if issued == 0 {
		return fmt.Errorf("没有有效的领料明细")
	}
	return nil
}

// ensureStockEnough 校验领用量不超过当前结存，把负库存挡在写入之前。
func (s *InventoryService) ensureStockEnough(
	ctx context.Context,
	ids []uint64,
	need map[uint64]float64,
) error {
	stocks, err := s.Repo.StockByIDs(ctx, models.InventoryItemMaterial, ids)
	if err != nil {
		return err
	}

	onHand := make(map[uint64]float64, len(ids))
	label := make(map[uint64]string, len(ids))
	for _, st := range stocks {
		onHand[st.ItemID] += st.StockQuantity
		if name := strings.TrimSpace(st.ItemCode + " " + st.ItemName); name != "" {
			label[st.ItemID] = name
		}
	}

	short := make([]string, 0)
	for _, id := range ids {
		if need[id] <= onHand[id]+0.0001 {
			continue
		}
		name := label[id]
		if name == "" {
			name = fmt.Sprintf("材料#%d", id)
		}
		short = append(short, fmt.Sprintf("%s（结存 %s，需领 %s）",
			name, formatQty(onHand[id]), formatQty(need[id])))
	}

	if len(short) > 0 {
		return fmt.Errorf("以下材料结存不足，请先入库或减少领用量：%s",
			strings.Join(short, "；"))
	}
	return nil
}

// RecordIn 入库（采购到货等调用），数量取绝对值。
func (s *InventoryService) RecordIn(
	ctx context.Context,
	itemType string,
	itemID uint64,
	bizType string,
	refType string,
	refID uint64,
	quantity float64,
	unitCost float64,
	currency string,
	warehouse string,
	remark string,
) error {
	if itemID == 0 || quantity <= 0 {
		return nil
	}
	if warehouse == "" {
		warehouse = "default"
	}
	if currency == "" {
		currency = "CNY"
	}
	ref := refID
	return s.Repo.Create(ctx, &models.InventoryTransaction{
		ItemType:  itemType,
		ItemID:    itemID,
		Direction: 1,
		BizType:   bizType,
		Quantity:  quantity,
		UnitCost:  unitCost,
		Currency:  currency,
		Warehouse: warehouse,
		RefType:   refType,
		RefID:     &ref,
		Remark:    remark,
	})
}

// RecordOut 出库（发货/领料等调用），数量取绝对值。
func (s *InventoryService) RecordOut(
	ctx context.Context,
	itemType string,
	itemID uint64,
	bizType string,
	refType string,
	refID uint64,
	quantity float64,
	unitCost float64,
	currency string,
	warehouse string,
	remark string,
) error {
	if itemID == 0 || quantity <= 0 {
		return nil
	}
	if warehouse == "" {
		warehouse = "default"
	}
	if currency == "" {
		currency = "CNY"
	}
	ref := refID
	return s.Repo.Create(ctx, &models.InventoryTransaction{
		ItemType:  itemType,
		ItemID:    itemID,
		Direction: -1,
		BizType:   bizType,
		Quantity:  quantity,
		UnitCost:  unitCost,
		Currency:  currency,
		Warehouse: warehouse,
		RefType:   refType,
		RefID:     &ref,
		Remark:    remark,
	})
}
