package handler

import (
	"encoding/json"
	"errors"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/util"
	"go.uber.org/zap"
	"log"
	"net/http"
)

type BalanceHandler struct {
	storage *storage.BalanceStorage
	logger  *zap.SugaredLogger
}

func NewBalanceHandler(storage *storage.BalanceStorage, logger *zap.SugaredLogger) *BalanceHandler {
	return &BalanceHandler{
		storage: storage,
		logger:  logger,
	}
}

func (b *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	current, withdrawn, err := b.storage.GetBalance(r.Context(), userID)
	if err != nil {
		b.logger.Error("get balance failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := model.BalanceDTO{
		Current:   current,
		Withdrawn: withdrawn,
	}

	respondJSON(w, http.StatusOK, resp)
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
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		b.logger.Error("Unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	list, err := b.storage.GetWithdrawals(r.Context(), userID)
	if err != nil {
		b.logger.Error("get withdrawals failed", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(list) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	respondJSON(w, http.StatusOK, list)
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal response", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
