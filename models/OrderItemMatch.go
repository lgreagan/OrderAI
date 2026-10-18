package models

import "time"

// OrderItemMatch 订单行匹配结果表
type OrderItemMatch struct {
	ID                  uint64    `json:"id"`
	OrderItemID         uint64    `json:"order_item_id"`
	SkuID               uint64    `json:"sku_id"`
	RuleID              *uint64   `json:"rule_id"`
	QuantityPerSet      float64   `json:"quantity_per_set"`
	OrderedQuantity     float64   `json:"ordered_quantity"`
	RequiredQuantity    float64   `json:"required_quantity"`
	SkuCodeSnapshot     string    `json:"sku_code_snapshot"`
	ProductNameSnapshot string    `json:"product_name_snapshot"`
	ModelSnapshot       string    `json:"model_snapshot"`
	ColorSnapshot       string    `json:"color_snapshot"`
	RuleCodeSnapshot    string    `json:"rule_code_snapshot"`
	CreatedAt           time.Time `json:"created_at"`
}

// 匹配方式，供前端展示使用。
const (
	MatchTypeSKU  = "sku"  // 单品直配
	MatchTypeRule = "rule" // 组合规则展开
)

// OrderItemMatchSku 匹配结果中的一个单品 SKU。
type OrderItemMatchSku struct {
	SkuID            uint64  `json:"sku_id"`
	SkuCode          string  `json:"sku_code"`
	ProductName      string  `json:"product_name"`
	ProductNameZH    string  `json:"product_name_zh"`
	Model            string  `json:"model"`
	Color            string  `json:"color"`
	Status           string  `json:"status"`
	QuantityPerSet   float64 `json:"quantity_per_set"`
	RequiredQuantity float64 `json:"required_quantity"`
}

// OrderItemMatchPreview 订单行匹配预览结果，用于确认入库之前展示给用户。
type OrderItemMatchPreview struct {
	LineNumber     int     `json:"line_number"`
	ProductName    string  `json:"product_name"`
	Specification  string  `json:"specification"`
	Quantity       float64 `json:"quantity"`
	Unit           string  `json:"unit"`
	ActualQuantity float64 `json:"actual_quantity"`

	Matched   bool   `json:"matched"`
	MatchType string `json:"match_type"`
	Message   string `json:"message"`

	RuleID        *uint64 `json:"rule_id"`
	RuleCode      string  `json:"rule_code"`
	SetsPerCarton float64 `json:"sets_per_carton"`

	// BaseQuantity 是换算成“套”之后的订单数量；
	// CartonConverted 表示订单单位是箱，已按每箱套数换算。
	BaseQuantity    float64 `json:"base_quantity"`
	CartonConverted bool    `json:"carton_converted"`

	SKUs []OrderItemMatchSku `json:"skus"`
}

// OrderMatchRequest 是匹配预览接口的请求体。
type OrderMatchRequest struct {
	Items []OrderItem `json:"items"`
}

// OrderMatchSummary 是匹配结果的行数统计。
type OrderMatchSummary struct {
	Total          int `json:"total"`
	DirectCount    int `json:"direct_count"`
	RuleCount      int `json:"rule_count"`
	UnmatchedCount int `json:"unmatched_count"`
}
