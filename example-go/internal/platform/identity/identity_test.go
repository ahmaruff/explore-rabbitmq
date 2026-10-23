package identity

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func newContext() *echo.Context {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	return echo.New().NewContext(request, recorder)
}

func TestUserIDInt(t *testing.T) {
	c := newContext()

	SetUserID(c, "42")

	id, ok := UserIDInt(c)
	if !ok || id != 42 {
		t.Fatalf("UserIDInt() = (%d, %v), want (42, true)", id, ok)
	}
}

func TestUserIDIntRejectsNonNumericSubject(t *testing.T) {
	c := newContext()

	SetUserID(c, "not-a-number")

	if _, ok := UserIDInt(c); ok {
		t.Fatal("UserIDInt() ok = true, want false")
	}
}

func TestUserIDIntRejectsZero(t *testing.T) {
	c := newContext()

	SetUserID(c, "0")

	if _, ok := UserIDInt(c); ok {
		t.Fatal("UserIDInt() ok = true, want false")
	}
}
