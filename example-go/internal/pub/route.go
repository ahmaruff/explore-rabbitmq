package pub

import "github.com/labstack/echo/v5"

func RegisterRoutes(e *echo.Echo, h *Handler) {
	e.GET("/", h.Page)
	e.GET("/send-msg", h.Send)
}
