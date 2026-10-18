package models

import "time"

type OrderItem struct {
	ID         uint64 `json:"id"`
	OrderID    uint64 `json:"order_id"`
	LineNumber int    `json:"line_number"`

	ProductName   string `json:"product_name"`
	Description   string `json:"description"`
	Specification string `json:"specification"`

	// MatchType 与 MatchID 是手动下单时选择的匹配产品：
	// MatchType 为 MatchTypeSKU 时 MatchID 是 sku_id（直接匹配），
	// 为 MatchTypeRule 时 MatchID 是 rule_id（组合匹配）。
	MatchType string `json:"match_type"`
	MatchID   uint64 `json:"match_id"`

	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`

	PackSize       float64 `json:"pack_size"`
	ActualQuantity float64 `json:"actual_quantity"`

	DeclaredQuantity string `json:"declared_quantity"`

	CreatedAt time.Time `json:"created_at"`

	// Matches 是确认入库时写入 order_item_matches 的匹配结果。
	Matches []OrderItemMatch `json:"matches"`
}
