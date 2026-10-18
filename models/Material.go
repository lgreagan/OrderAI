package models

import (
	"errors"
	"time"
)

// ErrMaterialNotFound 表示材料不存在。
var ErrMaterialNotFound = errors.New("材料不存在")

// 材料分类。
const (
	MaterialCategoryPackaging = "packaging" // 包材（箱子/标签）
	MaterialCategoryMaterial  = "material"  // 原材料
)

// 材料需求的折算依据。
const (
	// MaterialBasisOrder 按源订单需求量折算（与采购量无关）。
	MaterialBasisOrder = "order"
	// MaterialBasisPurchase 按采购量折算（没有来源订单时的回退口径）。
	MaterialBasisPurchase = "purchase"
)

// MaterialRequirement 由产品数量推导的材料需求（按材料汇总）。
// 用于采购单「采购材料」按钮带出纸箱等用量，也可扩展包材。
type MaterialRequirement struct {
	MaterialID   uint64  `json:"material_id"`
	MaterialCode string  `json:"material_code"`
	MaterialName string  `json:"material_name"`
	Spec         string  `json:"spec"`
	Unit         string  `json:"unit"`
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price"`
	ProductCount int     `json:"product_count"`
	// PackPerCarton 每箱数量（由用料清单用量反推 = 1 / 每单位用量），
	// 仅用于界面展示"需求量 ÷ 每箱数量"的折算依据；
	// 同一材料涉及多个产品且每箱数量不一致时为 0（无法用单一数字表达）。
	PackPerCarton float64 `json:"pack_per_carton,omitempty"`

	// PackUnit 每箱数量的计量单位：单品纸箱是"个"，组合纸箱是"套"。
	// 只影响折算依据的文案（"每箱 72 套"），不参与计算。
	PackUnit string `json:"pack_unit,omitempty"`

	// DemandQuantity 参与折算的产品数量：源订单口径下是订单需求量（个 / 套），
	// 回退口径下是采购量。仅用于界面显示折算依据。
	DemandQuantity float64 `json:"demand_quantity,omitempty"`

	// Basis 折算依据：order=按源订单需求（不随采购量变化），purchase=按采购量。
	Basis string `json:"basis,omitempty"`
}

// ShipmentPackagingItem 发货明细中的产品数量，用于核算随货消耗的包装材料。
type ShipmentPackagingItem struct {
	SkuID    uint64  `json:"sku_id"`
	Quantity float64 `json:"quantity"`
}

// ShipmentPackagingRequest 发货包装材料核算请求（只读预览，不写库存）。
type ShipmentPackagingRequest struct {
	Items []ShipmentPackagingItem `json:"items"`
}

// Material 材料主数据（包材/原材料），PI 的箱费行取这里的 price。
type Material struct {
	ID           uint64    `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	NameZH       string    `json:"name_zh"`
	ShortName    string    `json:"short_name"`
	Category     string    `json:"category"`
	Spec         string    `json:"spec"`
	Unit         string    `json:"unit"`
	Price        float64   `json:"price"`
	Currency     string    `json:"currency"`
	SupplierName string    `json:"supplier_name"`
	SafetyStock  float64   `json:"safety_stock"`
	Status       string    `json:"status"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
