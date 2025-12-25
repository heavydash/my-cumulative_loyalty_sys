package storage

import (
	"context"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"golang.org/x/crypto/bcrypt"
	"sync"
	"time"
)

type UserStorage struct {
	mu     sync.RWMutex
	users  map[string]*model.User
	nextID int64
}

func NewUserStorage() *UserStorage {
	return &UserStorage{
		users:  make(map[string]*model.User),
		nextID: 1,
	}
}

func (s *UserStorage) Create(ctx context.Context, login, password string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[login]; exists {
		return nil, bcrypt.ErrMismatchedHashAndPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           s.nextID,
		Login:        login,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
	s.users[login] = user
	s.nextID++
	return user, nil
}

func (s *UserStorage) GetByLogin(ctx context.Context, login string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[login]
	if !ok {
		return nil,
			bcrypt.ErrMismatchedHashAndPassword
	}
	return user, nil
}
