package main

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/config"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/server"
	"go.uber.org/zap"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()
	logger := server.NewLogger("info")

	logger.Infow("Starting Gophermart",
		"run_adress", cfg.RunAddr,
		"database_url", cfg.DatabaseURL,
		"accrual_address", cfg.AccrualSystemAddr,
	)

	r := chi.NewRouter()

	srv := &http.Server{
		Addr:    cfg.RunAddr,
		Handler: r,
	}

	// Gracefull shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}
	logger.Info("Server stopped")
}
