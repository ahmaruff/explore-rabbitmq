package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestErrorHandlerUsesEnvelopeForUnknownRoute(t *testing.T) {
	server := NewServer("0")

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body Response

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not json: %v", err)
	}

	if body.Success {
		t.Error("success = true, want false")
	}

	if body.Code != http.StatusNotFound {
		t.Errorf("code = %d, want %d", body.Code, http.StatusNotFound)
	}
}

func TestStaticServesHead(t *testing.T) {
	server := NewServer("0")

	server.Static("/files/*", "/files", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodHead, "/files/example.txt", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRedactURI(t *testing.T) {
	got := redactURI("/files/report.pdf?expires=123&signature=super-secret")

	if strings.Contains(got, "super-secret") {
		t.Fatalf("redacted uri still contains signature: %q", got)
	}

	if !strings.Contains(got, "signature=REDACTED") {
		t.Errorf("redacted uri = %q, want signature=REDACTED", got)
	}
}

func TestErrorHandlerUsesEnvelopeForPanic(t *testing.T) {
	server := NewServer("0")

	server.Echo().GET("/panic", func(c *echo.Context) error {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	server.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var body Response

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not json: %v", err)
	}

	if body.Success {
		t.Error("success = true, want false")
	}

	if body.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want %d", body.Code, http.StatusInternalServerError)
	}
}
