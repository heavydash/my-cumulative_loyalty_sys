package handler

import (
	"encoding/json"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io"
	"net/http"
	"time"
)

type OrderHandler struct {
	order_storage *storage.OrderStorage
	logger        *zap.SugaredLogger
}

func NewOrderHandler(order_storage *storage.OrderStorage, logger *zap.SugaredLogger) *OrderHandler {
	return &OrderHandler{
		order_storage: order_storage,
		logger:        logger,
	}
}

func (h *OrderHandler) AddOrder(w http.ResponseWriter, r *http.Request) {
	// Проверка аутентификации
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.logger.Error("get user id from context")
		http.Error(w, "get user id from context", http.StatusUnauthorized)
		return
	}
	// Читаем тело
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Bad request body", zap.Error(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Конвертация
	number := string(body)

	// Проверка и добавление заказа для UserId,
	//Если нет, проверка типа ошибки
	err = h.order_storage.AddOrder(r.Context(), userID, number)
	if err != nil {
		if errors.Is(err, errors.New("invalid order number")) {
			h.logger.Error("Unprocessable Entity", zap.Error(err))
			http.Error(w, "Unprocessable Entity", http.StatusUnprocessableEntity)
			return
		}
		if errors.Is(err, errors.New("order already added by user")) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, errors.New("order added by another user")) {
			h.logger.Error("Conflict", zap.Error(err))
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}
		h.logger.Error("Internal server error", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	// Проверка аутентификации
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.logger.Error("get user id from context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Проверка заказов по UserID
	orders, err := h.order_storage.GetOrders(r.Context(), userID)
	if err != nil {
		h.logger.Error("GetOrders failed", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Проверка на количество заказов
	if len(orders) == 0 {
		h.logger.Info("No Content", zap.Error(err))
		http.Error(w, "No Content", http.StatusNoContent)
		return
	}

	// Конверт в DTO
	var dtos []model.OrderDTO
	for _, o := range orders {
		dtos = append(dtos, model.OrderDTO{
			Number:     o.Number,
			Status:     string(o.Status),
			Accrual:    o.Accrual,
			UploadedAT: o.UploadedAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	data, err := json.Marshal(dtos)
	if err != nil {
		h.logger.Error("Marshal response failed", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
