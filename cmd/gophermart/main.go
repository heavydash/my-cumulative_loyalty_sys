package main

import (
	"context"
	"database/sql"
	"github.com/go-chi/chi/v5"
	chi_mw "github.com/go-chi/chi/v5/middleware"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/auth"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/config"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/handler"
	my_mw "github.com/heavydash/my-cumulative_loyalty_sys/internal/middleware"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/server"
	"github.com/heavydash/my-cumulative_loyalty_sys/internal/storage"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
	"net/http"
	"os/signal"
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

	var currentDB string
	err = db.QueryRow("SELECT current_database()").Scan(&currentDB)
	if err != nil {
		logger.Fatal("failed to get current database", zap.Error(err))
	}
	logger.Infow("Connected to database", "name", currentDB)

	// Миграции
	logger.Info("running migrations)")
	if err := goose.SetDialect("postgres"); err != nil {
		logger.Fatal("goose set dialect error", zap.Error(err))
	}
	if err := goose.Up(db, "migrations"); err != nil {
		logger.Fatal("Goose up", zap.Error(err))
	}
	logger.Info("migrations done")

	r := chi.NewRouter()

	ErrorHandler := my_mw.NewErrorHandler(logger)

	// Для трейсинга
	r.Use(chi_mw.RequestID)

	r.Use(ErrorHandler.Handle)            // паники
	r.Use(my_mw.WithErrorLogging(logger)) // логирование 400/500
	r.Use(my_mw.AccessLog(logger))

	userStorage := storage.NewUserStorage(db)
	orderStorage := storage.NewOrderStorage(db, logger)
	signingKey := []byte(cfg.JWTKey)

	UserHandler := handler.NewUserHandler(userStorage, signingKey, logger)
	OrderHandler := handler.NewOrderHandler(orderStorage, logger)
	BalanceHandler := handler.NewBalanceHandler(logger)

	r.Group(func(r chi.Router) {
		r.Post("/api/user/register", UserHandler.Register)
		r.Post("/api/user/login", UserHandler.Login)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Auth([]byte(cfg.JWTKey)))
		r.Get("/api/user/balance", BalanceHandler.GetBalance)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Auth([]byte(cfg.JWTKey)))
		r.Post("/api/user/orders", OrderHandler.AddOrder)
		r.Get("/api/user/orders", OrderHandler.GetOrders)
	})

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
