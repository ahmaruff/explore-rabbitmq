package http

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type Response struct {
	Success bool                `json:"success"`
	Code    int                 `json:"code"`
	Message string              `json:"message,omitempty"`
	Data    any                 `json:"data,omitempty"`
	Errors  map[string][]string `json:"errors,omitempty"`
}

func JSON(c *echo.Context, code int, message string, data any, errorData map[string][]string) error {
	return c.JSON(code, Response{
		Success: code < http.StatusBadRequest,
		Code:    code,
		Message: message,
		Data:    data,
		Errors:  errorData,
	})
}

func OK(c *echo.Context, message string, data any) error {
	return JSON(c, http.StatusOK, message, data, nil)
}

func Created(c *echo.Context, message string, data any) error {
	return JSON(c, http.StatusCreated, message, data, nil)
}

func Error(c *echo.Context, status int, message string) error {
	return JSON(c, status, message, nil, nil)
}

func ValidationError(c *echo.Context, err error, message string) error {
	fields, ok := FormatValidationError(err)

	if !ok {
		return Error(c, http.StatusInternalServerError, "Failed to format validation errors")
	}

	return JSON(c, http.StatusUnprocessableEntity, message, nil, fields)

}
