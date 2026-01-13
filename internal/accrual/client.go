package accrual

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	logger  *zap.SugaredLogger
	client  *http.Client
}

func NewClient(baseURL string, logger *zap.SugaredLogger) *Client {
	// Trim trailing slash без двойного //
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		logger:  logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Структура ответа от accrual системы

type AccrualResponse struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`            // Registered, Invalid, Processing, Processed
	Accrual float64 `json:"accrual,omitempty"` // только, если Processed
}

// GetAccrual - дергает accrual по номеру заказа
func (c *Client) GetAccrual(ctx context.Context, orderNumber string) (*AccrualResponse, error) {
	// Поиск по baseURL, orderNumber
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.logger.Errorw("error request failed", zap.Error(err), zap.String("order", orderNumber))
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Errorw("accrual request failed", zap.Error(err), zap.String("order", orderNumber))
		return nil, err
	}
	defer resp.Body.Close()

	// Обрабатываем статусы
	switch resp.StatusCode {

	case http.StatusOK:
		// Декодируем тело только для 200
		var ar AccrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
			c.logger.Errorw("decode accrual response failed", zap.Error(err), zap.String("order", orderNumber))
			return nil, err
		}
		c.logger.Infow("accrual status received", zap.String("order", ar.Order), zap.String("status", ar.Status), zap.Float64("accrual", ar.Accrual))
		return &ar, nil
		// OK, заказ найден

	case http.StatusNoContent:
		// Заказ не найден, можно считать, что Invalid или New
		c.logger.Info("order not found in accrual", zap.String("order", orderNumber))
		return nil, fmt.Errorf("order not register in accrual")

	case http.StatusTooManyRequests:
		// Ограничение по запросам - попробуйте позже
		c.logger.Info("accrual rate limited", zap.String("order", orderNumber))
		return nil, fmt.Errorf("accrual rate limited")

	case http.StatusInternalServerError:
		// Accrual упал - retry
		c.logger.Warn("accrual internal error", zap.String("order", orderNumber))
		return nil, fmt.Errorf("accrual internal error")

	// Неожиданный статус
	default:
		c.logger.Error("unexpected accrual status", zap.Int("status", resp.StatusCode),
			zap.String("order", orderNumber))
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}

func (c *Client) RegisterOrder(ctx context.Context, orderNumber string) error {

	url := c.baseURL + "/api/orders"

	payload := struct {
		Order string `json:"order"`
	}{Order: orderNumber}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal register order payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		bytes.NewReader(body))
	if err != nil {
		return err
	}

	// Указываем заголовок
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("accrual register failed: %d", resp.StatusCode)
	}

	c.logger.Info("order registered in accrual", "number", orderNumber)
	return nil
}
