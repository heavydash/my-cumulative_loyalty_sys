package handler

import (
	"encoding/json"
	"errors"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/util"
	"go.uber.org/zap"
	"math"
	"net/http"
)

type BalanceHandler struct {
	storage storage.BalanceRepository
	logger  *zap.SugaredLogger
}

func NewBalanceHandler(storage storage.BalanceRepository, logger *zap.SugaredLogger) *BalanceHandler {
	return &BalanceHandler{
		storage: storage,
		logger:  logger,
	}
}

func (b *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	// Извлекаем userID из контекста
	userID, ok := auth.UserIDFromContext(r.Context())

	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Получение сырых значений из storage
	currentBalance, withdrawn, err := b.storage.GetBalance(r.Context(), userID)
	if err != nil {
		b.logger.Error("get balance failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Округляем до 2 знаков после запятой
	currentRounded := math.Round(currentBalance*100) / 100
	withdrawnRounded := math.Round(withdrawn*100) / 100

	// Формируем ответ по спецификации
	response := struct {
		Current   float64 `json:"current"`   // доступные баллы
		Withdrawn float64 `json:"withdrawn"` // потраченные баллы
	}{
		Current:   currentRounded,
		Withdrawn: withdrawnRounded,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		b.logger.Error("failed to encode balance response", "err", err)
	}

	b.logger.Infow("balance response sent",
		"userId", userID, "current", currentRounded, "withdrawn", withdrawnRounded)
}

func (b *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		b.logger.Error("decode withdraw request body failed", zap.Error(err))
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.Sum <= 0 {
		b.logger.Error("sum must be positive: %d", req.Sum)
		http.Error(w, "invalid sum", http.StatusBadRequest)
		return
	}

	if !util.IsLuhnValid(req.Order) {
		b.logger.Error("failed to validate order", zap.String("order", req.Order))
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}

	err := b.storage.Withdraw(r.Context(), userID, req.Order, req.Sum)
	if errors.Is(err, storage.ErrInsufficientFunds) {
		b.logger.Error("not enough funds", zap.String("order", req.Order))
		http.Error(w, "not enough funds", http.StatusPaymentRequired) //402
		return
	}
	if errors.Is(err, storage.ErrOrderAlreadyWithdrawn) {
		b.logger.Error("failed to withdraw", zap.String("order", req.Order), zap.Error(err))
		http.Error(w, "order already used", http.StatusUnprocessableEntity) //422
		return
	}
	if err != nil {
		b.logger.Error("withdraw failed", zap.String("order", req.Order), zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (b *BalanceHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	// Извлекаем userID из контекста
	userID, ok := auth.UserIDFromContext(r.Context())

	if !ok || userID == 0 {
		b.logger.Info("Unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized) //401
		return
	}

	// Получаем список из storage (DTO float64, sum, time.Time, processed_at)
	withdrawals, err := b.storage.GetWithdrawals(r.Context(), userID)
	if err != nil {
		b.logger.Error("get withdrawals failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for i := range withdrawals {
		withdrawals[i].Sum = math.Round(withdrawals[i].Sum*100) / 100
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(withdrawals); err != nil {
		b.logger.Errorw("failed to encode withdrawals response", "err", err)
	}

	b.logger.Infow("withdrawals response sent", "userId", userID, "count", len(withdrawals))
}
