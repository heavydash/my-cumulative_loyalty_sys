package storage

import (
	"context"
	"database/sql"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/pkg/errors"
)

type OrderStorage struct {
	db *sql.DB
}

func NewOrderStorage(db *sql.DB) *OrderStorage {
	return &OrderStorage{db: db}
}

func (s *OrderStorage) AddOrder(ctx context.Context, userID int64, number string) error {
	var existingUserId int64

	err := s.db.QueryRowContext(ctx, "SELECT user_id FROM orders WHERE number = $1", number).
		Scan(&existingUserId)
	if err == nil {
		if existingUserId == userID {
			return errors.New("order already added by user")
		}
		return errors.New("order already added by another user")
	} else if err != sql.ErrNoRows {
		return err
	}

	_, err = s.db.ExecContext(ctx, "INSERT INTO orders (number, user_id) VALUES ($1, $2)", number, userID)
	if err != nil {
		return err

	}
	return nil
}

func (s *OrderStorage) GetOrders(ctx context.Context, userID int64) ([]model.Order, error) {

	rows, err := s.db.QueryContext(ctx, "SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var order model.Order
		err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}
