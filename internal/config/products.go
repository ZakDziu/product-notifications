package config

import (
	"fmt"
	"os"
	"time"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultMigrationsPath  = "migrations/products"
	defaultShutdownTimeout = 10 * time.Second

	defaultDBMaxOpenConns    = 25
	defaultDBMaxIdleConns    = 5
	defaultDBConnMaxLifetime = 5 * time.Minute
	defaultDBPingTimeout     = 5 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
)

type Products struct {
	DatabaseURL        string
	RabbitMQURL        string
	HTTPAddr           string
	MigrationsPath     string
	ShutdownTimeout    time.Duration
	DBMaxOpenConns     int
	DBMaxIdleConns     int
	DBConnMaxLifetime  time.Duration
	DBPingTimeout      time.Duration
	ReadHeaderTimeout  time.Duration
}

func LoadProducts() (Products, error) {
	cfg := Products{
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		RabbitMQURL:        getEnv("RABBITMQ_URL", ""),
		HTTPAddr:           getEnv("HTTP_ADDR", defaultHTTPAddr),
		MigrationsPath:     getEnv("MIGRATIONS_PATH", defaultMigrationsPath),
		ShutdownTimeout:    defaultShutdownTimeout,
		DBMaxOpenConns:     defaultDBMaxOpenConns,
		DBMaxIdleConns:     defaultDBMaxIdleConns,
		DBConnMaxLifetime:  defaultDBConnMaxLifetime,
		DBPingTimeout:      defaultDBPingTimeout,
		ReadHeaderTimeout:  defaultReadHeaderTimeout,
	}

	if cfg.DatabaseURL == "" {
		return Products{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RabbitMQURL == "" {
		return Products{}, fmt.Errorf("RABBITMQ_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
