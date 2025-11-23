package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPPort        string
	DatabaseURL     string
	LogLevel        string
	ShutdownTimeout time.Duration
}

const (
	defaultHTTPPort        = "8080"
	defaultDatabaseURL     = "postgres://avito:avito@localhost:5432/avito?sslmode=disable"
	defaultLogLevel        = "debug"
	defaultShutdownTimeout = "10s"
)

func Load() (Config, error) {
	cfg := Config{
		HTTPPort:    getEnv("HTTP_PORT", defaultHTTPPort),
		DatabaseURL: getEnv("DATABASE_URL", defaultDatabaseURL),
		LogLevel:    getEnv("LOG_LEVEL", defaultLogLevel),
	}

	timeoutRaw := getEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	timeout, err := time.ParseDuration(timeoutRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = timeout

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
