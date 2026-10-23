package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"rabbitmq-example/internal/platform/logger"
)

type DBConfig struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

func openDB(driver, dsn string, cfg DBConfig) (*sql.DB, error) {
	database, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, logger.WithStack(
			fmt.Errorf("open %s: %w", driver, err),
		)
	}

	database.SetMaxOpenConns(poolValue(cfg.MaxOpenConns, defaultMaxOpenConns))
	database.SetMaxIdleConns(poolValue(cfg.MaxIdleConns, defaultMaxIdleConns))
	database.SetConnMaxLifetime(durationValue(cfg.ConnMaxLifetime, defaultConnMaxLifetime))
	database.SetConnMaxIdleTime(durationValue(cfg.ConnMaxIdleTime, defaultConnMaxIdleTime))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, logger.WithStack(
			fmt.Errorf("ping %s: %w", driver, err),
		)
	}

	return database, nil
}

func poolValue(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}

	return value
}

func durationValue(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}

	return value
}
