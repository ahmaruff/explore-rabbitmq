package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rabbitmq-example/internal/platform/config"
	"rabbitmq-example/internal/platform/db"
	platformhttp "rabbitmq-example/internal/platform/http"
	"rabbitmq-example/internal/platform/logger"

	"rabbitmq-example/internal/auth"
	internalrmq "rabbitmq-example/internal/rabbitmq"
	"rabbitmq-example/internal/sub"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		// The logger is not configured yet, so this reaches stdout only.
		logger.L().Error().Stack().Err(err).Msg("Failed to load config")
		return err
	}

	if logErr := setupLogger(cfg); logErr != nil {
		logger.L().Warn().Err(logErr).Msg("file logging disabled, stdout only")
	}
	defer logger.Close()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStartup()

	server := platformhttp.NewServer(cfg.HTTPPort)

	rmqEnv, rmqConn, err := internalrmq.NewConnection(startupCtx, cfg.BrokerURI)
	if err != nil {
		logger.L().Error().Stack().Err(err).Msg("Failed to connect to RabbitMQ")
		return err
	}
	defer func() {
		if err := internalrmq.CloseConnection(rmqEnv); err != nil {
			logger.L().Error().Stack().Err(err).Msg("Failed to close RabbitMQ")
		}
	}()

	if _, err := internalrmq.NewQueue(startupCtx, rmqConn, cfg.QueueName); err != nil {
		logger.L().Error().Stack().Err(err).Msg("Failed to declare queue")
		return err
	}

	consumer, err := internalrmq.NewConsumer(startupCtx, rmqConn, cfg.QueueName)
	if err != nil {
		logger.L().Error().Stack().Err(err).Msg("Failed to create consumer")
		return err
	}
	defer func() {
		if err := internalrmq.CloseConsumer(consumer); err != nil {
			logger.L().Error().Stack().Err(err).Msg("Failed to close consumer")
		}
	}()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	handler := sub.NewHandler()
	sub.RegisterRoutes(server.Echo(), handler)

	go func() {
		err := internalrmq.Consume(ctx, consumer, func(_ context.Context, body []byte) error {
			handler.Broadcast(string(body))
			return nil
		})
		if err != nil {
			logger.L().Error().Stack().Err(err).Msg("Consumer stopped")
		}
	}()

	logger.L().Info().
		Str("address", server.Address()).
		Msg("Subscriber HTTP server starting")

	if err := server.Start(ctx); err != nil {
		logger.L().Error().Stack().Err(err).Msg("HTTP server failed")
		return fmt.Errorf("Failed to start HTTP server: %w", err)
	}

	logger.L().Info().Msg("Subscriber HTTP server stopped")
	return nil
}

func loadConfig() (config.Config, error) {
	if err := godotenv.Load(".env.sub"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return config.Config{}, fmt.Errorf("failed to load config: %w", err)
		}
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return config.Config{}, err
	}

	if err := cfg.ValidateBroker(); err != nil {
		return config.Config{}, err
	}

	return cfg, nil
}

func setupLogger(cfg config.Config) error {
	return logger.Init(logger.Config{
		Env:         cfg.AppEnv,
		Level:       cfg.LogLevel,
		Service:     cfg.AppName,
		Version:     cfg.AppVersion,
		File:        cfg.LogFile,
		MaxSizeMB:   cfg.LogMaxSizeMB,
		MaxBackups:  cfg.LogMaxBackups,
		MaxAgeDays:  cfg.LogMaxAgeDays,
		Compression: "gzip",
	})

}

func openDatabase(cfg config.Config) (*sql.DB, error) {
	database, err := db.NewPostgres(db.DBConfig{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		Database: cfg.DBDatabase,
		Username: cfg.DBUsername,
		Password: cfg.DBPassword,

		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.DBConnMaxLifetimeMinutes) * time.Minute,
		ConnMaxIdleTime: time.Duration(cfg.DBConnMaxIdleMinutes) * time.Minute,
	})

	if err != nil {
		// The stack is captured where the error was born (db.New); here we only
		// add context, so formatting with %w keeps it intact.
		return nil, fmt.Errorf("connect database: %w", err)
	}

	return database, nil
}

func setupJWTService(cfg config.Config) (*auth.JWTService, error) {
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if cfg.JWTTTLMinutes <= 0 {
		return nil, fmt.Errorf("JWT_TTL_MINUTES must be greater than zero")
	}

	jwtService := auth.NewService(cfg.JWTSecret, cfg.JWTIssuer, time.Duration(cfg.JWTTTLMinutes)*time.Minute)

	return jwtService, nil
}
