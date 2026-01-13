package middleware

import (
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net/http"
)

type ErrorHandler struct {
	logger *zap.SugaredLogger
}

func NewErrorHandler(logger *zap.SugaredLogger) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

func (e *ErrorHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				err := errors.Errorf("panic recovered: %v", rec)
				e.logger.Error("panic recovered", zap.Error(err), zap.Stack("stack"))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func WithErrorLogging(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//Оборачивание writer, для статуса
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status >= 400 {
				logger.Warnw("request failed",
					"method", r.Method,
					"path", r.URL.Path,
					"status", status,
					"remote_addr", r.RemoteAddr)
			}
		})
	}
}
