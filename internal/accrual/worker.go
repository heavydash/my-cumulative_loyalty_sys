package accrual

import (
	"context"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/model"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"strings"
	"time"
)

type AccrualWorker struct {
	client         *Client
	balanceStorage *storage.BalanceStorage
	orderStorage   *storage.OrderStorage
	logger         *zap.SugaredLogger
}

func NewAccrualWorker(client *Client, balanceStorage *storage.BalanceStorage,
	orderStorage *storage.OrderStorage, logger *zap.SugaredLogger) *AccrualWorker {
	return &AccrualWorker{
		client:         client,
		balanceStorage: balanceStorage,
		orderStorage:   orderStorage,
		logger:         logger,
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
	orders, err := w.orderStorage.GetPendingOrders(ctx)
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

	g, ctx := errgroup.WithContext(ctx)

	for _, o := range orders {
		o := o // захват переменной для горутины

		g.Go(func() error {
			// Если заказ NEW, переводим в PROCESSING
			if o.Status == model.OrderStatusNew {
				regErr := w.client.RegisterOrder(ctx, o.Number)
				if regErr != nil {
					if isConflict(regErr) {
						w.logger.Infow("order already registered", "order", o.Number)
					} else {
						w.logger.Warnw("failed to register order", "number", o.Number, zap.Error(regErr))
						return nil
					}
				} else {
					w.logger.Infow("order registered", "order", o.Number)
				}
				// Переход в processing
				if err := w.orderStorage.UpdateOrderStatus(ctx, o.Number,
					string(model.OrderStatusProcessing), o.Accrual); err != nil {
					w.logger.Error("set PROCESSING failed", zap.Error(err), "number", o.Number)
					return err
				}
			}

			// GetAccrual с retry
			var resp *AccrualResponse
			var accrualErr error
			for attempt := 0; attempt < 3; attempt++ {
				// Опрашиваем accrual
				resp, accrualErr = w.client.GetAccrual(ctx, o.Number)
				if accrualErr == nil {
					break // успех — resp заполнен
				}
				w.logger.Warnw("accrual request failed, retry", "error", accrualErr, "number", o.Number,
					"attempt", attempt+1)
				time.Sleep(time.Second * time.Duration(attempt+1))
			}

			if accrualErr != nil {
				w.logger.Errorw("accrual failed after retries", "error", accrualErr, "number", o.Number)
				return nil
			}

			// Защита от дубликатов
			addAmount := resp.Accrual - o.Accrual
			if addAmount < 0 {
				addAmount = 0
			}

			if err := w.balanceStorage.AddAccrualToUserBalance(ctx, o.UserID, addAmount); err != nil {
				w.logger.Errorw("failed to add accrual", zap.Error(err), "user_id", o.UserID, "amount", addAmount)
				return err // критическая
			}

			// Обновляем статус и accrual в БД
			if err := w.orderStorage.UpdateOrderStatus(ctx, o.Number, resp.Status, resp.Accrual); err != nil {
				w.logger.Errorw("update status failed", zap.Error(err), "number", o.Number)
				return err
			}

			w.logger.Infow("order updated", "number", o.Number, "status", resp.Status,
				"accrual", resp.Accrual)

			return nil
		})
	}

	// Ждём завершения всех задач
	if err := g.Wait(); err != nil {
		w.logger.Errorw("critical error in processing", "error", err)
	} else {
		w.logger.Info("all pending orders processed successfully")
	}
}

func isConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status code: 409")
}
