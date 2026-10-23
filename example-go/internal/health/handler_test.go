package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

type fakePinger struct {
	err error
}

func (p fakePinger) PingContext(ctx context.Context) error {
	return p.err
}

func TestHealthReturnsOK(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, fakePinger{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not json: %v", err)
	}

	if !body.Success || body.Code != http.StatusOK {
		t.Errorf("envelope = %+v, want success with code 200", body)
	}

	if body.Data.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Data.Status, "ok")
	}
}

func TestHealthReturnsServiceUnavailableWhenDatabaseFails(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, fakePinger{err: errors.New("connection refused")})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not json: %v", err)
	}

	if body.Success || body.Code != http.StatusServiceUnavailable {
		t.Errorf("envelope = %+v, want failure with code 503", body)
	}
}
