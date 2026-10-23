package health

import (
	"context"
	"net/http"
	"time"

	platformhttp "rabbitmq-example/internal/platform/http"
	"rabbitmq-example/internal/platform/logger"

	"github.com/labstack/echo/v5"
)

const pingTimeout = 2 * time.Second

type Pinger interface {
	PingContext(ctx context.Context) error
}

type Handler struct {
	db Pinger
}

func NewHandler(db Pinger) *Handler {
	return &Handler{db: db}
}

type Response struct {
	Status string `json:"status"`
}

func (h *Handler) Get(c *echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), pingTimeout)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		logger.Err(c.Request().Context(), err).Msg("Health check failed")

		return platformhttp.Error(c, http.StatusServiceUnavailable, "Service unavailable")
	}

	return platformhttp.OK(c, "OK", Response{Status: "ok"})
}
