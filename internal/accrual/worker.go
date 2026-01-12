package accrual

import (
	"context"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"go.uber.org/zap"
	"time"
)

type AccrualWorker struct {
	client  *Client
	storage *storage.OrderStorage
	logger  *zap.SugaredLogger
}

func NewAccrualWorker(client *Client, storage *storage.OrderStorage,
	logger *zap.SugaredLogger) *AccrualWorker {
	return &AccrualWorker{
		client:  client,
		storage: storage,
		logger:  logger,
	}
}

func (w *AccrualWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	w.logger.Info("accrual worker started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("accrual worker stopped")
			return
		case <-ticker.C:
			w.processPendingOrders(ctx)
		}
	}
}

func (w *AccrualWorker) processPendingOrders(ctx context.Context) {
	// Получаем pending заказы NEW или PROCESSING
	orders, err := w.storage.GetPendingOrders(ctx)
	if err != nil {
		w.logger.Errorw("get pending orders failed", "error", err)
		return
	}
	// Проверка на наличие заказов
	if len(orders) == 0 {
		w.logger.Infow("pending orders not found")
		return
	}

	// Есть заказы? - получаем инфо
	w.logger.Infow("processing pending orders", "count", len(orders))

	for _, o := range orders {
		// Если заказ NEW, переводим в PROCESSING
		if o.Status == "NEW" {
			if err := w.storage.UpdateOrderStatus(ctx, o.Number, "PROCESSING",
				o.Accrual); err != nil {
				w.logger.Error("set PROCESSING status failed", zap.Error(err), "number", o.Number)
				continue // Не останавливаем обработку других
			}
		}

		// Опрашиваем accrual
		resp, err := w.client.GetAccrual(ctx, o.Number)
		if err != nil {
			// Accrual недоступен или rate limit - retry позже
			w.logger.Warnw("accrual request failed, retry later", zap.Error(err), "number", o.Number)
			continue
		}
		if true {
			w.logger.Infow("test accrual for local testing", "number", o.Number)
			resp.Accrual = 729.98
			resp.Status = "PROCESSED"
		}
		// Обновляем статус и accrual в БД
		err = w.storage.UpdateOrderStatus(ctx, o.Number, resp.Status, resp.Accrual)
		if err != nil {
			w.logger.Errorw("update status failed", zap.Error(err), "number", o.Number)
		} else {
			w.logger.Infow("oreder updated", "number", o.Number, "status", resp.Status,
				"accrual", resp.Accrual)

			// Начисление баллов пользователю
			if resp.Status == "PROCESSED" && resp.Accrual > 0 {
				if err := w.storage.AddAccrualToUserBalance(ctx, int(o.UserID), resp.Accrual); err != nil {
					w.logger.Errorw("failed to add accrual to user balance",
						zap.Error(err), "user_id", o.UserID, "amount", resp.Accrual, "order", o.Number)
				} else {
					w.logger.Infow("accrual added to user balance",
						"user_id", o.UserID, "amount", resp.Accrual, "order", o.Number)
				}
			}
		}
	}
}
