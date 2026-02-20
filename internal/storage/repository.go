package storage

import (
	"context"

	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
)

type UserRepository interface {
	Create(ctx context.Context, login, password string) (*model.User, error)
	GetByLogin(ctx context.Context, login string) (*model.User, error)
}

type OrderRepository interface {
	AddOrder(ctx context.Context, userID int64, number string) error
	GetOrders(ctx context.Context, userID int64) ([]model.Order, error)
}

type BalanceRepository interface {
	GetBalance(ctx context.Context, userID int64) (float64, float64, error)
	Withdraw(ctx context.Context, userID int64, orderNumber string, sum float64) error
	GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
}
