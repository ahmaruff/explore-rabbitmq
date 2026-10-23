package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv     string
	AppName    string
	AppVersion string
	AppURL     string

	HTTPPort string

	LogLevel      string
	LogFile       string
	LogMaxSizeMB  int
	LogMaxBackups int
	LogMaxAgeDays int

	DBHost     string
	DBPort     string
	DBDatabase string
	DBUsername string
	DBPassword string

	DBMaxOpenConns           int
	DBMaxIdleConns           int
	DBConnMaxLifetimeMinutes int
	DBConnMaxIdleMinutes     int

	JWTSecret     string
	JWTIssuer     string
	JWTTTLMinutes int

	Storage StorageConfig
}

const (
	StorageDriverLocal = "local"
	StorageDriverS3    = "s3"
)

type StorageConfig struct {
	Driver      string
	S3BucketURL string

	// SigningKey lets drivers served through the API (local) create and verify
	// signed links. Use a long random value.
	SigningKey string

	// RequireSignature rejects /files requests without a valid signature.
	RequireSignature bool
}

func LoadConfig() (Config, error) {
	var problems []error

	mustInt := func(key string, fallback int) int {
		value, err := getEnvInt(key, fallback)

		if err != nil {
			problems = append(problems, err)
			return fallback
		}

		return value
	}

	mustBool := func(key string, fallback bool) bool {
		value, err := getEnvBool(key, fallback)

		if err != nil {
			problems = append(problems, err)
			return fallback
		}

		return value
	}

	cfg := Config{
		AppEnv:     getEnv("APP_ENV", "development"),
		AppName:    getEnv("APP_NAME", "Sucor Go API"),
		AppVersion: getEnv("APP_VERSION", "1.0.0"),
		AppURL:     getEnv("APP_URL", ""),

		HTTPPort: getEnv("HTTP_PORT", "8090"),

		LogLevel:      getEnv("LOG_LEVEL", "info"),
		LogFile:       getEnv("LOG_FILE", "/app/logs/app.log"),
		LogMaxSizeMB:  mustInt("LOG_MAX_SIZE_MB", 100),
		LogMaxBackups: mustInt("LOG_MAX_BACKUPS", 5),
		LogMaxAgeDays: mustInt("LOG_MAX_AGE_DAYS", 28),

		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBDatabase: os.Getenv("DB_DATABASE"),
		DBUsername: os.Getenv("DB_USERNAME"),
		DBPassword: os.Getenv("DB_PASSWORD"),

		DBMaxOpenConns:           mustInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:           mustInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetimeMinutes: mustInt("DB_CONN_MAX_LIFETIME_MINUTES", 30),
		DBConnMaxIdleMinutes:     mustInt("DB_CONN_MAX_IDLE_MINUTES", 5),

		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTIssuer:     os.Getenv("JWT_ISSUER"),
		JWTTTLMinutes: mustInt("JWT_TTL_MINUTES", 30),

		Storage: StorageConfig{
			Driver:           getEnv("STORAGE_DRIVER", StorageDriverLocal),
			S3BucketURL:      os.Getenv("STORAGE_S3_BUCKET_URL"),
			SigningKey:       os.Getenv("STORAGE_SIGNING_KEY"),
			RequireSignature: mustBool("STORAGE_REQUIRE_SIGNATURE", false),
		},
	}

	if err := errors.Join(problems...); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate fails fast for configuration that would otherwise
// produce a confusing runtime error or an insecure deployment.
func (c Config) Validate() error {
	var problems []string

	if strings.TrimSpace(c.HTTPPort) == "" {
		problems = append(problems, "HTTP_PORT is required")
	} else if port, err := strconv.Atoi(c.HTTPPort); err != nil || port < 1 || port > 65535 {
		problems = append(problems, "HTTP_PORT must be a number between 1 and 65535")
	}

	if strings.TrimSpace(c.DBHost) == "" {
		problems = append(problems, "DB_HOST is required")
	}

	if strings.TrimSpace(c.DBPort) == "" {
		problems = append(problems, "DB_PORT is required")
	} else if port, err := strconv.Atoi(c.DBPort); err != nil || port < 1 || port > 65535 {
		problems = append(problems, "DB_PORT must be a number between 1 and 65535")
	}

	if strings.TrimSpace(c.DBDatabase) == "" {
		problems = append(problems, "DB_DATABASE is required")
	}

	if strings.TrimSpace(c.DBUsername) == "" {
		problems = append(problems, "DB_USERNAME is required")
	}

	if c.JWTSecret == "" {
		problems = append(problems, "JWT_SECRET is required")
	} else if len(c.JWTSecret) < 32 {
		problems = append(problems, "JWT_SECRET must be at least 32 characters")
	}

	if c.JWTTTLMinutes <= 0 {
		problems = append(problems, "JWT_TTL_MINUTES must be greater than zero")
	}

	driver := strings.ToLower(strings.TrimSpace(c.Storage.Driver))

	switch driver {
	case StorageDriverLocal:
	case StorageDriverS3:
		if strings.TrimSpace(c.Storage.S3BucketURL) == "" {
			problems = append(problems, "STORAGE_S3_BUCKET_URL is required when STORAGE_DRIVER=s3")
		}
	default:
		problems = append(problems, "STORAGE_DRIVER must be local or s3")
	}

	if c.Storage.RequireSignature && c.Storage.SigningKey == "" {
		problems = append(problems, "STORAGE_SIGNING_KEY is required when STORAGE_REQUIRE_SIGNATURE=true")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	return parsed, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}

	return parsed, nil
}
