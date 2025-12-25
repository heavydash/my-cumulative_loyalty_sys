package main

import (
	"context"
	"database/sql"
	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/config"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/handler"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/server"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	logger := server.NewLogger("info")

	logger.Infow("Starting Gophermart",
		"run_adress", cfg.RunAddr,
		"database_url", cfg.DatabaseURL,
		"accrual_address", cfg.AccrualSystemAddr,
	)

	if cfg.DatabaseURL == "" {
		logger.Fatal("Database URL is empty")
	}

	// БД
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("db open error", zap.Error(err))
	}
	defer db.Close()

	// Миграции
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		logger.Fatal("Migrate driver", zap.Error(err))
	}
	dir, err := os.Getwd()
	if err != nil {
		logger.Fatal("Get working dir", zap.Error(err))
	}
	migPath := filepath.Join(dir, "migrations")
	m, err := migrate.NewWithDatabaseInstance("file://"+migPath, "postgres", driver)
	if err != nil {
		logger.Fatal("Migrate init", zap.Error(err))
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Fatal("Migrate up", zap.Error(err))
	}

	r := chi.NewRouter()

	userStorage := storage.NewUserStorage(db)
	signingKey := []byte(cfg.JWTKey)

	UserHandler := handler.NewUserHandler(userStorage, signingKey)

	r.Group(func(r chi.Router) {
		r.Use(auth.Auth([]byte(cfg.JWTKey)))
		r.Get("/api/user/balance", handler.GetBalance)
	})

	r.Post("/api/user/register", UserHandler.Register)
	r.Post("/api/user/login", UserHandler.Login)

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
