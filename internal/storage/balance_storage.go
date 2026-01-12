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
	// Читаем Current иWithdrawn из строки пользователя
	err = b.db.QueryRowContext(ctx, `
		SELECT current_balance, withdrawn
		FROM users
		WHERE id = $1`, userID).Scan(&current, &withdrawn)

	if err == sql.ErrNoRows {
		return 0, 0, ErrUserNotFound
	}

	// Любая другая ошибка БД
	if err != nil {
		b.logger.Errorw("fail to get balance", "user_id", userID, "error", err)
	}

	b.logger.Infow("balance retrieved from users table",
		"user_id", userID,
		"current", current,
		"withdrawn", withdrawn)

	return current, withdrawn, nil
}

func (b *BalanceStorage) AddAccrualToUserBalance(ctx context.Context, user_id int64, amount float64) error {
	if amount <= 0 {
		return nil
	}

	_, err := b.db.ExecContext(ctx, `
		UPDATE users 
		SET current_balance = current_balance + $1
		WHERE id = $2
`, amount, user_id)
	if err != nil {
		return fmt.Errorf("fail to add accrual to user balance: %w", err)
	}
	b.logger.Infow("accrual added to current_balance",
		"user_id", user_id, "amount", amount)

	return nil
}

func (b *BalanceStorage) Withdraw(ctx context.Context, userID int64, orderNumber string, sum float64) error {
	if sum <= 0 {
		return ErrInvalidSum
	}

	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	// Блокируем строку пользователя и читаем current
	var currentBalance float64
	err = tx.QueryRowContext(ctx, `
		SELECT current_balance FROM users WHERE id = $1 FOR UPDATE
		`, userID).Scan(&currentBalance)
	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to select current_balance for update: %w", err)
	}

	// Проверка достаточности средств, если мало 402
	if currentBalance < sum {
		return ErrInsufficientFunds // 402
	}

	// Проверка уникальности order_number для списания
	var exists bool
	err = tx.QueryRowContext(ctx, `
	SELECT EXISTS(SELECT 1 FROM withdrawals WHERE order_number = $1`, orderNumber).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check order uniqueness: %w", err)
	}
	if exists {
		return ErrOrderAlreadyUsed // 422
	}

	// Списание (уменьшаем доступный баланс,увеличиваем архив потраченного)
	_, err = tx.ExecContext(ctx, `
		UPDATE users 
		SET current_balance = current_balance - $1,
		    withdrawn = withdrawn + $1
		    WHERE id = $2`, sum, userID)
	if err != nil {
		return fmt.Errorf("failed to balance in users: %w", err)
	}
	// Записываем операцию в журнал
	_, err = tx.ExecContext(ctx, `
		INSERT INTO withdrawals (user_id, order_number, sum)
		VALUES ($1, $2, $3)`, userID, orderNumber, sum)
	if err != nil {
		return fmt.Errorf("failed to insert into withdrawal: %w", err)
	}

	// Коммит
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	// Лог

	b.logger.Infow("withdrawal successful",
		"user_id", userID,
		"order_number", orderNumber,
		"sum", sum,
		"new_current_balance", currentBalance-sum,
	)
	return nil
}

// Список списаний
func (b *BalanceStorage) GetWithdrawals(ctx context.Context, userID int64) ([]model.WithdrawalDTO, error) {
	rows, err := b.db.QueryContext(ctx, `
	SELECT order_number, sum, processed_at
	FROM withdrawals
	WHERE user_id = $1
	ORDER BY processed_at ASC 
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query withdrawals failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []model.WithdrawalDTO
	for rows.Next() {
		var w model.WithdrawalDTO
		if err := rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, fmt.Errorf("failed to scan withdrawals %w", err)
		}
		withdrawals = append(withdrawals, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows %w", err)
	}
	return withdrawals, nil
}
