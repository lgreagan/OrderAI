package models

import (
	"errors"
	"time"
)

// 订单状态，按履约顺序推进：
// 已确认 → 已采购 → 已生产 → 已发货 → 已收款 → 完结。
//
// 已采购之后（含已采购）表示采购已完成，不再计入采购订单的汇总统计。
const (
	OrderStatusConfirmed = "confirmed" // 已确认
	OrderStatusPurchased = "purchased" // 已采购
	OrderStatusProduced  = "produced"  // 已生产
	OrderStatusShipped   = "shipped"   // 已发货
	OrderStatusPaid      = "paid"      // 已收款
	OrderStatusCompleted = "completed" // 完结
)

// ErrOrderNotFound 表示订单不存在。
var ErrOrderNotFound = errors.New("订单不存在")

// orderStatuses 是全部有效的订单状态。
var orderStatuses = []string{
	OrderStatusConfirmed,
	OrderStatusPurchased,
	OrderStatusProduced,
	OrderStatusShipped,
	OrderStatusPaid,
	OrderStatusCompleted,
}

// IsValidOrderStatus 判断订单状态是否有效。
func IsValidOrderStatus(status string) bool {
	for _, value := range orderStatuses {
		if status == value {
			return true
		}
	}

	return false
}

type Order struct {
	ID             uint64    `json:"id"`
	OrderNumber    string    `json:"order_number"`
	Country        string    `json:"country"`
	ShippingMethod string    `json:"shipping_method"`
	Customer       string    `json:"customer"`
	Status         string    `json:"status"`
	SourceImage    string    `json:"source_image"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// RecognitionImage 是提交给 AI 识别的那张原图（ai_results.image_path，
	// 缺失时回退 source_image）。仅列表展示用，不参与入库。
	RecognitionImage string `json:"recognition_image"`

	Items []OrderItem `json:"items"`
}

type ConfirmOrderRequest struct {
	Order     Order  `json:"order"`
	RawJSON   string `json:"raw_json"`
	ImagePath string `json:"image_path"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
}