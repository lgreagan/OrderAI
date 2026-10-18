package models

// FinanceFilter 财务报表筛选条件。
type FinanceFilter struct {
	DateFrom string // PI 签发日期起（含）
	DateTo   string // PI 签发日期止（含）
	Customer string // 客户名称模糊匹配
	Keyword  string // 订单号 / PI 号 / 客户 模糊匹配
}

// FinanceOrderRow 利润明细行。
//
// 一行一个客户订单；未关联客户订单的 PI 明细（如手工添加的箱费行）
// 按 PI 单独成行，此时 OrderID 为 0。
type FinanceOrderRow struct {
	OrderID     uint64 `json:"order_id"`
	OrderNumber string `json:"order_number"`
	Customer    string `json:"customer"`
	Country     string `json:"country"`
	PINumbers   string `json:"pi_numbers"`
	IssuedDate  Date   `json:"issued_date"`

	GoodsUSD     float64 `json:"goods_usd"`     // 货款收入（USD）
	BoxUSD       float64 `json:"box_usd"`       // 箱费收入（USD）
	FreightUSD   float64 `json:"freight_usd"`   // 向客户收取的运费（USD）
	InsuranceUSD float64 `json:"insurance_usd"` // 向客户收取的保险（USD）

	RevenueUSD float64 `json:"revenue_usd"` // 货款 + 箱费
	RevenueCNY float64 `json:"revenue_cny"` // 按该 PI 签发月份的汇率折算

	// ExchangeRate 是折算本行使用的人民币汇率；RateFallback 为 true 表示
	// 该月份没有维护月度汇率，已按系统设置里的默认汇率折算。
	ExchangeRate float64 `json:"exchange_rate"`
	RateFallback bool    `json:"rate_fallback"`

	StdCostCNY    float64 `json:"std_cost_cny"`    // 标准成本：数量 × 主数据成本价
	ActualCostCNY float64 `json:"actual_cost_cny"` // 实际成本：该订单采购单合计

	GrossStdCNY    float64 `json:"gross_std_cny"`
	GrossActualCNY float64 `json:"gross_actual_cny"`
	MarginStd      float64 `json:"margin_std"`    // 毛利率 0~1
	MarginActual   float64 `json:"margin_actual"` // 毛利率 0~1

	PurchaseCount  int    `json:"purchase_count"`
	PurchaseOrders string `json:"purchase_orders"` // 采购单号（| 分隔）

	ShipFreightUSD   float64 `json:"ship_freight_usd"`   // 发货实际支付的运费
	ShipInsuranceUSD float64 `json:"ship_insurance_usd"` // 发货实际支付的保险

	MissingGoodsCost int  `json:"missing_goods_cost"` // 缺 SKU 成本价的货款行数
	MissingBoxCost   int  `json:"missing_box_cost"`   // 缺材料单价或未关联材料的箱费行数
	NoPurchase       bool `json:"no_purchase"`        // 该订单没有采购单

	// PurchaseIncomplete 表示采购单数量少于 PI 出货数量（采购单未填全数量，
	// 或发货动用了历史库存），此时实际成本不完整，不参与实际成本口径。
	PurchaseIncomplete bool `json:"purchase_incomplete"`
}

// FinanceMonthlyRow 月度利润汇总行。
//
// 实际成本口径只统计已下采购单的订单，ActualOrderCount 表示其中已采购的订单数。
type FinanceMonthlyRow struct {
	Month            string  `json:"month"` // yyyy-MM
	ExchangeRate     float64 `json:"exchange_rate"`
	RateFallback     bool    `json:"rate_fallback"`
	OrderCount       int     `json:"order_count"`
	ActualOrderCount int     `json:"actual_order_count"`
	RevenueCNY       float64 `json:"revenue_cny"`
	StdCostCNY       float64 `json:"std_cost_cny"`
	ActualCostCNY    float64 `json:"actual_cost_cny"`
	GrossStdCNY      float64 `json:"gross_std_cny"`
	GrossActualCNY   float64 `json:"gross_actual_cny"`
	MarginStd        float64 `json:"margin_std"`
	MarginActual     float64 `json:"margin_actual"`
	ActualRevenueCNY float64 `json:"actual_revenue_cny"`
}

// FinanceSummary 财务报表汇总（顶部卡片）。
type FinanceSummary struct {
	OrderCount int `json:"order_count"`

	// DefaultExchangeRate 是系统设置里的默认汇率；EffectiveRate 是本期折算结果
	// 反算出的加权平均汇率（收入 CNY ÷ 收入 USD），便于快速核对。
	DefaultExchangeRate float64 `json:"default_exchange_rate"`
	EffectiveRate       float64 `json:"effective_rate"`

	// FallbackMonths 列出本期数据里未维护月度汇率、已按默认汇率折算的月份。
	FallbackMonths []string `json:"fallback_months"`

	RevenueUSD float64 `json:"revenue_usd"`
	RevenueCNY float64 `json:"revenue_cny"`

	StdCostCNY    float64 `json:"std_cost_cny"`
	ActualCostCNY float64 `json:"actual_cost_cny"`

	GrossStdCNY    float64 `json:"gross_std_cny"`
	GrossActualCNY float64 `json:"gross_actual_cny"`
	MarginStd      float64 `json:"margin_std"`
	MarginActual   float64 `json:"margin_actual"`

	FreightUSD       float64 `json:"freight_usd"`
	InsuranceUSD     float64 `json:"insurance_usd"`
	ShipFreightUSD   float64 `json:"ship_freight_usd"`
	ShipInsuranceUSD float64 `json:"ship_insurance_usd"`

	MissingGoodsCost int `json:"missing_goods_cost"`
	MissingBoxCost   int `json:"missing_box_cost"`
	NoPurchaseCount  int `json:"no_purchase_count"`
	// IncompletePurchaseCount 是采购单数量未覆盖 PI 出货数量、被排除在实际口径外的订单数。
	IncompletePurchaseCount int `json:"incomplete_purchase_count"`

	// ActualOrderCount / ActualRevenueCNY 描述实际成本口径的覆盖范围：
	// 只有已下采购单的订单才参与该口径，毛利率也只用这部分收入作分母。
	ActualOrderCount int     `json:"actual_order_count"`
	ActualRevenueCNY float64 `json:"actual_revenue_cny"`

	PI FinancePISummary `json:"pi"`
}

// FinancePISummary PI 应收汇总。
type FinancePISummary struct {
	Count     int     `json:"count"`
	AmountUSD float64 `json:"amount_usd"`
	PaidUSD   float64 `json:"paid_usd"`
	UnpaidUSD float64 `json:"unpaid_usd"`
}

// FinanceShippedItem 某订单下某个 SKU 的出货情况（含展示用产品信息）。
type FinanceShippedItem struct {
	OrderID  uint64  `json:"order_id"`
	SkuID    uint64  `json:"sku_id"`
	Quantity float64 `json:"quantity"`
	SkuCode  string  `json:"sku_code"`
	SkuName  string  `json:"sku_name"`
	Spec     string  `json:"spec"`
	Unit     string  `json:"unit"`
}

// FinancePurchasedItem 某订单下某个 SKU 的采购情况。
type FinancePurchasedItem struct {
	OrderID   uint64  `json:"order_id"`
	SkuID     uint64  `json:"sku_id"`
	Quantity  float64 `json:"quantity"`
	PONumbers string  `json:"po_numbers"`
}

// FinanceShortageRow 采购缺口明细行：某订单某 SKU 还需补采购的数量。
//
// 缺口 = PI 出货数量 − 已采购数量，供采购照着补单。
type FinanceShortageRow struct {
	OrderID      uint64  `json:"order_id"`
	OrderNumber  string  `json:"order_number"`
	Customer     string  `json:"customer"`
	PINumbers    string  `json:"pi_numbers"`
	SkuID        uint64  `json:"sku_id"`
	SkuCode      string  `json:"sku_code"`
	SkuName      string  `json:"sku_name"`
	Spec         string  `json:"spec"`
	Unit         string  `json:"unit"`
	ShippedQty   float64 `json:"shipped_qty"`
	PurchasedQty float64 `json:"purchased_qty"`
	ShortageQty  float64 `json:"shortage_qty"`
	PONumbers    string  `json:"po_numbers"` // 已存在该 SKU 的采购单，补数量就填在这些单里
}

// FinancePurchaseCost 某客户订单的采购成本汇总。
type FinancePurchaseCost struct {
	OrderID   uint64  `json:"order_id"`
	Count     int     `json:"count"`
	AmountCNY float64 `json:"amount_cny"`
	PONumbers string  `json:"po_numbers"`
}

// FinanceShipmentCost 某客户订单的发货实际支出（运费/保险，USD）。
type FinanceShipmentCost struct {
	OrderID      uint64  `json:"order_id"`
	FreightUSD   float64 `json:"freight_usd"`
	InsuranceUSD float64 `json:"insurance_usd"`
}

// FinanceShipmentCharge 单张发货单录入的运费/保险原值（币种 = 发货单 currency）。
//
// 报表要和 PI 的「向客户收取」同口径比较，必须在 Go 侧逐单折算：
// 每张发货单可能币种不同、跨月汇率也不同，SQL 里直接 SUM 会把人民币和美元加在一起。
type FinanceShipmentCharge struct {
	OrderID       uint64      `json:"order_id"`
	Currency      string      `json:"currency"`
	ShipDate      Date        `json:"ship_date"`
	FreightCost   float64     `json:"freight_cost"`
	InsuranceCost float64     `json:"insurance_cost"`
}

// FinanceReport 财务报表数据。
type FinanceReport struct {
	Rows    []FinanceOrderRow   `json:"rows"`
	Monthly []FinanceMonthlyRow `json:"monthly"`
	Summary FinanceSummary      `json:"summary"`

	// Shortages 列出实际成本口径无法覆盖的订单（未采购 / 采购未齐）缺哪些 SKU、差多少数量。
	Shortages []FinanceShortageRow `json:"shortages"`
}
