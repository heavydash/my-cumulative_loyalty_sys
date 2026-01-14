package model

import "time"

type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "new"
	OrderStatusProcessing OrderStatus = "processing"
	OrderStatusInvalid    OrderStatus = "invalid"
	OrderStatusProcessed  OrderStatus = "processed"
)

type Order struct {
	ID         int64       `db:"id" json:"-"`
	Number     string      `db:"number" json:"number"`
	UserID     int64       `db:"user_id" json:"-"`
	Status     OrderStatus `db:"status" json:"status"`
	Accrual    float64     `db:"accrual" json:"accrual,omitempty"`
	UploadedAt time.Time   `db:"uploaded_at" json:"uploaded_at"`
}

type OrderDTO struct {
	Number     string  `json:"number"`
	Status     string  `json:"status"`
	Accrual    float64 `json:"accrual,omitempty"`
	UploadedAT string  `json:"uploaded_at"`
}
