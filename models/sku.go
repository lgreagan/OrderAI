package models

import "time"

type SKU struct {
	ID uint64 `json:"id"`
	// SortOrder 产品排序值：产品列表统一按它升序展示，值越小越靠前。
	SortOrder int `json:"sort_order"`

	SKUCode       string `json:"sku_code"`
	ProductName   string `json:"product_name"`
	ProductNameZH string `json:"product_name_zh"`

	// PI 展示相关：产品简称(如 V1-8cm blue)、系列、尺寸、客户称呼的颜色。
	ShortName string `json:"short_name"`
	Series    string `json:"series"`
	SizeSpec  string `json:"size_spec"`
	PiColor   string `json:"pi_color"`

	Model    string  `json:"model"`
	Price    float64 `json:"price"`     // 成本价
	PriceUSD float64 `json:"price_usd"` // PI 美金售价
	// DeclarePrice 报关申报单价（USD）：报关资料每行优先用它；
	// 为 0 时回落到 SKU 挂的报关品类默认申报价（product_categories.declare_price）。
	DeclarePrice float64 `json:"declare_price"`
	Currency     string  `json:"currency"`
	Image        string  `json:"image"`
	Color        string  `json:"color"`

	PackQuantity float64 `json:"pack_quantity"`
	Unit         string  `json:"unit"`

	BoxLength  float64 `json:"box_length"`
	BoxWidth   float64 `json:"box_width"`
	BoxHeight  float64 `json:"box_height"`
	BoxUnit    string  `json:"box_unit"`
	BoxWeight  float64 `json:"box_weight"`
	WeightUnit string  `json:"weight_unit"`

	HSCode   string `json:"hs_code"`
	Category string `json:"category"` // finished/packaging/material（物料形态）

	// ASIN/FNSKU 亚马逊商品标识（报关模板上的「产品ASIN/产品FNSKU」列）。
	ASIN  string `json:"asin"`
	FNSKU string `json:"fnsku"`

	// ProductCategoryID 报关品类（→ product_categories.id），报关资料按品类出行。
	ProductCategoryID uint64 `json:"product_category_id"`
	// CategoryName 报关品类中文名（联表展示用，不落库）。
	CategoryName string `json:"category_name"`

	Status      string    `json:"status"`
	Description string    `json:"description"`
	Remark      string    `json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
