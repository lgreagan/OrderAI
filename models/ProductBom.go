package models

import (
	"errors"
	"time"
)

// ErrProductBomNotFound 表示产品用料记录不存在。
var ErrProductBomNotFound = errors.New("产品用料记录不存在")

// ProductBom 产品用料清单：某产品(组合规则/单品)每单位消耗的材料数量。
type ProductBom struct {
	ID              uint64    `json:"id"`
	ProductType     string    `json:"product_type"`
	ProductRefID    uint64    `json:"product_ref_id"`
	MaterialID      uint64    `json:"material_id"`
	QuantityPerUnit float64   `json:"quantity_per_unit"`
	Unit            string    `json:"unit"`
	Remark          string    `json:"remark"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// 展示用。
	ProductName  string `json:"product_name"`
	MaterialCode string `json:"material_code"`
	MaterialName string `json:"material_name"`
	MaterialSpec string `json:"material_spec"`
	// 材料分类（packaging / material），用于判断是否需要按整数计算用量。
	MaterialCategory string `json:"material_category"`
}
