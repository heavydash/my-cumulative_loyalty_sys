package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockBalanceRepo struct {
	mock.Mock
}

func (m *mockBalanceRepo) GetBalance(ctx context.Context, userID int64) (float64, float64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(float64), args.Get(1).(float64), args.Error(2)
}

func (m *mockBalanceRepo) Withdraw(ctx context.Context, userID int64, orderNumber string, sum float64) error {
	args := m.Called(ctx, userID, orderNumber, sum)
	return args.Error(0)
}

func (m *mockBalanceRepo) GetWithdrawals(ctx context.Context, userID int64) ([]model.WithdrawalDTO, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.WithdrawalDTO), args.Error(1)
}

func TestBalanceHandler_GetBalance(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	t.Run("unauthorized", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
		w := httptest.NewRecorder()

		handler.GetBalance(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockRepo.AssertNotCalled(t, "GetBalance")
	})

	t.Run("success", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		mockRepo.On("GetBalance", mock.Anything, int64(42)).Return(1000.555, 200.333, nil)

		w := httptest.NewRecorder()

		handler.GetBalance(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp struct {
			Current   float64 `json:"current"`
			Withdrawn float64 `json:"withdrawn"`
		}
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Equal(t, math.Round(1000.555*100)/100, resp.Current) // округление до 2 знаков
		assert.Equal(t, math.Round(200.333*100)/100, resp.Withdrawn)

		mockRepo.AssertExpectations(t)
	})

	t.Run("internal error", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		mockRepo.On("GetBalance", mock.Anything, int64(42)).Return(0.0, 0.0, errors.New("db error"))

		w := httptest.NewRecorder()

		handler.GetBalance(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestBalanceHandler_Withdraw(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()

	t.Run("unauthorized", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		body := model.WithdrawRequest{Order: "12345678903", Sum: 100.0}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/withdraw", bytes.NewReader(jsonBody))
		w := httptest.NewRecorder()

		handler.Withdraw(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockRepo.AssertNotCalled(t, "Withdraw")
	})

	t.Run("invalid json", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		req := httptest.NewRequest(http.MethodPost, "/api/user/withdraw", bytes.NewReader([]byte("invalid")))
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		w := httptest.NewRecorder()

		handler.Withdraw(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockRepo.AssertNotCalled(t, "Withdraw")
	})

	t.Run("sum <=0", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		body := model.WithdrawRequest{Order: "12345678903", Sum: 0}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/withdraw", bytes.NewReader(jsonBody))
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		w := httptest.NewRecorder()

		handler.Withdraw(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		mockRepo.AssertNotCalled(t, "Withdraw")
	})

	t.Run("invalid luhn", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		body := model.WithdrawRequest{Order: "123", Sum: 100.0}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/withdraw", bytes.NewReader(jsonBody))
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		w := httptest.NewRecorder()

		handler.Withdraw(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		mockRepo.AssertNotCalled(t, "Withdraw")
	})

	t.Run("insufficient funds", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		body := model.WithdrawRequest{Order: "12345678903", Sum: 1000.0}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/withdraw", bytes.NewReader(jsonBody))
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		mockRepo.On("Withdraw", mock.Anything, int64(42), "12345678903", 1000.0).Return(storage.ErrInsufficientFunds)

		w := httptest.NewRecorder()

		handler.Withdraw(w, req)

		assert.Equal(t, http.StatusPaymentRequired, w.Code) // 402

		mockRepo.AssertExpectations(t)
	})

	t.Run("order already used", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		body := model.WithdrawRequest{Order: "12345678903", Sum: 100.0}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/withdraw", bytes.NewReader(jsonBody))
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		mockRepo.On("Withdraw", mock.Anything, int64(42), "12345678903", 100.0).Return(storage.ErrOrderAlreadyWithdrawn) // или твоя ошибка

		w := httptest.NewRecorder()

		handler.Withdraw(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code) // 422

		mockRepo.AssertExpectations(t)
	})

	t.Run("success", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		body := model.WithdrawRequest{Order: "12345678903", Sum: 100.0}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/user/withdraw", bytes.NewReader(jsonBody))
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		mockRepo.On("Withdraw", mock.Anything, int64(42), "12345678903", 100.0).Return(nil)

		w := httptest.NewRecorder()

		handler.Withdraw(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		mockRepo.AssertExpectations(t)
	})
}

func TestBalanceHandler_GetWithdrawals(t *testing.T) {
	logger := zap.NewNop().Sugar()

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("unauthorized", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
		w := httptest.NewRecorder()

		handler.GetWithdrawals(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		mockRepo.AssertNotCalled(t, "GetWithdrawals")
	})

	t.Run("no content - empty", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		mockRepo.On("GetWithdrawals", mock.Anything, int64(42)).Return([]model.WithdrawalDTO{}, nil)

		w := httptest.NewRecorder()

		handler.GetWithdrawals(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)

		mockRepo.AssertExpectations(t)
	})

	t.Run("success - with withdrawals", func(t *testing.T) {
		mockRepo := new(mockBalanceRepo)

		handler := NewBalanceHandler(mockRepo, logger)

		req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
		req = req.WithContext(auth.ContextWithUserID(ctx, int64(42)))

		rawWithdrawals := []model.WithdrawalDTO{
			{Order: "123", Sum: 100.555, ProcessedAt: fixedTime},
		}

		mockRepo.On("GetWithdrawals", mock.Anything, int64(42)).Return(rawWithdrawals, nil)

		w := httptest.NewRecorder()

		handler.GetWithdrawals(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp []model.WithdrawalDTO
		err := json.NewDecoder(w.Body).Decode(&resp)
		require.NoError(t, err)

		assert.Len(t, resp, 1)
		assert.Equal(t, "123", resp[0].Order)
		assert.Equal(t, math.Round(100.555*100)/100, resp[0].Sum) // округление до 2 знаков

		assert.Equal(t, fixedTime, resp[0].ProcessedAt)

		mockRepo.AssertExpectations(t)
	})
}
