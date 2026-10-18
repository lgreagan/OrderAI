package models

import "time"

// 库存物品类型。
const (
	InventoryItemSku      = "sku"
	InventoryItemMaterial = "material"
)

// 库存流水业务类型。
const (
	InventoryBizPurchaseIn   = "purchase_in"
	InventoryBizProductionIn = "production_in"
	InventoryBizShipmentOut  = "shipment_out"
	InventoryBizConsumption  = "consumption"
	InventoryBizAdjust       = "adjust"
	InventoryBizManualIn     = "manual_in"         // 手工入库
	InventoryBizRequisition  = "requisition"       // 领料出库
	InventoryBizShipmentPack = "shipment_pack_out" // 发货随货核销包装材料
)

// InventoryTransaction 库存流水（入库 direction=1 / 出库 direction=-1）。
type InventoryTransaction struct {
	ID         uint64    `json:"id"`
	ItemType   string    `json:"item_type"`
	ItemID     uint64    `json:"item_id"`
	Direction  int       `json:"direction"`
	BizType    string    `json:"biz_type"`
	Quantity   float64   `json:"quantity"`
	UnitCost   float64   `json:"unit_cost"`
	Currency   string    `json:"currency"`
	Warehouse  string    `json:"warehouse"`
	BatchNo    string    `json:"batch_no"`
	RefType    string    `json:"ref_type"`
	RefID      *uint64   `json:"ref_id"`
	OccurredAt time.Time `json:"occurred_at"`
	Operator   string    `json:"operator"`
	Remark     string    `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`

	// 展示用：来自 skus / materials 的编码与名称。
	ItemCode string `json:"item_code"`
	ItemName string `json:"item_name"`
}

// InventoryStock 库存结存（按 物品+仓库 汇总）。
type InventoryStock struct {
	ItemType      string  `json:"item_type"`
	ItemID        uint64  `json:"item_id"`
	Warehouse     string  `json:"warehouse"`
	StockQuantity float64 `json:"stock_quantity"`
	ItemCode      string  `json:"item_code"`
	ItemName      string  `json:"item_name"`
	Unit          string  `json:"unit"`
}

// InventoryAdjustRequest 手工盘点调整请求。
type InventoryAdjustRequest struct {
	ItemType  string  `json:"item_type"`
	ItemID    uint64  `json:"item_id"`
	Direction int     `json:"direction"`
	Quantity  float64 `json:"quantity"`
	UnitCost  float64 `json:"unit_cost"`
	Warehouse string  `json:"warehouse"`
	Remark    string  `json:"remark"`
}

// InventoryInRequest 手工入库请求（产品/材料）。
type InventoryInRequest struct {
	ItemType  string  `json:"item_type"`
	ItemID    uint64  `json:"item_id"`
	Quantity  float64 `json:"quantity"`
	UnitCost  float64 `json:"unit_cost"`
	Warehouse string  `json:"warehouse"`
	Remark    string  `json:"remark"`
}

// MaterialIssueItem 领料单行。
type MaterialIssueItem struct {
	MaterialID uint64  `json:"material_id"`
	Quantity   float64 `json:"quantity"`
	UnitCost   float64 `json:"unit_cost"`
	Remark     string  `json:"remark"`
}

// MaterialIssueRequest 领料出库请求。
type MaterialIssueRequest struct {
	Warehouse string              `json:"warehouse"`
	RefType   string              `json:"ref_type"` // 可选：production_order / manual
	RefID     uint64              `json:"ref_id"`
	Remark    string              `json:"remark"`
	Items     []MaterialIssueItem `json:"items"`
}
