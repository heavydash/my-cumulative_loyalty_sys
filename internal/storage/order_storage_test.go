package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestOrderStorage_AddOrder(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	t.Run("empty number", func(t *testing.T) {
		db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		err = storage.AddOrder(ctx, 1, "")

		assert.ErrorIs(t, err, ErrOrderIsEmpty) // проверяем доменную ошибку валидации
	})

	t.Run("invalid luhn", func(t *testing.T) {
		db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		err = storage.AddOrder(ctx, 1, "123")

		assert.ErrorIs(t, err, ErrInvalidOrderNumber) // Luhn не прошёл
	})

	t.Run("already added by same user", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		userID := int64(42)
		number := "12345678903"

		mock.ExpectQuery("SELECT user_id FROM orders WHERE number = $1").
			WithArgs(number).
			WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(userID))

		err = storage.AddOrder(ctx, userID, number)

		assert.ErrorIs(t, err, ErrOrderAlreadyAddedByUser)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("added by another user", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		userID := int64(42)
		otherUserID := int64(100)
		number := "12345678903"

		mock.ExpectQuery("SELECT user_id FROM orders WHERE number = $1").
			WithArgs(number).
			WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(otherUserID))

		err = storage.AddOrder(ctx, userID, number)

		assert.ErrorIs(t, err, ErrOrderAddedAnotherUser)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("success new order", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		userID := int64(42)
		number := "12345678903"

		// Ожидаем SELECT — заказ не найден
		mock.ExpectQuery("SELECT user_id FROM orders WHERE number = $1").
			WithArgs(number). // только 1 аргумент
			WillReturnError(sql.ErrNoRows)

		// Ожидаем INSERT — точный SQL из кода
		expectedInsertSQL := "INSERT INTO orders (number, user_id, status, uploaded_at) VALUES ($1, $2, 'NEW', NOW())"

		// WithArgs для реальных параметров (number, userID)
		mock.ExpectExec(expectedInsertSQL).
			WithArgs(number, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err = storage.AddOrder(ctx, userID, number)

		assert.NoError(t, err) // 202

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error on select", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		number := "12345678903"

		mock.ExpectQuery("SELECT user_id FROM orders WHERE number = $1").
			WithArgs(number).
			WillReturnError(errors.New("db down"))

		err = storage.AddOrder(ctx, 42, number)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db down")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOrderStorage_GetOrders(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	olderTime := fixedTime.AddDate(0, 0, -1)

	t.Run("success with orders", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		userID := int64(42)

		expectedSQL := "SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC"

		rows := sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}).
			AddRow("12345678903", "PROCESSED", 500.0, fixedTime).
			AddRow("987654321", "NEW", 0.0, olderTime)

		mock.ExpectQuery(expectedSQL).
			WithArgs(userID).
			WillReturnRows(rows)

		orders, err := storage.GetOrders(ctx, userID)

		require.NoError(t, err)
		assert.Len(t, orders, 2)
		assert.Equal(t, "12345678903", orders[0].Number)
		assert.Equal(t, model.OrderStatus("PROCESSED"), orders[0].Status)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty list", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		expectedSQL := "SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC"

		mock.ExpectQuery(expectedSQL).
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"number", "status", "accrual", "uploaded_at"}))

		orders, err := storage.GetOrders(ctx, 42)

		require.NoError(t, err)
		assert.Empty(t, orders)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOrderStorage_GetPendingOrders(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("success with pending orders", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		expectedSQL := "SELECT user_id, number, status, accrual, uploaded_at FROM orders WHERE status IN ('NEW', 'PROCESSING') ORDER BY uploaded_at ASC LIMIT 50"

		rows := sqlmock.NewRows([]string{"user_id", "number", "status", "accrual", "uploaded_at"}).
			AddRow(int64(1), "123", "NEW", 0.0, fixedTime).
			AddRow(int64(2), "456", "PROCESSING", 100.0, fixedTime.Add(1))

		mock.ExpectQuery(expectedSQL).
			WillReturnRows(rows)

		orders, err := storage.GetPendingOrders(ctx)

		require.NoError(t, err)
		assert.Len(t, orders, 2)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOrderStorage_UpdateOrderStatus(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		expectedSQL := "UPDATE orders SET status = $1, accrual = $2 WHERE number = $3"

		mock.ExpectExec(expectedSQL).
			WithArgs("PROCESSED", 500.0, "12345678903").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = storage.UpdateOrderStatus(ctx, "12345678903", "PROCESSED", 500.0)

		assert.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("order not found", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewOrderStorage(db, logger)

		expectedSQL := "UPDATE orders SET status = $1, accrual = $2 WHERE number = $3"

		mock.ExpectExec(expectedSQL).
			WithArgs("INVALID", 0.0, "unknown").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err = storage.UpdateOrderStatus(ctx, "unknown", "INVALID", 0.0)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
