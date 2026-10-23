package auth

import (
	"errors"
	"net/http"

	"rabbitmq-example/internal/platform/identity"
)

var (
	ErrUserNotFound      = identity.ErrUserNotFound
	ErrEmailAlreadyExist = errors.New("email already exist")
	ErrInvalidCredential = errors.New("invalid credential")
)

type HTTPError struct {
	Status  int
	Message string
}

func MapError(err error) (HTTPError, bool) {
	if errors.Is(err, ErrUserNotFound) {
		return HTTPError{Status: http.StatusNotFound, Message: "User not found"}, true
	}

	if errors.Is(err, ErrInvalidCredential) {
		return HTTPError{Status: http.StatusUnauthorized, Message: "Invalid credentials"}, true
	}

	if errors.Is(err, ErrEmailAlreadyExist) {
		return HTTPError{Status: http.StatusConflict, Message: "Email already exists"}, true
	}

	return HTTPError{}, false
}
