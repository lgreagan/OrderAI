package models

import "time"

// ProductMatchRuleItem 组合匹配规则明细表
type ProductMatchRuleItem struct {
	ID             uint64    `json:"id"`
	RuleID         uint64    `json:"rule_id"`
	SkuID          uint64    `json:"sku_id"`
	QuantityPerSet float64   `json:"quantity_per_set"`
	SortOrder      int       `json:"sort_order"`
	Remark         string    `json:"remark"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ProductMatchRuleItemDetail 组合规则明细，附带关联单品 SKU 的展示信息。
type ProductMatchRuleItemDetail struct {
	ProductMatchRuleItem
	SKUCode       string `json:"sku_code"`
	ProductName   string `json:"product_name"`
	ProductNameZH string `json:"product_name_zh"`
	Model         string `json:"model"`
	Color         string `json:"color"`
	Status        string `json:"status"`
}
