package model

import "time"

// Balance — ответ GET /api/user/balance
type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// Withdrawal — элемент списка GET /api/user/withdrawals
type Withdrawal struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

// WithdrawRequest — тело POST /api/user/balance/withdraw
type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}
