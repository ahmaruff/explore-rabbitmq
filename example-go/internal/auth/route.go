package auth

import (
	"github.com/labstack/echo/v5"
)

func RegisterRoutes(e *echo.Echo, protected echo.MiddlewareFunc, service *Service) {
	h := NewHandler(service)

	authorized := e.Group("/auth", protected)

	authorized.GET("/me", h.GetMe)
}
