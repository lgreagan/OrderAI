package models

import (
	"errors"
	"time"
)

// ErrProductMatchRuleNotFound 表示组合匹配规则不存在。
var ErrProductMatchRuleNotFound = errors.New("组合规则不存在")

// ProductMatchRule 组合匹配规则主表
type ProductMatchRule struct {
	ID                    uint64  `json:"id"`
	RuleCode              string  `json:"rule_code"`
	CustomerProductName   string  `json:"customer_product_name"`
	CustomerSpecification string  `json:"customer_specification"`
	ProductNameZH         string  `json:"product_name_zh"`
	OrderUnit             string  `json:"order_unit"`
	SetsPerCarton         float64 `json:"sets_per_carton"`
	CartonWeight          float64 `json:"carton_weight"`
	// 删除 CartonSize，新增长宽高
	BoxLength float64   `json:"box_length"`
	BoxWidth  float64   `json:"box_width"`
	BoxHeight float64   `json:"box_height"`
	Status    string    `json:"status"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// ItemCount 与 Items 仅在列表查询时填充，用于页面展示组合明细。
	ItemCount int                          `json:"item_count"`
	Items     []ProductMatchRuleItemDetail `json:"items"`
}
