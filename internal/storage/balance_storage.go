package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"go.uber.org/zap"
)

type BalanceStorage struct {
	repo   *GenericRepository[model.Withdrawal]
	db     *sql.DB
	logger *zap.SugaredLogger
}

func NewBalanceStorage(db *sql.DB, logger *zap.SugaredLogger) *BalanceStorage {
	scanCreate := func(row *sql.Row, dto *model.Withdrawal) error {
		return row.Scan(&dto.Order, &dto.Sum, &dto.ProcessedAt)
	}
	return &BalanceStorage{
		db:     db,
		logger: logger,
		repo: NewGenericRepository[model.Withdrawal](
			db,
			"",
			"INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3) RETURNING order_number, sum, processed_at",
			scanCreate,
			"",
			"SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at ASC",
		),
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

func (b *BalanceStorage) AddAccrualToUserBalance(ctx context.Context, userID int64, amount float64) error {
	if amount <= 0 {
		return nil
	}

	_, err := b.db.ExecContext(ctx, `
		UPDATE users 
		SET current_balance = current_balance + $1
		WHERE id = $2
`, amount, userID)
	if err != nil {
		return fmt.Errorf("fail to add accrual to user balance: %w", err)
	}
	b.logger.Infow("accrual added to current_balance",
		"user_id", userID, "amount", amount)

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
	// todo никогда деньги во float не храним. Всегда int
	// RUB 1.99 | Йпонская йена 1.999
	// на собесе будут спрашивать про ISO, округления, делиметры для копеек
	var currentBalance float64
	err = tx.QueryRowContext(ctx, `
		SELECT current_balance FROM users WHERE id = $1 FOR UPDATE
		`, userID).Scan(&currentBalance)
	if errors.Is(err, sql.ErrNoRows) {
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
	SELECT EXISTS(SELECT 1 FROM withdrawals WHERE order_number = $1)`, orderNumber).Scan(&exists)
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
func (b *BalanceStorage) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	rows, err := b.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("query withdrawals failed: %w", err)
	}
	defer rows.Close()

	var withdrawals []model.Withdrawal
	for rows.Next() {
		var w model.Withdrawal
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
