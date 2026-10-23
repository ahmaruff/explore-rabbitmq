package auth

import (
	"net/http"
	"strings"

	platformhttp "rabbitmq-example/internal/platform/http"
	"rabbitmq-example/internal/platform/identity"
	"rabbitmq-example/internal/platform/logger"

	"github.com/labstack/echo/v5"
)

func JWTMiddleware(jwtService *JWTService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get("Authorization")

			parts := strings.Split(header, " ")

			if len(parts) != 2 ||
				!strings.EqualFold(parts[0], "Bearer") ||
				strings.TrimSpace(parts[1]) == "" {
				return platformhttp.Error(c, http.StatusUnauthorized, "Missing or invalid Authorization header")
			}

			claims, err := jwtService.Validate(strings.TrimSpace(parts[1]))

			if err != nil {
				logger.Ctx(c.Request().Context()).Warn().
					Err(err).
					Msg("JWT validation failed")

				return platformhttp.Error(c, http.StatusUnauthorized, "Invalid or expired token")
			}

			identity.SetUserID(c, claims.Subject)

			return next(c)

		}
	}
}
