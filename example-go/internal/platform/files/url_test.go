package files

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestURL(t *testing.T) {
	service, _ := newTestService(t)

	if got := service.URL("2026/10/pic.png"); got != "/files/2026/10/pic.png" {
		t.Errorf("URL = %q, want /files/2026/10/pic.png", got)
	}

	if got := service.URL("/2026/10/pic.png"); got != "/files/2026/10/pic.png" {
		t.Errorf("URL with leading slash = %q, want /files/2026/10/pic.png", got)
	}
}

func TestURLWithBaseURL(t *testing.T) {
	service, err := New(Config{Store: newMemStore(), BaseURL: "http://localhost:8090/"})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	if got := service.URL("2026/10/pic.png"); got != "http://localhost:8090/files/2026/10/pic.png" {
		t.Errorf("URL = %q, want http://localhost:8090/files/2026/10/pic.png", got)
	}

	if got := service.URL("/2026/10/pic.png"); got != "http://localhost:8090/files/2026/10/pic.png" {
		t.Errorf("URL with leading slash = %q, want http://localhost:8090/files/2026/10/pic.png", got)
	}
}

func TestSignedURLUnavailable(t *testing.T) {
	service, _ := newTestService(t)

	_, err := service.SignedURL(context.Background(), "2026/10/pic.png", time.Minute)
	if !errors.Is(err, ErrSigningUnavailable) {
		t.Errorf("SignedURL error = %v, want ErrSigningUnavailable", err)
	}
}

func TestSignedURLLocal(t *testing.T) {
	service, err := New(Config{Store: newMemStore(), SigningKey: "test-signing-key"})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	got, err := service.SignedURL(context.Background(), "2026/10/pic.png", time.Minute)
	if err != nil {
		t.Fatalf("SignedURL error = %v", err)
	}

	pattern := regexp.MustCompile(`^/files/2026/10/pic\.png\?expires=\d+&signature=[0-9a-f]{64}$`)
	if !pattern.MatchString(got) {
		t.Errorf("signed URL = %q, want it to match %s", got, pattern)
	}

	request := httptest.NewRequest(http.MethodGet, got, nil)
	if !service.signatureValid(request, "2026/10/pic.png") {
		t.Error("signatureValid = false, want true")
	}
}

func TestSignedURLIsBoundToTheKey(t *testing.T) {
	service, err := New(Config{Store: newMemStore(), SigningKey: "test-signing-key"})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	signed := service.signedURL("2026/10/pic.png", time.Now().Add(time.Minute))
	request := httptest.NewRequest(http.MethodGet, signed, nil)

	// The same signature must not unlock a different file.
	if service.signatureValid(request, "2026/10/other.png") {
		t.Error("signatureValid for another key = true, want false")
	}
}

func TestSignedURLExpires(t *testing.T) {
	service, err := New(Config{Store: newMemStore(), SigningKey: "test-signing-key"})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	signed := service.signedURL("2026/10/pic.png", time.Now().Add(-time.Minute))
	request := httptest.NewRequest(http.MethodGet, signed, nil)

	if service.signatureValid(request, "2026/10/pic.png") {
		t.Error("signatureValid for an expired link = true, want false")
	}
}

func TestRequireSignatureNeedsKey(t *testing.T) {
	_, err := New(Config{Store: newMemStore(), RequireSignature: true})
	if err == nil {
		t.Error("New error = nil, want error")
	}
}

// signerStore is a memStore that can presign, standing in for the S3 driver.
type signerStore struct {
	*memStore
}

func (s signerStore) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://signed.example.com/" + key, nil
}

func TestSignedURLDelegates(t *testing.T) {
	service, err := New(Config{Store: signerStore{newMemStore()}, SigningKey: "ignored"})
	if err != nil {
		t.Fatalf("New error = %v", err)
	}

	got, err := service.SignedURL(context.Background(), "2026/10/pic.png", time.Minute)
	if err != nil {
		t.Fatalf("SignedURL error = %v", err)
	}

	if want := "https://signed.example.com/2026/10/pic.png"; got != want {
		t.Errorf("SignedURL = %q, want %q", got, want)
	}
}
