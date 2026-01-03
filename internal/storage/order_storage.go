package storage

import (
	"context"
	"database/sql"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/util"
	_ "github.com/heavydash/my-cumulative_loyalty_sys/internal/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var (
	ErrInvalidOrderNumber      = errors.New("invalid order number")
	ErrOrderAlreadyAddedByUser = errors.New("order already added by user")
	ErrOrderAddedAnotherUser   = errors.New("order added by another user")
	ErrOrderIsEmpty            = errors.New("order is empty")
)

type OrderStorage struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

func NewOrderStorage(db *sql.DB, logger *zap.SugaredLogger) *OrderStorage {
	return &OrderStorage{db: db, logger: logger}
}

func (s *OrderStorage) AddOrder(ctx context.Context, userID int64, number string) error {
	s.logger.Info("checking order existence", zap.String("number", number))

	if number == "" {
		return ErrOrderIsEmpty
	}

	if !util.IsLuhnValid(number) {
		return ErrInvalidOrderNumber
	}

	var existingUserId int64
	err := s.db.QueryRowContext(ctx, "SELECT user_id FROM orders WHERE number = $1", number).
		Scan(&existingUserId)
	// Обработки ошибок
	if err == nil {
		// Нашли номер
		s.logger.Info("found existing order", zap.Int64("existing_user_id",
			existingUserId), zap.Int64("current_user_id", userID))
		if existingUserId == userID {
			s.logger.Info("same user, return already added")
			return ErrOrderAlreadyAddedByUser // 200
		}
		s.logger.Info("same user, return already added")
		return ErrOrderAddedAnotherUser // 409
	}
	if err != sql.ErrNoRows {
		// Другая ошибка БД
		s.logger.Info("select failed", zap.Error(err))
		return err
	}
	// Не нашли, вставляем новый
	s.logger.Info("inserting new order", zap.String("number", number),
		zap.Int64("user_id", userID))
	_, err = s.db.ExecContext(ctx, "INSERT INTO orders (number, user_id, status, uploaded_at) VALUES ($1, $2, 'NEW', NOW())", number, userID)
	if err != nil {
		s.logger.Info("insert failed", zap.Error(err))
		return err
	}
	return nil // 202
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
