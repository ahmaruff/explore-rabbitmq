package config

import (
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{
		HTTPPort:      "8090",
		DBHost:        "localhost",
		DBPort:        "5432",
		DBDatabase:    "sucor",
		DBUsername:    "sucor",
		JWTSecret:     strings.Repeat("a", 32),
		JWTTTLMinutes: 30,
		Storage: StorageConfig{
			Driver: StorageDriverLocal,
		},
		BrokerURI: "amqp://guest:guest@localhost:5672/",
		QueueName: "hello",
	}
}

func TestLoadConfigRejectsInvalidInteger(t *testing.T) {
	t.Setenv("LOG_MAX_SIZE_MB", "not-a-number")

	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "LOG_MAX_SIZE_MB") {
		t.Fatalf("LoadConfig() error = %v, want LOG_MAX_SIZE_MB error", err)
	}
}

func TestLoadConfigRejectsInvalidBoolean(t *testing.T) {
	t.Setenv("STORAGE_REQUIRE_SIGNATURE", "not-a-bool")

	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "STORAGE_REQUIRE_SIGNATURE") {
		t.Fatalf("LoadConfig() error = %v, want STORAGE_REQUIRE_SIGNATURE error", err)
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsMissingRequiredValues(t *testing.T) {
	cfg := validConfig()
	cfg.DBHost = ""
	cfg.DBUsername = ""
	cfg.JWTSecret = ""

	err := cfg.Validate()

	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}

	for _, want := range []string{"DB_HOST", "DB_USERNAME", "JWT_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() error = %q, want %q", err, want)
		}
	}
}

func TestValidateRejectsShortJWTSecret(t *testing.T) {
	cfg := validConfig()
	cfg.JWTSecret = "short"

	err := cfg.Validate()

	if err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("Validate() error = %v, want minimum length error", err)
	}
}

func TestValidateRejectsS3WithoutBucket(t *testing.T) {
	cfg := validConfig()
	cfg.Storage.Driver = StorageDriverS3

	err := cfg.Validate()

	if err == nil || !strings.Contains(err.Error(), "STORAGE_S3_BUCKET_URL") {
		t.Fatalf("Validate() error = %v, want bucket error", err)
	}
}

func TestValidateRejectsSignatureWithoutKey(t *testing.T) {
	cfg := validConfig()
	cfg.Storage.RequireSignature = true

	err := cfg.Validate()

	if err == nil || !strings.Contains(err.Error(), "STORAGE_SIGNING_KEY") {
		t.Fatalf("Validate() error = %v, want signing key error", err)
	}
}
