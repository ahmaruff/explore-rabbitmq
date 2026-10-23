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
	"rabbitmq-example/internal/platform/files"
	platformhttp "rabbitmq-example/internal/platform/http"
	"rabbitmq-example/internal/platform/logger"
	"rabbitmq-example/internal/platform/storage"

	"rabbitmq-example/internal/auth"
	"rabbitmq-example/internal/hello"

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

	// start logger
	if logErr := setupLogger(cfg); logErr != nil {
		logger.L().Warn().Err(logErr).Msg("file logging disabled, stdout only")
	}

	defer logger.Close()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStartup()

	// database, err := openDatabase(cfg)
	// if err != nil {
	// 	logger.L().Error().Stack().Err(err).Msg("Failed to connect database")
	// 	return err
	// }

	// defer database.Close()

	server := platformhttp.NewServer(cfg.HTTPPort)

	// jwtService, err := setupJWTService(cfg)

	// if err != nil {
	// 	logger.L().Error().Stack().Err(err).Msg("Failed to setup JWT")
	// 	return err
	// }

	// SETUP STORAGE
	fileStore, err := storage.New(startupCtx, cfg.Storage)
	if err != nil {
		logger.L().Error().Stack().Err(err).Msg("Failed to setup storage")
		return err
	}

	defer fileStore.Close()

	// SETUP FILES
	fileService, err := files.New(files.Config{
		Store:            fileStore,
		BaseURL:          cfg.AppURL,
		SigningKey:       cfg.Storage.SigningKey,
		RequireSignature: cfg.Storage.RequireSignature,
	})
	if err != nil {
		logger.L().Error().Stack().Err(err).Msg("Failed to setup files")
		return err
	}

	// SETUP REPOSITORY
	// userRepository := auth.NewUserRepository(database)
	// authService := auth.NewUserService(userRepository)

	// SETUP SERVICE

	// REGISTER ROUTES
	// protected := auth.JWTMiddleware(jwtService)

	hello.RegisterRoutes(server.Echo())
	// health.RegisterRoutes(server.Echo(), database)
	// auth.RegisterRoutes(server.Echo(), protected, authService)

	server.Static(files.RoutePattern, files.RoutePrefix, fileService)

	logger.L().Info().
		Str("address", server.Address()).
		Msg("HTTP server starting")

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := server.Start(ctx); err != nil {
		logger.L().Error().Stack().Err(err).Msg("HTTP server failed")
		return fmt.Errorf("Failed to start HTTP server: %w", err)
	}

	logger.L().Info().Msg("HTTP server stopped")
	return nil
}

func loadConfig() (config.Config, error) {
	if err := godotenv.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return config.Config{}, fmt.Errorf("failed to load config: %w", err)
		}

		// return config.Config{}, fmt.Errorf("Failed to load config: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return config.Config{}, err
	}

	if err := cfg.Validate(); err != nil {
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
