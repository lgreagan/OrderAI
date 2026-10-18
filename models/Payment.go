package models

import (
	"errors"
	"time"
)

// ErrPaymentNotFound 表示收款记录不存在。
var ErrPaymentNotFound = errors.New("收款记录不存在")

// Payment 收款记录（可按客户、可选关联某张 PI）。
type Payment struct {
	ID           uint64    `json:"id"`
	CustomerName string    `json:"customer_name"`
	PIID         *uint64   `json:"pi_id"`
	PINumber     string    `json:"pi_number"`
	Amount       float64   `json:"amount"`
	Currency     string    `json:"currency"`
	PaidDate     Date      `json:"paid_date"`
	Method       string    `json:"method"`
	Reference    string    `json:"reference"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}
