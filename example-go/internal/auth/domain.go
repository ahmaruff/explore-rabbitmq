package auth

import (
	"context"
	"time"
)

type User struct {
	ID           string
	Name         string
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Repository interface {
	FindById(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
}
