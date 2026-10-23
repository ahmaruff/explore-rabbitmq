package auth

import (
	"net/http"
	"time"

	platformhttp "rabbitmq-example/internal/platform/http"
	"rabbitmq-example/internal/platform/identity"
	"rabbitmq-example/internal/platform/logger"

	"github.com/labstack/echo/v5"
)

type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) GetMe(c *echo.Context) error {
	userID, ok := identity.UserID(c)

	if !ok {
		return platformhttp.Error(c, http.StatusUnauthorized, "Unauthenticated")
	}

	user, err := h.service.GetUser(c.Request().Context(), userID)

	if err != nil {
		return h.writeError(c, err, "Failed to load user")
	}

	return platformhttp.OK(c, "OK", UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	})
}

func (h *Handler) writeError(c *echo.Context, err error, logMessage string) error {
	mapped, ok := MapError(err)

	if ok {
		return platformhttp.Error(c, mapped.Status, mapped.Message)
	}

	logger.Err(c.Request().Context(), err).Msg(logMessage)

	return platformhttp.Error(c, http.StatusInternalServerError, "Something went wrong")
}
