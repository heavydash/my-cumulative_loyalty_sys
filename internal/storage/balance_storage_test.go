package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestBalanceStorage_GetBalance(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		userID := int64(42)

		expectedSQL := "SELECT current_balance, withdrawn FROM users WHERE id = $1"

		mock.ExpectQuery(expectedSQL).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"current_balance", "withdrawn"}).
				AddRow(1000.5, 200.0))

		current, withdrawn, err := storage.GetBalance(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, 1000.5, current)
		assert.Equal(t, 200.0, withdrawn)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user not found", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		mock.ExpectQuery("SELECT current_balance, withdrawn FROM users WHERE id = $1").
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		current, withdrawn, err := storage.GetBalance(ctx, 999)

		assert.Equal(t, 0.0, current)
		assert.Equal(t, 0.0, withdrawn)
		assert.ErrorIs(t, err, ErrUserNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		expectedSQL := "SELECT current_balance, withdrawn FROM users WHERE id = $1"

		mock.ExpectQuery(expectedSQL).
			WithArgs(int64(42)).
			WillReturnError(errors.New("db down"))

		current, withdrawn, err := storage.GetBalance(ctx, 42)

		assert.NoError(t, err)
		assert.Equal(t, 0.0, current)
		assert.Equal(t, 0, 0, withdrawn)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestBalanceStorage_AddAccrualToUserBalance(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	t.Run("amount zero or negative - no op", func(t *testing.T) {
		db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		err = storage.AddAccrualToUserBalance(ctx, 42, -100.0)

		assert.NoError(t, err)
	})

	t.Run("success", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		userID := int64(42)
		amount := 500.0

		expectedSQL := "UPDATE users SET current_balance = current_balance + $1 WHERE id = $2"

		mock.ExpectExec(expectedSQL).
			WithArgs(amount, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = storage.AddAccrualToUserBalance(ctx, userID, amount)

		assert.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		mock.ExpectExec("UPDATE users SET current_balance = current_balance + $1 WHERE id = $2").
			WithArgs(500.0, int64(42)).
			WillReturnError(errors.New("update failed"))

		err = storage.AddAccrualToUserBalance(ctx, 42, 500.0)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fail to add accrual")

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestBalanceStorage_Withdraw(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	t.Run("invalid sum", func(t *testing.T) {
		db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		err = storage.Withdraw(ctx, 42, "123", -100.0)

		assert.ErrorIs(t, err, ErrInvalidSum)
	})

	t.Run("success", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		userID := int64(42)
		orderNumber := "12345678903"
		sum := 300.0

		// Expect Begin (транзакция)
		mock.ExpectBegin()

		// SELECT FOR UPDATE — читаем баланс (достаточно)
		mock.ExpectQuery("SELECT current_balance FROM users WHERE id = $1 FOR UPDATE").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"current_balance"}).AddRow(1000.0))

		// Проверка уникальности order_number — не существует
		mock.ExpectQuery("SELECT EXISTS(SELECT 1 FROM withdrawals WHERE order_number = $1)").
			WithArgs(orderNumber).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		// UPDATE users (списание)
		mock.ExpectExec("UPDATE users SET current_balance = current_balance - $1, withdrawn = withdrawn + $1 WHERE id = $2").
			WithArgs(sum, userID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// INSERT into withdrawals
		mock.ExpectExec("INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3)").
			WithArgs(userID, orderNumber, sum).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Commit
		mock.ExpectCommit()

		err = storage.Withdraw(ctx, userID, orderNumber, sum)

		assert.NoError(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("insufficient funds", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		mock.ExpectBegin()

		mock.ExpectQuery("SELECT current_balance FROM users WHERE id = $1 FOR UPDATE").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"current_balance"}).AddRow(100.0)) // мало средств

		// После недостатка средств — Rollback
		mock.ExpectRollback()

		err = storage.Withdraw(ctx, 42, "123", 300.0)

		assert.ErrorIs(t, err, ErrInsufficientFunds)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("order already used", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		mock.ExpectBegin()

		mock.ExpectQuery("SELECT current_balance FROM users WHERE id = $1 FOR UPDATE").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"current_balance"}).AddRow(1000.0))

		mock.ExpectQuery("SELECT EXISTS(SELECT 1 FROM withdrawals WHERE order_number = $1)").
			WithArgs("used").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		mock.ExpectRollback()

		err = storage.Withdraw(ctx, 42, "used", 100.0)

		assert.ErrorIs(t, err, ErrOrderAlreadyUsed)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user not found", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		mock.ExpectBegin()

		mock.ExpectQuery("SELECT current_balance FROM users WHERE id = $1 FOR UPDATE").
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		mock.ExpectRollback()

		err = storage.Withdraw(ctx, 999, "123", 100.0)

		assert.ErrorIs(t, err, ErrUserNotFound)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestBalanceStorage_GetWithdrawals(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("success with withdrawals", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		userID := int64(42)

		expectedSQL := "SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at ASC"

		rows := sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}).
			AddRow("123", 100.0, fixedTime).
			AddRow("456", 200.0, fixedTime.Add(1))

		mock.ExpectQuery(expectedSQL).
			WithArgs(userID).
			WillReturnRows(rows)

		withdrawals, err := storage.GetWithdrawals(ctx, userID)

		require.NoError(t, err)
		assert.Len(t, withdrawals, 2)
		assert.Equal(t, "123", withdrawals[0].Order)
		assert.Equal(t, 100.0, withdrawals[0].Sum)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty list", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		storage := NewBalanceStorage(db, logger)

		mock.ExpectQuery("SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at ASC").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}))

		withdrawals, err := storage.GetWithdrawals(ctx, 42)

		require.NoError(t, err)
		assert.Empty(t, withdrawals)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
