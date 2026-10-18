package models

import "time"

// ProductionDemandTaskItem 组合套的组成单品。
type ProductionDemandTaskItem struct {
	SkuID            uint64  `json:"sku_id"`
	SkuCode          string  `json:"sku_code"`
	ProductName      string  `json:"product_name"`
	ProductNameZH    string  `json:"product_name_zh"`
	ShortName        string  `json:"short_name"`
	Model            string  `json:"model"`
	Color            string  `json:"color"`
	PackQuantity     float64 `json:"pack_quantity"`
	QuantityPerSet   float64 `json:"quantity_per_set"`
	RequiredQuantity float64 `json:"required_quantity"`
}

// ProductionDemandTask 命中组合规则的订单行任务（以"套"为生产单位）。
type ProductionDemandTask struct {
	OrderItemID uint64 `json:"order_item_id"`
	OrderID     uint64 `json:"order_id"`
	OrderNumber string `json:"order_number"`
	LineNumber  int    `json:"line_number"`

	Customer       string    `json:"customer"`
	Country        string    `json:"country"`
	OrderCreatedAt time.Time `json:"order_created_at"`
	ProductName    string    `json:"product_name"`
	Specification  string    `json:"specification"`
	Quantity       float64   `json:"quantity"`
	Unit           string    `json:"unit"`

	RuleID            *uint64 `json:"rule_id"`
	RuleCode          string  `json:"rule_code"`
	RuleProductName   string  `json:"rule_product_name"`
	RuleProductNameZH string  `json:"rule_product_name_zh"`
	OrderUnit         string  `json:"order_unit"`
	SetsPerCarton     float64 `json:"sets_per_carton"`

	Sets        float64 `json:"sets"`
	CartonCount float64 `json:"carton_count"`
	SKUCount    int     `json:"sku_count"`

	Items []ProductionDemandTaskItem `json:"items"`
}

// ProductionDemandSourceOrder 生产需求对应的来源订单。
type ProductionDemandSourceOrder struct {
	ID             uint64    `json:"id"`
	OrderNumber    string    `json:"order_number"`
	Country        string    `json:"country"`
	ShippingMethod string    `json:"shipping_method"`
	Customer       string    `json:"customer"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}
