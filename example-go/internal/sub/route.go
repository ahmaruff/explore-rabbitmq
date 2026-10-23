package sub

import "github.com/labstack/echo/v5"

func RegisterRoutes(e *echo.Echo, h *Handler) {
	e.GET("/", h.Page)
	e.GET("/events", h.Stream)
}
