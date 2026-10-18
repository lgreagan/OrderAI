package models

type AIOrderResult struct {
	OrderNumber    string        `json:"order_number"`
	Country        string        `json:"country"`
	ShippingMethod string        `json:"shipping_method"`
	Customer       string        `json:"customer"`
	Items          []AIOrderItem `json:"items"`
	Notes          []string      `json:"notes"`
}

type AIOrderItem struct {
	LineNumber       int     `json:"line_number"`
	ProductName      string  `json:"product_name"`
	Description      string  `json:"description"`
	Specification    string  `json:"specification"`
	Quantity         float64 `json:"quantity"`
	Unit             string  `json:"unit"`
	PackSize         float64 `json:"pack_size"`
	ActualQuantity   float64 `json:"actual_quantity"`
	DeclaredQuantity string  `json:"declared_quantity"`
}
