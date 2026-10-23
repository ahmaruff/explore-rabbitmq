package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"rabbitmq-example/internal/platform/logger"
)

type UserRepository struct {
	db *sql.DB
}

var _ Repository = (*UserRepository)(nil)

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) FindById(ctx context.Context, id string) (*User, error) {

	row := r.db.QueryRowContext(ctx, `
		SELECT id::text, name, username, email, password, created_at
		FROM users
		WHERE id = $1
	`, id)

	user, err := scanUser(row)

	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id::text, name, username, email, password, created_at
		FROM users
		WHERE email = $1
	`, email)

	user, err := scanUser(row)

	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id::text, name, username, email, password, created_at
		FROM users
		WHERE username = $1
	`, username)

	user, err := scanUser(row)

	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}

	return user, nil
}

// MutedUntil returns the stored mute time, or nil when the column is NULL.
// The caller decides whether the mute is still active. ErrUserNotFound is
// returned when the user does not exist.
func (r *UserRepository) MutedUntil(ctx context.Context, userID int) (*time.Time, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT muted_until
		FROM users
		WHERE id = $1
	`, userID)

	var mutedUntil sql.NullTime

	err := row.Scan(&mutedUntil)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, logger.WithStack(fmt.Errorf("scan muted until: %w", err))
	}

	if !mutedUntil.Valid {
		return nil, nil
	}

	return &mutedUntil.Time, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var user User

	err := row.Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, logger.WithStack(fmt.Errorf("scan user row: %w", err))
	}

	return &user, nil
}
