package models

import (
	"errors"
	"time"
)

// ErrPurchaseOrderNotFound 表示采购订单不存在。
var ErrPurchaseOrderNotFound = errors.New("采购订单不存在")

// 采购单状态。
const (
	PurchaseStatusDraft     = "draft"
	PurchaseStatusOrdered   = "ordered"
	PurchaseStatusPartial   = "partial"
	PurchaseStatusReceived  = "received"
	PurchaseStatusCancelled = "cancelled"
)

// PurchaseOrder 采购订单头（持久化，可人工维护）。
//
// 采购数量可以大于系统建议的客户需求（凑折扣更划算），超出部分到货入库后即形成库存。
type PurchaseOrder struct {
	ID            uint64    `json:"id"`
	PONumber      string    `json:"po_number"`
	SupplierName  string    `json:"supplier_name"`
	SourceOrderID *uint64   `json:"source_order_id"`
	OrderDate     Date      `json:"order_date"`
	ExpectedDate  Date      `json:"expected_date"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	GoodsAmount   float64   `json:"goods_amount"`
	FreightAmount float64   `json:"freight_amount"`
	TotalAmount   float64   `json:"total_amount"`
	Remark        string    `json:"remark"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// SourceOrderNumber 关联订单号（展示用）。
	SourceOrderNumber string `json:"source_order_number"`

	Items []PurchaseOrderItem `json:"items"`
}

// PurchaseOrderItem 采购明细行（产品 SKU 或材料）。
type PurchaseOrderItem struct {
	ID               uint64  `json:"id"`
	PurchaseOrderID  uint64  `json:"purchase_order_id"`
	ItemType         string  `json:"item_type"`
	ItemID           *uint64 `json:"item_id"`
	ItemCode         string  `json:"item_code"`
	ItemName         string  `json:"item_name"`
	ShortName        string  `json:"short_name"`
	Spec             string  `json:"spec"`
	Unit             string  `json:"unit"`
	RequiredQuantity float64 `json:"required_quantity"`
	Quantity         float64 `json:"quantity"`
	UnitPrice        float64 `json:"unit_price"`
	Amount           float64 `json:"amount"`
	ReceivedQuantity float64 `json:"received_quantity"`
	Remark           string  `json:"remark"`
	// SourceOrderID 采购单关联的客户订单，PI 带出箱费行时据此归属到订单
	// （财务利润明细按 order_id 分组，缺了它箱费会落到「未关联订单」）。
	SourceOrderID *uint64 `json:"source_order_id"`
}

// PurchaseOrderListItem 采购单列表项。
type PurchaseOrderListItem struct {
	ID                uint64    `json:"id"`
	PONumber          string    `json:"po_number"`
	SupplierName      string    `json:"supplier_name"`
	SourceOrderID     *uint64   `json:"source_order_id"`
	SourceOrderNumber string    `json:"source_order_number"`
	OrderDate         Date      `json:"order_date"`
	ExpectedDate      Date      `json:"expected_date"`
	Currency          string    `json:"currency"`
	Status            string    `json:"status"`
	GoodsAmount       float64   `json:"goods_amount"`
	FreightAmount     float64   `json:"freight_amount"`
	TotalAmount       float64   `json:"total_amount"`
	ItemCount         int       `json:"item_count"`
	ReceivedCount     int       `json:"received_count"`
	CreatedAt         time.Time `json:"created_at"`
}

// PurchaseItemSummary 采购报表行：多张采购单按物品汇总采购量与金额。
type PurchaseItemSummary struct {
	ItemType         string  `json:"item_type"`
	ItemCode         string  `json:"item_code"`
	ItemName         string  `json:"item_name"`
	Spec             string  `json:"spec"`
	Unit             string  `json:"unit"`
	RequiredQuantity float64 `json:"required_quantity"`
	Quantity         float64 `json:"quantity"`
	ReceivedQuantity float64 `json:"received_quantity"`
	Amount           float64 `json:"amount"`
}

// PurchaseOrderReceiveItem 单行到货数量。
type PurchaseOrderReceiveItem struct {
	ID               uint64  `json:"id"`
	ReceivedQuantity float64 `json:"received_quantity"`
}

// PurchaseOrderReceiveRequest 到货登记请求。
type PurchaseOrderReceiveRequest struct {
	Items []PurchaseOrderReceiveItem `json:"items"`
}

// PurchaseMaterialRequestItem 计算材料需求的产品行（数量按采购量或需求量）。
type PurchaseMaterialRequestItem struct {
	ItemType string  `json:"item_type"`
	ItemID   uint64  `json:"item_id"`
	Quantity float64 `json:"quantity"`
}

// PurchaseMaterialRequest 「采购材料」按钮的请求体：按产品数量推导纸箱等用料。
//
// SourceOrderID 为采购单关联的客户订单：有值时按订单需求把采购量拆成
// 「组合 / 单品」两部分，组合部分用组合纸箱折算；无值时整单按单品箱规折算。
type PurchaseMaterialRequest struct {
	SourceOrderID uint64                        `json:"source_order_id"`
	Items         []PurchaseMaterialRequestItem `json:"items"`
}

// PurchaseOrderSuggestion 由订单生成采购单时的建议（表头 + 需求明细）。
type PurchaseOrderSuggestion struct {
	OrderID     uint64               `json:"order_id"`
	OrderNumber string               `json:"order_number"`
	Customer    string               `json:"customer"`
	Country     string               `json:"country"`
	Currency    string               `json:"currency"`
	Items       []PurchaseDemandItem `json:"items"`
}
