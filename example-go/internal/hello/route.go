package hello

import (
	"github.com/labstack/echo/v5"
)

func RegisterRoutes(e *echo.Echo) {
	h := NewHandler()

	e.GET("/", h.Get)
}
