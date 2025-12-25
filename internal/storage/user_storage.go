package storage

import (
	"context"
	"database/sql"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"golang.org/x/crypto/bcrypt"
	"sync"
)

type UserStorage struct {
	mu sync.RWMutex
	db *sql.DB
}

func NewUserStorage(db *sql.DB) *UserStorage {
	return &UserStorage{db: db}
}

func (s *UserStorage) Create(ctx context.Context, login, password string) (*model.User, error) {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var user model.User
	err = s.db.QueryRowContext(ctx, "INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id, login, password_hash, created_at", login, string(hash)).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStorage) GetByLogin(ctx context.Context, login string) (*model.User, error) {
	user := &model.User{}
	err := s.db.QueryRowContext(ctx, "SELECT id, login, password_hash, created_at FROM users WHERE login = $1", login).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}
