package hello

import (
	platformhttp "rabbitmq-example/internal/platform/http"

	"github.com/labstack/echo/v5"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Get(c *echo.Context) error {
	g := NewGreeting()

	return platformhttp.OK(c, "greeting retrieved successfully", g)
}
