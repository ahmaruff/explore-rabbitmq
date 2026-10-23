package health

import (
	"github.com/labstack/echo/v5"
)

func RegisterRoutes(e *echo.Echo, db Pinger) {
	h := NewHandler(db)

	e.GET("/health", h.Get)
}
