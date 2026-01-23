package model

import "time"

// BalanceDTO — ответ GET /api/user/balance
type BalanceDTO struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// WithdrawalDTO — элемент списка GET /api/user/withdrawals
type WithdrawalDTO struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

// WithdrawRequest — тело POST /api/user/balance/withdraw
type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}
