package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) Create(ctx context.Context, login, password string) (*model.User, error) {
	args := m.Called(ctx, login, password)
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *mockUserRepo) GetByLogin(ctx context.Context, login string) (*model.User, error) {
	args := m.Called(ctx, login)
	return args.Get(0).(*model.User), args.Error(1)
}

func TestUserHandler_Register(t *testing.T) {
	logger := zap.NewNop().Sugar()

	signingKey := []byte("test-key")

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := new(mockUserRepo)

		user := &model.User{ID: int64(42), Login: "test"}

		mockRepo.On("Create", mock.Anything, "test", "pass").Return(user, nil)

		handler := NewUserHandler(mockRepo, signingKey, logger)

		body := creds{Login: "test", Password: "pass"}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(jsonBody))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.Register(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		cookies := resp.Cookies()
		assert.Len(t, cookies, 1)
		assert.Equal(t, "jwt", cookies[0].Name)
		assert.NotEmpty(t, cookies[0].Value) // токен сгенерирован

		mockRepo.AssertExpectations(t) // все вызовы мока выполнены
	})

	t.Run("empty login or password", func(t *testing.T) {
		mockRepo := new(mockUserRepo) // репозиторий не вызывается

		handler := NewUserHandler(mockRepo, signingKey, logger)

		body := creds{Login: "", Password: "pass"}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(jsonBody))
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("conflict - user exists", func(t *testing.T) {
		mockRepo := new(mockUserRepo)

		mockRepo.On("Create", mock.Anything, "test", "pass").Return((*model.User)(nil), errors.New("conflict"))

		handler := NewUserHandler(mockRepo, signingKey, logger)

		body := creds{Login: "test", Password: "pass"}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(jsonBody))
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("invalid json", func(t *testing.T) {
		mockRepo := new(mockUserRepo)

		handler := NewUserHandler(mockRepo, signingKey, logger)

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader([]byte("invalid")))
		w := httptest.NewRecorder()

		handler.Register(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockRepo.AssertNotCalled(t, "Create")
	})
}

func TestUserHandler_Login(t *testing.T) {
	logger := zap.NewNop().Sugar()

	signingKey := []byte("test-key")

	ctx := context.Background()

	// Валидный хэш для пароля "pass"
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)

	t.Run("success", func(t *testing.T) {
		mockRepo := new(mockUserRepo)

		user := &model.User{ID: int64(42), PasswordHash: string(hash)}

		mockRepo.On("GetByLogin", mock.Anything, "test").Return(user, nil)

		handler := NewUserHandler(mockRepo, signingKey, logger)

		body := creds{Login: "test", Password: "pass"}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(jsonBody))
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.Login(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		cookies := resp.Cookies()

		assert.Len(t, cookies, 1)
		assert.Equal(t, "jwt", cookies[0].Name)

		mockRepo.AssertExpectations(t)
	})

	t.Run("wrong credentials", func(t *testing.T) {
		mockRepo := new(mockUserRepo)

		user := &model.User{PasswordHash: string(hash)}

		mockRepo.On("GetByLogin", mock.Anything, "test").Return(user, nil) // пароль не совпадёт

		handler := NewUserHandler(mockRepo, signingKey, logger)

		body := creds{Login: "test", Password: "wrong"}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(jsonBody))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		mockRepo := new(mockUserRepo)

		mockRepo.On("GetByLogin", mock.Anything, "unknown").Return((*model.User)(nil), sql.ErrNoRows)

		handler := NewUserHandler(mockRepo, signingKey, logger)

		body := creds{Login: "unknown", Password: "pass"}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(jsonBody))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockRepo.AssertExpectations(t)
	})
}
