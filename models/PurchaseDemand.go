package models

import "time"

// PurchaseDemandSourceOrder 采购需求对应的来源订单（orders 表）。
type PurchaseDemandSourceOrder struct {
	ID             uint64    `json:"id"`
	OrderNumber    string    `json:"order_number"`
	Country        string    `json:"country"`
	ShippingMethod string    `json:"shipping_method"`
	Customer       string    `json:"customer"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// PurchaseDemandItem 订单的采购需求明细：按单品 SKU 汇总。
//
// StockQuantity / InTransitQuantity / SuggestedQuantity 供"生成采购单"时给出建议采购量：
// 建议量 = max(0, 客户需求 − 当前库存 − 在途采购)。
type PurchaseDemandItem struct {
	SkuID            uint64  `json:"sku_id"`
	SkuCode          string  `json:"sku_code"`
	ProductName      string  `json:"product_name"`
	ProductNameZH    string  `json:"product_name_zh"`
	ShortName        string  `json:"short_name"`
	Model            string  `json:"model"`
	Color            string  `json:"color"`
	Status           string  `json:"status"`
	Price            float64 `json:"price"`
	PriceUSD         float64 `json:"price_usd"`
	Currency         string  `json:"currency"`
	Unit             string  `json:"unit"`
	RequiredQuantity float64 `json:"required_quantity"`
	LineCount        int     `json:"line_count"`

	StockQuantity     float64 `json:"stock_quantity"`
	InTransitQuantity float64 `json:"in_transit_quantity"`
	SuggestedQuantity float64 `json:"suggested_quantity"`
}

// PurchaseDemandDetail 某订单的采购需求（表头 + 明细）。
type PurchaseDemandDetail struct {
	Order PurchaseDemandSourceOrder `json:"order"`
	Items []PurchaseDemandItem      `json:"items"`
}

// PurchaseDemandLine 客户订单的一行需求（一对一，不做汇总）。
//
// 纸箱按源订单的需求量折算，与采购量无关：
// 单品行需求个数 ÷ SKU 每箱数量，组合行套数 ÷ 组合每箱套数。
//
// 数量取 RequiredQuantity（order_item_matches.required_quantity）：
// order_items.quantity 是下单口径（480 × "2 Pack"），不是单品个数。
//
// RuleID 为 0 表示该行是单品直配（order_item_matches.rule_id 为空）；
// 组合行按成员单品展开成多条，需按 OrderItemID 去重。
type PurchaseDemandLine struct {
	OrderItemID      uint64  `json:"order_item_id"`
	SkuID            uint64  `json:"sku_id"`
	RuleID           uint64  `json:"rule_id"`
	RuleCode         string  `json:"rule_code"`
	RuleNameZH       string  `json:"rule_name_zh"`
	SetsPerCarton    float64 `json:"sets_per_carton"`
	RequiredQuantity float64 `json:"required_quantity"`
	QuantityPerSet   float64 `json:"quantity_per_set"`
	// Quantity 下单口径的数量（组合行通常等于套数），仅在需求量为 0 时兜底。
	Quantity float64 `json:"quantity"`
}
