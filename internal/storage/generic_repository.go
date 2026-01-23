package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// Сканирование Row в entity
type ScanFunc[T any] func(*sql.Row, *T) error

// Репозиторий для сущностей с ID
type GenericRepository[T any] struct {
	db               *sql.DB
	getByIDStmt      string
	createQuery      string
	scanCreate       ScanFunc[T] //callback для Scan после Returning
	updateStmt       string
	listByUserIDStmt string
}

// Конструктор с хардкорными запросами
func NewGenericRepository[T any](
	db *sql.DB,
	getByIDStmt string,
	createQuery string,
	scanCreate ScanFunc[T],
	updateStmt string,
	listByUserIDStmt string, // для заказов и списаний
) *GenericRepository[T] {
	return &GenericRepository[T]{
		db:               db,
		getByIDStmt:      getByIDStmt,
		createQuery:      createQuery,
		scanCreate:       scanCreate,
		updateStmt:       updateStmt,
		listByUserIDStmt: listByUserIDStmt,
	}

}

// GetByID - общий метод (Scan в отдельном хранилище)
func (r *GenericRepository[T]) GetByID(ctx context.Context, id int64, dest *T) error {
	return r.db.QueryRowContext(ctx, r.getByIDStmt, id).Scan(dest)
}

// Create - общий метод
func (r *GenericRepository[T]) Create(ctx context.Context, args ...interface{}) (*T, error) {
	var entity T

	// QueryRow выполняет Insert Returning с args с любым количеством полей
	row := r.db.QueryRowContext(ctx, r.createQuery, args...)

	// Деллегирование Scan в callback
	if err := r.scanCreate(row, &entity); err != nil {
		return nil, err
	}
	return &entity, nil
}

// Update - общий метод
func (r *GenericRepository[T]) Update(ctx context.Context, entity *T) error {
	_, err := r.db.ExecContext(ctx, r.updateStmt, entity)
	return err
}

// ListByUserID - общий метод для заказов и списаний
func (r *GenericRepository[T]) ListByUserID(ctx context.Context, userID int64) (*sql.Rows, error) {
	if r.listByUserIDStmt == "" {
		return nil, fmt.Errorf("listByUserIDStmt not supported for this entity")
	}
	return r.db.QueryContext(ctx, r.listByUserIDStmt, userID)
}
