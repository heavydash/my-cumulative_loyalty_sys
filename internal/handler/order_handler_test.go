package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/accrual"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockOrderRepo struct {
	mock.Mock
}

func (m *mockOrderRepo) AddOrder(ctx context.Context, userID int64, number string) error {
	args := m.Called(ctx, userID, number)
	return args.Error(0)
}

func (m *mockOrderRepo) GetOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.Order), args.Error(1)
}

func TestOrderHandler_AddOrder(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	client := accrual.NewClient("", logger)

	t.Run("unauthorized - no userID in context", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		// Запрос без userID в контексте
		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte("12345678903")))
		w := httptest.NewRecorder()

		handler.AddOrder(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockRepo.AssertNotCalled(t, "AddOrder")
	})

	t.Run("empty body", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte("")))
		req = req.WithContext(auth.ContextWithUserID(ctx, 42)) // добавляем userID

		w := httptest.NewRecorder()

		handler.AddOrder(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockRepo.AssertNotCalled(t, "AddOrder")
	})

	t.Run("invalid luhn", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte("123")))
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		mockRepo.On("AddOrder", mock.Anything, int64(42), "123").Return(storage.ErrInvalidOrderNumber)

		w := httptest.NewRecorder()

		handler.AddOrder(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("already added by same user", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		number := "12345678903"

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte(number)))
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		mockRepo.On("AddOrder", mock.Anything, int64(42), number).Return(storage.ErrOrderAlreadyAddedByUser)

		w := httptest.NewRecorder()

		handler.AddOrder(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("added by another user", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		number := "12345678903"

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte(number)))
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		mockRepo.On("AddOrder", mock.Anything, int64(42), number).Return(storage.ErrOrderAddedAnotherUser)

		w := httptest.NewRecorder()

		handler.AddOrder(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("success - accepted", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		number := "12345678903"

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte(number)))
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		mockRepo.On("AddOrder", mock.Anything, int64(42), number).Return(nil)

		w := httptest.NewRecorder()

		handler.AddOrder(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code) // 202

		mockRepo.AssertExpectations(t)

	})

	t.Run("internal error", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		number := "12345678903"

		req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewReader([]byte(number)))
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		mockRepo.On("AddOrder", mock.Anything, int64(42), number).Return(errors.New("db error"))

		w := httptest.NewRecorder()

		handler.AddOrder(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestOrderHandler_GetOrders(t *testing.T) {
	logger := zap.NewNop().Sugar()

	client := &accrual.Client{}

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("unauthorized", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		w := httptest.NewRecorder()

		handler.GetOrders(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockRepo.AssertNotCalled(t, "GetOrders")
	})

	t.Run("no content - empty orders", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		mockRepo.On("GetOrders", mock.Anything, int64(42)).Return([]model.Order{}, nil)

		w := httptest.NewRecorder()

		handler.GetOrders(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("success - with orders", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		orders := []model.Order{
			{Number: "123", Status: "PROCESSED", Accrual: 500.0, UploadedAt: fixedTime},
		}

		mockRepo.On("GetOrders", mock.Anything, int64(42)).Return(orders, nil)

		w := httptest.NewRecorder()

		handler.GetOrders(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var dtos []model.OrderDTO
		err := json.NewDecoder(w.Body).Decode(&dtos)
		require.NoError(t, err)
		assert.Len(t, dtos, 1)
		assert.Equal(t, "123", dtos[0].Number)
		assert.Equal(t, fixedTime.Format(time.RFC3339), dtos[0].UploadedAT)

		mockRepo.AssertExpectations(t)
	})

	t.Run("internal error", func(t *testing.T) {
		mockRepo := new(mockOrderRepo)

		handler := NewOrderHandler(mockRepo, logger, client)

		req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
		req = req.WithContext(auth.ContextWithUserID(ctx, 42))

		mockRepo.On("GetOrders", mock.Anything, int64(42)).Return([]model.Order{}, errors.New("db error"))

		w := httptest.NewRecorder()

		handler.GetOrders(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockRepo.AssertExpectations(t)
	})
}
