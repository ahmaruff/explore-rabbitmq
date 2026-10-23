package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "01234567890123456789012345678901"

func TestJWTGenerateAndValidate(t *testing.T) {
	service := NewService(testJWTSecret, "test-issuer", time.Minute)

	token, err := service.Generate("42")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	claims, err := service.Validate(token)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if claims.Subject != "42" {
		t.Errorf("subject = %q, want 42", claims.Subject)
	}
}

func TestJWTRejectsWrongIssuer(t *testing.T) {
	issuer := NewService(testJWTSecret, "issuer-a", time.Minute)
	validator := NewService(testJWTSecret, "issuer-b", time.Minute)

	token, err := issuer.Generate("42")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if _, err := validator.Validate(token); err == nil {
		t.Fatal("Validate() expected issuer error, got nil")
	}
}

func TestJWTRejectsExpiredToken(t *testing.T) {
	service := NewService(testJWTSecret, "test-issuer", -time.Minute)

	token, err := service.Generate("42")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if _, err := service.Validate(token); err == nil {
		t.Fatal("Validate() expected expiry error, got nil")
	}
}

func TestJWTRejectsWrongAlgorithm(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub": "42",
		"exp": time.Now().Add(time.Minute).Unix(),
	})

	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	service := NewService(testJWTSecret, "", time.Minute)

	if _, err := service.Validate(signed); err == nil {
		t.Fatal("Validate() expected algorithm error, got nil")
	}
}

func TestJWTAcceptsNumericSubject(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": 42,
		"exp": time.Now().Add(time.Minute).Unix(),
	})

	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	service := NewService(testJWTSecret, "", time.Minute)

	claims, err := service.Validate(signed)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if claims.Subject != "42" {
		t.Errorf("subject = %q, want 42", claims.Subject)
	}
}
