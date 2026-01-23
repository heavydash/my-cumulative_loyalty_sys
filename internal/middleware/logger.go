package middleware

import (
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"net/http"
	"time"
)

func AccessLog(logger *zap.SugaredLogger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Оборачиваем writer
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			logger.Info("request received",
				"method", r.Method,
				"path", r.URL.Path,
				"remote address", r.RemoteAddr,
				"request_id", middleware.GetReqID(r.Context()),
			)

			// Выполняем следующий хендлер
			next.ServeHTTP(ww, r)

			duration := time.Since(start)

			// Ответ ушел
			logger.Infow("response sent",
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration", duration,
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
