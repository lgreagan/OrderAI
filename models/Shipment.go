package models

import (
	"errors"
	"time"
)

// ErrShipmentNotFound 表示发货单不存在。
var ErrShipmentNotFound = errors.New("发货单不存在")

// 发货单状态。
const (
	ShipmentStatusDraft     = "draft"
	ShipmentStatusShipped   = "shipped"
	ShipmentStatusDelivered = "delivered"
)

// 发货明细来源。
const (
	ShipmentSourceStock      = "stock"      // 库存
	ShipmentSourceProduction = "production" // 新生产
)

// Shipment 发货单：一张客户订单可拆成多批发货。
type Shipment struct {
	ID             uint64    `json:"id"`
	ShipmentNumber string    `json:"shipment_number"`
	OrderID        uint64    `json:"order_id"`
	BatchNo        int       `json:"batch_no"`
	ShipDate       Date      `json:"ship_date"`
	ShippingMethod string    `json:"shipping_method"`
	Carrier        string    `json:"carrier"`
	TrackingNo     string    `json:"tracking_no"`
	CartonCount    float64   `json:"carton_count"`
	FreightCost    float64   `json:"freight_cost"`
	InsuranceCost  float64   `json:"insurance_cost"`
	Currency       string    `json:"currency"`
	Status         string    `json:"status"`
	Remark         string    `json:"remark"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// ---------- 报关资料（货代 xls 模板）抬头字段 ----------
	WarehouseCode   string `json:"warehouse_code"`   // 地址库编码(→warehouse_addresses.short_name)
	PONumber        string `json:"po_number"`        // PO Number
	CustomsMode     string `json:"customs_mode"`     // 报关方式(买单报关/报关退税)
	TaxMode         string `json:"tax_mode"`         // 交税方式(包税/不包税/自主税号/自税递延)
	DeclareCurrency string `json:"declare_currency"` // 申报币种
	IsElectric      bool   `json:"is_electric"`      // 带电
	IsMagnetic      bool   `json:"is_magnetic"`      // 带磁
	IsLiquid        bool   `json:"is_liquid"`        // 液体
	IsPowder        bool   `json:"is_powder"`        // 粉末
	IsDanger        bool   `json:"is_danger"`        // 危险品
	NeedInsurance   bool   `json:"need_insurance"`   // 购买保险
	TemplateID      uint64 `json:"template_id"`      // 上次使用的报关模板

	// 关联订单展示字段（来自 orders）。
	OrderNumber   string `json:"order_number"`
	Customer      string `json:"customer"`
	Country       string `json:"country"`
	OrderShipping string `json:"order_shipping"`

	Items []ShipmentItem `json:"items"`
}

// ShipmentItem 发货明细行（产品 SKU 或材料）。
type ShipmentItem struct {
	ID           uint64  `json:"id"`
	ShipmentID   uint64  `json:"shipment_id"`
	OrderItemID  *uint64 `json:"order_item_id"`
	ItemType     string  `json:"item_type"`
	SkuID        *uint64 `json:"sku_id"`
	MaterialID   *uint64 `json:"material_id"`
	ShortName    string  `json:"short_name"`
	Description  string  `json:"description"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	PackQuantity float64 `json:"pack_quantity"`
	CartonCount  float64 `json:"carton_count"`
	Source       string  `json:"source"`

	// ---------- 报关资料（货代 xls 模板）用字段 ----------
	CartonNo     string  `json:"carton_no"`     // 货箱编号/FBA货件号
	PONumber     string  `json:"po_number"`     // PO Number(卡派必填)
	CartonWeight float64 `json:"carton_weight"` // 0 = 取 skus.box_weight
	CartonLength float64 `json:"carton_length"` // 0 = 取 skus.box_length
	CartonWidth  float64 `json:"carton_width"`  // 0 = 取 skus.box_width
	CartonHeight float64 `json:"carton_height"` // 0 = 取 skus.box_height

	UnitPrice float64 `json:"unit_price"`
	Amount    float64 `json:"amount"`
	IsPaid    bool    `json:"is_paid"`
	Remark    string  `json:"remark"`
}

// ShipmentListItem 发货单列表项。
type ShipmentListItem struct {
	ID             uint64  `json:"id"`
	ShipmentNumber string  `json:"shipment_number"`
	OrderID        uint64  `json:"order_id"`
	OrderNumber    string  `json:"order_number"`
	Customer       string  `json:"customer"`
	Country        string  `json:"country"`
	BatchNo        int     `json:"batch_no"`
	ShipDate       Date    `json:"ship_date"`
	ShippingMethod string  `json:"shipping_method"`
	CartonCount    float64 `json:"carton_count"`
	FreightCost    float64 `json:"freight_cost"`
	InsuranceCost  float64 `json:"insurance_cost"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	ItemCount      int     `json:"item_count"`
}

// OrderShipmentLine 某订单可发货行（订单行 × 匹配到的 SKU），用于新建发货单。
//
// 当订单行走**组合套装**（MatchType = MatchTypeRule）时，同一 OrderItemID 会展开出多行，
// 前端按「套装父行 + 组成单品子行」分组折叠展示；单品直配（MatchType = MatchTypeSKU）
// 则每个订单行只有一行。分组键始终是 OrderItemID。
type OrderShipmentLine struct {
	OrderItemID uint64 `json:"order_item_id"`
	LineNumber  int    `json:"line_number"`

	// MatchType 匹配方式：sku = 单品直配，rule = 组合套装的组成单品。
	MatchType string  `json:"match_type"`
	RuleID    *uint64 `json:"rule_id"`
	RuleCode  string  `json:"rule_code"`
	// RuleNameZH 组合规则的中文名（套装父行展示用，如「6厘米两件套」）。
	RuleNameZH string `json:"rule_name_zh"`

	SkuID         *uint64 `json:"sku_id"`
	SkuCode       string  `json:"sku_code"`
	ShortName     string  `json:"short_name"`
	ProductNameZH string  `json:"product_name_zh"`

	// CustomerProductName / Specification 是客户订单原件写法（备查，默认不展示）。
	CustomerProductName string `json:"customer_product_name"`
	Specification       string `json:"specification"`

	// ---- 组合套装信息（MatchType = rule 时有效）----
	OrderQuantity  float64 `json:"order_quantity"`   // 订单行数量（套）
	OrderUnit      string  `json:"order_unit"`       // 订单行计量单位（套）
	QuantityPerSet float64 `json:"quantity_per_set"` // 每套含该 SKU 的数量
	SetsPerCarton  float64 `json:"sets_per_carton"`  // 组合每箱套数（组合箱规）

	RequiredQuantity  float64 `json:"required_quantity"`
	ShippedQuantity   float64 `json:"shipped_quantity"`
	RemainingQuantity float64 `json:"remaining_quantity"`
	// PackQuantity 每箱数量：单品直配取 SKU 箱规；组合套装取
	// sets_per_carton × quantity_per_set（即「每箱装 N 套，每套 M 个」折算出的每箱个数），
	// 否则箱数会按单品箱规算错。
	PackQuantity float64 `json:"pack_quantity"`
	UnitPrice    float64 `json:"unit_price"`
	Unit         string  `json:"unit"`
}

// OrderShipmentContext 新建发货单时返回的订单上下文。
type OrderShipmentContext struct {
	OrderID        uint64              `json:"order_id"`
	OrderNumber    string              `json:"order_number"`
	Customer       string              `json:"customer"`
	Country        string              `json:"country"`
	ShippingMethod string              `json:"shipping_method"`
	NextBatchNo    int                 `json:"next_batch_no"`
	Lines          []OrderShipmentLine `json:"lines"`
}
