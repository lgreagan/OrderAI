package models

import (
	"errors"
	"time"
)

// ErrProformaInvoiceNotFound 表示形式发票不存在。
var ErrProformaInvoiceNotFound = errors.New("形式发票不存在")

// PI 状态。
const (
	PIStatusDraft  = "draft"
	PIStatusIssued = "issued"
	PIStatusClosed = "closed"
)

// PI 明细类型。
const (
	PIItemGroup  = "group"  // 分组标题
	PIItemGoods  = "goods"  // 货款行
	PIItemBox    = "box"    // 箱费行
	PIItemCharge = "charge" // 费用行（保险/运费/其它）
	PIItemNote   = "note"   // 备注文本行
)

// PI 费用类型。
const (
	PIChargeInsurance = "insurance"
	PIChargeShipping  = "shipping"
	PIChargeOther     = "other"
)

// ProformaInvoice 形式发票头。
type ProformaInvoice struct {
	ID              uint64    `json:"id"`
	PINumber        string    `json:"pi_number"`
	CustomerName    string    `json:"customer_name"`
	IssuedDate      Date      `json:"issued_date"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	ItemAmount      float64   `json:"item_amount"`
	BoxAmount       float64   `json:"box_amount"`
	InsuranceAmount float64   `json:"insurance_amount"`
	FreightAmount   float64   `json:"freight_amount"`
	Amount          float64   `json:"amount"`
	CarryoverAmount float64   `json:"carryover_amount"`
	TotalUnpaid     float64   `json:"total_unpaid"`
	PaidAmount      float64   `json:"paid_amount"`
	StockNote       string    `json:"stock_note"`
	Remark          string    `json:"remark"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	Items []ProformaInvoiceItem `json:"items"`
}

// ProformaInvoiceItem PI 明细行（分组/货款/箱费/费用/备注统一存放）。
type ProformaInvoiceItem struct {
	ID          uint64  `json:"id"`
	PIID        uint64  `json:"pi_id"`
	ItemType    string  `json:"item_type"`
	GroupNo     int     `json:"group_no"`
	GroupTitle  string  `json:"group_title"`
	OrderID     *uint64 `json:"order_id"`
	ShipmentID  *uint64 `json:"shipment_id"`
	SkuID       *uint64 `json:"sku_id"`
	MaterialID  *uint64 `json:"material_id"`
	Model       string  `json:"model"`
	ShortName   string  `json:"short_name"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	IsPaid      bool    `json:"is_paid"`
	ChargeType  string  `json:"charge_type"`
	Title       string  `json:"title"`
	SortOrder   int     `json:"sort_order"`
	Remark      string  `json:"remark"`
}

// ProformaInvoiceListItem PI 列表项。
type ProformaInvoiceListItem struct {
	ID              uint64    `json:"id"`
	PINumber        string    `json:"pi_number"`
	CustomerName    string    `json:"customer_name"`
	IssuedDate      Date      `json:"issued_date"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	ItemAmount      float64   `json:"item_amount"`
	BoxAmount       float64   `json:"box_amount"`
	InsuranceAmount float64   `json:"insurance_amount"`
	FreightAmount   float64   `json:"freight_amount"`
	Amount          float64   `json:"amount"`
	CarryoverAmount float64   `json:"carryover_amount"`
	TotalUnpaid     float64   `json:"total_unpaid"`
	PaidAmount      float64   `json:"paid_amount"`
	CreatedAt       time.Time `json:"created_at"`
}

// ProformaInvoiceGenerateRequest 生成 PI 的请求。
//
// 优先按 order_ids 指定的订单，其次按 customer_name / date 范围筛选。
type ProformaInvoiceGenerateRequest struct {
	CustomerName string   `json:"customer_name"`
	OrderIDs     []uint64 `json:"order_ids"`
	DateFrom     string   `json:"date_from"`
	DateTo       string   `json:"date_to"`
}
