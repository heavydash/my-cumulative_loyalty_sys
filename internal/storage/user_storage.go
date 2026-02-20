package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"golang.org/x/crypto/bcrypt"
	"sync"
)

type UserStorage struct {
	repo *GenericRepository[model.User] // дженерик для Create/GetById
	//todo и так передается структура в ресивере по указателю. Мьютекс лучше без указателя.
	mu sync.RWMutex
	db *sql.DB
}

func NewUserStorage(db *sql.DB) *UserStorage {
	scanCreate := func(row *sql.Row, user *model.User) error {
		return row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)

	}
	return &UserStorage{
		db: db,
		mu: sync.RWMutex{},
		repo: NewGenericRepository[model.User](
			db,
			"SELECT id, login, password_hash, created_at FROM users WHERE id = $1",                                    // GetByID
			"INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id, login, password_hash, created_at", // Create
			scanCreate,
			"UPDATE users SET login = $1, password_hash = $2 WHERE id = $3",
			"", // Update
		),
	}
}

func (s *UserStorage) Create(ctx context.Context, login, password string) (*model.User, error) {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	// Вызов дженерика
	return s.repo.Create(ctx, login, hash)
}

func (s *UserStorage) GetByLogin(ctx context.Context, login string) (*model.User, error) {
	user := &model.User{}
	err := s.db.QueryRowContext(ctx, "SELECT id, login, password_hash, created_at FROM users WHERE login = $1", login).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	//todo errors.Is, errors.As применять
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserStorage) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User
	err := s.repo.GetByID(ctx, id, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
