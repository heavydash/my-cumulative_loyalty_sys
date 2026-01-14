package handler

import (
	"context"
	"encoding/json"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/accrual"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"time"
)

type OrderHandler struct {
	orderStorage storage.OrderRepository
	logger       *zap.SugaredLogger
	client       *accrual.Client
}

func NewOrderHandler(orderStorage storage.OrderRepository, logger *zap.SugaredLogger, client *accrual.Client) *OrderHandler {
	if logger == nil {
		panic("nil logger")
	}
	if orderStorage == nil {
		panic("nil order_storage")
	}
	return &OrderHandler{
		orderStorage: orderStorage,
		logger:       logger,
		client:       client,
	}
}

func (h *OrderHandler) AddOrder(w http.ResponseWriter, r *http.Request) {
	// Берем пользователя из контекста (middleware аутентификации)
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.logger.Error("get user id from context")
		http.Error(w, "get user id from context", http.StatusUnauthorized)
		return
	}
	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Bad request body", zap.Error(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Конвертация и уборка пробелов
	number := strings.TrimSpace(string(body))
	if number == "" {
		h.logger.Error("order is empty", zap.Error(err))
		http.Error(w, "Order is empty", http.StatusBadRequest)
		return

	}
	// Проверка и добавление заказа для UserId,
	//Если нет, проверка типа ошибки
	err = h.orderStorage.AddOrder(r.Context(), userID, number)

	if err != nil {
		// Luhn
		if errors.Is(err, storage.ErrInvalidOrderNumber) {
			h.logger.Error("invalid order number(Luhn failed)", zap.Error(err),
				zap.String("number", number))
			http.Error(w, "Unprocessable Entity", http.StatusUnprocessableEntity) // 422
			return
		}
		if errors.Is(err, storage.ErrOrderAlreadyAddedByUser) {
			// Идемпотентность, заказ уже загружен этим же юзером
			w.WriteHeader(http.StatusOK) // 200
			return
		}
		if errors.Is(err, storage.ErrOrderAddedAnotherUser) {
			h.logger.Error("order added by another user", zap.Error(err),
				zap.String("number", number))
			http.Error(w, "Conflict", http.StatusConflict) // 409
			return
		}
		// Неизвестная ошибка
		h.logger.Error("failed to add order", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError) // 500
		return
	}

	// Асинхронная регистрация accural в фоне
	go func(orderNum string, uid int64) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Вызов метода клиента
		if err := h.client.RegisterOrder(ctx, orderNum); err != nil {
			h.logger.Warn("Failed to register order in accrual",
				zap.Error(err),
				zap.String("number", orderNum),
				zap.Int64("user_id", uid))
		} else {
			h.logger.Info("order registered in accrual",
				zap.String("number", orderNum),
				zap.Int64("user_id", uid))
		}
	}(number, userID)

	w.WriteHeader(http.StatusAccepted) //202
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
	orders, err := h.orderStorage.GetOrders(r.Context(), userID)
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
