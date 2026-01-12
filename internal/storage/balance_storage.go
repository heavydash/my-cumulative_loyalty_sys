package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"go.uber.org/zap"
)

type BalanceStorage struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

func NewBalanceStorage(db *sql.DB, logger *zap.SugaredLogger) *BalanceStorage {
	return &BalanceStorage{
		db:     db,
		logger: logger,
	}
}

func (b *BalanceStorage) GetBalance(ctx context.Context, userID int64) (current, withdrawn float64, err error) {
	// Начислено
	var accrued float64
	err = b.db.QueryRowContext(ctx,
		`
        SELECT COALESCE(SUM(accrual), 0)
        FROM orders
        WHERE user_id = $1 AND status = $2
    `, userID, model.OrderStatusProcessed,
	).Scan(&accrued)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to calculate accrued %w", err)
	}

	// Списано
	var withdrawnFloat float64
	err = b.db.QueryRowContext(ctx,
		`
        SELECT COALESCE(SUM(sum), 0)
        FROM withdrawals
        WHERE user_id = $1
    `,
		userID,
	).Scan(&withdrawnFloat)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to calculate withdrawn %w", err)
	}

	current = accrued - withdrawnFloat
	if current < 0 {
		current = 0
	}
	// лог для поиска проблемы начисления баллов
	b.logger.Info("balance calculated",
		zap.Int64("user_id", userID),
		zap.Float64("accrued", accrued),
		zap.Float64("withdrawn", withdrawnFloat),
		zap.Float64("current", current))

	return current, withdrawnFloat, nil

}

func (b *BalanceStorage) Withdraw(ctx context.Context, userID int64, orderNumber string, sum float64) error {
	if sum <= 0 {
		return ErrInvalidSum
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check balance
	current, _, err := b.GetBalance(ctx, userID)
	var accrued, withdrawn float64
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(accrual), 0), coalesce((SELECT SUM(sum) FROM withdrawals
		WHERE user_id = $1), 0)
		FROM orders
		WHERE user_id = $1 AND status = $2
`, userID, model.OrderStatusProcessed).Scan(&accrued, &withdrawn)
	if err != nil {
		return err
	}
	current = accrued - withdrawn
	if current < sum {
		return ErrInsufficientFunds
	}

	// Уникальные order_number
	var exists bool
	err = tx.QueryRowContext(ctx, `
	SELECT EXISTS(SELECT 1 FROM withdrawals WHERE order_number = $1`, orderNumber).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrOrderAlreadyWithdrawn
	}

	// Списание
	_, err = tx.ExecContext(ctx, `
		INSERT INTO withdrawals (user_id, order_number, sum)
		VALUES ($1, $2, $3)`, userID, orderNumber, sum)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Список списаний
func (b *BalanceStorage) GetWithdrawals(ctx context.Context, userID int64) ([]model.WithdrawalDTO, error) {
	rows, err := b.db.QueryContext(ctx, `
	SELECT order_number, sum, processed_at
	FROM withdrawals
	WHERE user_id = $1
	ORDER BY processed_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query withdrawals failed: %w", err)
	}
	defer rows.Close()

	var list []model.WithdrawalDTO
	for rows.Next() {
		var w model.WithdrawalDTO
		if err := rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, fmt.Errorf("failed to scan withdrawals %w", err)
		}
		list = append(list, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows %w", err)
	}
	return list, nil
}
