package models

// StatCount 分组计数。
type StatCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// StockAlert 库存预警行（负库存 / 低库存）。
type StockAlert struct {
	ItemType string  `json:"item_type"`
	ItemID   uint64  `json:"item_id"`
	ItemCode string  `json:"item_code"`
	ItemName string  `json:"item_name"`
	Stock    float64 `json:"stock"`
}

// StatisticsOverview 数据统计总览。
type StatisticsOverview struct {
	OrderTotal    int         `json:"order_total"`
	OrderByStatus []StatCount `json:"order_by_status"`

	PurchaseCount  int     `json:"purchase_count"`
	PurchaseAmount float64 `json:"purchase_amount"`

	ProductionCount    int         `json:"production_count"`
	ProductionByStatus []StatCount `json:"production_by_status"`

	ShipmentCount  int     `json:"shipment_count"`
	FreightTotal   float64 `json:"freight_total"`
	InsuranceTotal float64 `json:"insurance_total"`

	PICount  int     `json:"pi_count"`
	PIAmount float64 `json:"pi_amount"`
	PIUnpaid float64 `json:"pi_unpaid"`
	PIPaid   float64 `json:"pi_paid"`

	PaymentCount  int     `json:"payment_count"`
	PaymentAmount float64 `json:"payment_amount"`

	SkuCount      int `json:"sku_count"`
	MaterialCount int `json:"material_count"`

	LowStockThreshold float64      `json:"low_stock_threshold"`
	NegativeStock     []StockAlert `json:"negative_stock"`
	LowStock          []StockAlert `json:"low_stock"`
}
