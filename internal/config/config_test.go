package config

import (
	"flag"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func setupFlags() {
	// Новый чистый набор флагов
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	// Отключаем вывод помощи, чтобы тесты были тихими
	flag.CommandLine.Usage = func() {}
}

func TestLoad(t *testing.T) {
	// Сохраняем оригинальные аргументы командной строки
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Подтест: переменные окружения заданы (они должны переопределить дефолты)
	t.Run("env variables set", func(t *testing.T) {
		os.Clearenv() // очищаем, чтобы тест был изолированным

		// Задаём env-переменные
		os.Setenv("RUN_ADDRESS", ":9090")
		os.Setenv("DATABASE_URI", "postgres://env-db")
		os.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://env-accrual")
		os.Setenv("JWT_SECRET", "env-jwt-secret")
		defer os.Unsetenv("RUN_ADDRESS")
		defer os.Unsetenv("DATABASE_URI")
		defer os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS")
		defer os.Unsetenv("JWT_SECRET")

		setupFlags()
		// Эмулируем запуск без флагов
		os.Args = []string{"cmd"}

		cfg := Load()

		assert.Equal(t, ":9090", cfg.RunAddr)
		assert.Equal(t, "postgres://env-db", cfg.DatabaseURI)
		assert.Equal(t, "http://env-accrual", cfg.AccrualSystemAddr)
		assert.Equal(t, "env-jwt-secret", cfg.JWTKey)
	})

	// Ничего не задано - должны сработать все дефолты
	t.Run("default values", func(t *testing.T) {
		os.Clearenv()

		setupFlags()
		os.Args = []string{"cmd"}

		cfg := Load()

		assert.Equal(t, ":8080", cfg.RunAddr)
		assert.Equal(t, "postgres://postgres:postgres@postgres:5432/praktikum?sslmode=disable", cfg.DatabaseURI)
		assert.Equal(t, "http://localhost:8081", cfg.AccrualSystemAddr)
		assert.Equal(t, "secret-key", cfg.JWTKey)
	})
}

func TestLoad_RunAddr(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Clearenv() // флаги должны работать без env

	t.Run("from flag -a", func(t *testing.T) {
		setupFlags()
		os.Args = []string{"cmd", "-a", ":7070"}

		cfg := Load()
		assert.Equal(t, ":7070", cfg.RunAddr)
	})

	t.Run("empty flag -a", func(t *testing.T) {
		setupFlags()
		os.Args = []string{"cmd", "-a", ""}

		cfg := Load()
		assert.Equal(t, "", cfg.RunAddr) // пустой флаг перезапишет дефолт
	})
}

func TestLoad_DatabaseURI(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("from flag -d", func(t *testing.T) {
		os.Clearenv()
		setupFlags()
		os.Args = []string{"cmd", "-d", "postgres://flag-db"}

		cfg := Load()
		assert.Equal(t, "postgres://flag-db", cfg.DatabaseURI)
	})

	t.Run("empty flag -d", func(t *testing.T) {
		os.Clearenv()
		setupFlags()
		os.Args = []string{"cmd", "-d", ""}

		cfg := Load()
		// Пустой флаг → после парсинга "" → потом проверка if c.DatabaseURI == "" → fallback к дефолту
		assert.Equal(t, "postgres://postgres:postgres@postgres:5432/praktikum?sslmode=disable", cfg.DatabaseURI)
	})
}

func TestLoad_AccrualSystemAddr(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Clearenv()

	t.Run("from flag -r", func(t *testing.T) {
		setupFlags()
		os.Args = []string{"cmd", "-r", "http://flag-accrual:9999"}

		cfg := Load()
		assert.Equal(t, "http://flag-accrual:9999", cfg.AccrualSystemAddr)
	})

	t.Run("empty flag -r", func(t *testing.T) {
		setupFlags()
		os.Args = []string{"cmd", "-r", ""}

		cfg := Load()
		assert.Equal(t, "", cfg.AccrualSystemAddr)
	})
}

func TestLoad_JWTKey(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("from env", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("JWT_SECRET", "test-jwt-key")
		defer os.Unsetenv("JWT_SECRET")

		setupFlags()
		os.Args = []string{"cmd"}

		cfg := Load()
		assert.Equal(t, "test-jwt-key", cfg.JWTKey)
	})

	t.Run("default when env empty", func(t *testing.T) {
		os.Clearenv()
		setupFlags()
		os.Args = []string{"cmd"}

		cfg := Load()
		assert.Equal(t, "secret-key", cfg.JWTKey)
	})
}
