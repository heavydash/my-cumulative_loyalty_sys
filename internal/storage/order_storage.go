package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/util"
	"go.uber.org/zap"
)

type OrderStorage struct {
	repo   *GenericRepository[model.Order]
	db     *sql.DB
	logger *zap.SugaredLogger
}

func NewOrderStorage(db *sql.DB, logger *zap.SugaredLogger) *OrderStorage {
	scanCreate := func(row *sql.Row, order *model.Order) error {
		return row.Scan(&order.ID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
	}
	return &OrderStorage{
		db:     db,
		logger: logger,
		repo: NewGenericRepository[model.Order](
			db, "",
			"INSERT INTO orders (number, user_id, status, uploaded_at) VALUES ($1, $2, 'NEW', NOW()) RETURNING id, number, status, accrual, uploaded_at",
			scanCreate,
			"",
			"SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC",
		),
	}
}

// GetOrders - общий метод
func (s *OrderStorage) GetOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	// Запрос к БД - заказ полтзователя от новых к старым
	rows, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		s.logger.Errorw("failed get orders", "userID", userID, "err", err)
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var order model.Order
		if err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			s.logger.Errorw("failed to scan order", zap.Error(err))
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		s.logger.Errorw("rows iteration error during GetOrders", zap.Error(err))
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return orders, nil
}

func (s *OrderStorage) AddOrder(ctx context.Context, userID int64, number string) error {
	s.logger.Info("checking order existence", zap.String("number", number))

	if number == "" {
		return ErrOrderIsEmpty
	}

	if !util.IsLuhnValid(number) {
		return ErrInvalidOrderNumber
	}

	var existingUserID int64
	err := s.db.QueryRowContext(ctx, "SELECT user_id FROM orders WHERE number = $1", number).
		Scan(&existingUserID)
	// Обработки ошибок
	if err == nil {
		// Нашли номер
		s.logger.Info("found existing order", zap.Int64("existing_user_id",
			existingUserID), zap.Int64("current_user_id", userID))
		if existingUserID == userID {
			s.logger.Info("same user, return already added")
			return ErrOrderAlreadyAddedByUser // 200
		}
		s.logger.Info("same user, return already added")
		return ErrOrderAddedAnotherUser // 409
	}
	if !errors.Is(err, sql.ErrNoRows) {
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

func (s *OrderStorage) GetPendingOrders(ctx context.Context) ([]model.Order, error) {
	s.logger.Info("fetching pending orders for accrual processing")

	query := strings.TrimSpace(`
		SELECT user_id, number, status, accrual, uploaded_at
		FROM orders
		WHERE status IN ('NEW', 'PROCESSING')
		ORDER BY uploaded_at ASC
		LIMIT 50
	`)

	rows, err := s.db.QueryContext(ctx, query)

	if err != nil {
		s.logger.Error("get pending orders failed", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order // слайс всех заказов
	for rows.Next() {        // цикл по всем строкам из SELECT
		var o model.Order // одна модель заказов, новая на каждой итерации
		if err := rows.Scan(&o.UserID, &o.Number, &o.Status, &o.Accrual,
			&o.UploadedAt); err != nil { // заполняем модель о, данными из текущей строки
			s.logger.Error("get pending orders failed", zap.Error(err))
			return nil, err
		}

		s.logger.Infow("DEBUG: pending order loaded",
			"number", o.Number,
			"user_id", o.UserID,
			"status", o.Status,
			"accrual", o.Accrual)

		orders = append(orders, o) // добавляем о в общий слайс orders
		s.logger.Infow("pending order", "number", o.Number, "user_id", o.UserID)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("rows iteration error", zap.Error(err))
		return nil, err
	}

	s.logger.Infow("fetched pending orders", "count", len(orders))
	return orders, nil
}

func (s *OrderStorage) UpdateOrderStatus(ctx context.Context, number, status string, accrual float64) error {
	s.logger.Infow("updating order status", "number", number, "status", status,
		"accrual", accrual)

	result, err := s.db.ExecContext(ctx, `
	UPDATE orders
	SET status = $1, accrual = $2
	WHERE number = $3`, status, accrual, number)
	if err != nil {
		s.logger.Error("update order status failed", zap.Error(err), "number", number)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		s.logger.Warn("no order found for update", "number", number)
		return fmt.Errorf("order %s not found", number)
	}

	return nil
}
