package identity

import (
	"strconv"

	"github.com/labstack/echo/v5"
)

const (
	contextKey    = "auth_user_id"
	contextIntKey = "auth_user_id_int"
)

func SetUserID(c *echo.Context, userID string) {
	c.Set(contextKey, userID)

	id, err := strconv.Atoi(userID)

	if err == nil && id > 0 {
		c.Set(contextIntKey, id)
	}
}

func UserID(c *echo.Context) (string, bool) {
	userID, ok := c.Get(contextKey).(string)

	if !ok || userID == "" {
		return "", false
	}

	return userID, true
}

func UserIDInt(c *echo.Context) (int, bool) {
	id, ok := c.Get(contextIntKey).(int)

	if !ok || id <= 0 {
		return 0, false
	}

	return id, true
}
