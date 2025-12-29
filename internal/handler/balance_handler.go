package handler

import (
	"encoding/json"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"go.uber.org/zap"
	"net/http"
)

type BalanceHandler struct {
	logger *zap.SugaredLogger
}

type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

func NewBalanceHandler(logger *zap.SugaredLogger) *BalanceHandler {
	return &BalanceHandler{
		logger: logger,
	}
}

func (b *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	_ = userID

	resp := BalanceResponse{
		Current:   0.0,
		Withdrawn: 0.0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	data, err := json.Marshal(resp)
	if err != nil {
		b.logger.Error("Marshal failed", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
