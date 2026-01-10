package config

import (
	"flag"
	"os"
)

type Config struct {
	RunAddr           string
	DatabaseURI       string
	AccrualSystemAddr string
	JWTKey            string
}

func Load() *Config {
	var c Config

	flag.StringVar(&c.RunAddr, "a", ":8080", "address to run HTTP server")
	flag.StringVar(&c.DatabaseURI, "d", "", "database URI")
	flag.StringVar(&c.AccrualSystemAddr, "r", "http://localhost:8081", "accrual system address")
	flag.Parse()

	if envRun := os.Getenv("RUN_ADDRESS"); envRun != "" {
		c.RunAddr = envRun
	}
	if envDB := os.Getenv("DATABASE_URL"); envDB != "" {
		c.DatabaseURI = envDB
	}

	if envAccrual := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrual != "" {
		c.AccrualSystemAddr = envAccrual
	}
	if envJwt := os.Getenv("JWT_SECRET"); envJwt != "" {
		c.JWTKey = envJwt
	} else {
		c.JWTKey = "secret-key"
	}
	return &c
}
