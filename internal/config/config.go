package config

import (
	"flag"
	"os"
)

type Config struct {
	RunAddr           string
	DatabaseURL       string
	AccrualSystemAddr string
}

func Load() *Config {
	var c Config

	flag.StringVar(&c.RunAddr, "a", "8080", "adress to run HTTP server")
	flag.StringVar(&c.DatabaseURL, "b", "", "database URL")
	flag.StringVar(&c.AccrualSystemAddr, "c", "http://localhost:8081", "accrual system address")
	flag.Parse()

	if env := os.Getenv("RUN_ADDRESS"); env != "" && c.RunAddr == "localhost:8080" {
		c.RunAddr = env
	}
	if env := os.Getenv("DATABASE_URI"); env != "" && c.DatabaseURL == "" {
		c.DatabaseURL = env
	}
	if env := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); env != "" && c.AccrualSystemAddr == "http://localhost:8081" {
		c.AccrualSystemAddr = env
	}
	return &c
}
