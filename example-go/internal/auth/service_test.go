package auth

import (
	"context"
	"testing"
)

type fakeUserRepository struct {
	user *User
	err  error
}

func (r *fakeUserRepository) FindById(ctx context.Context, id string) (*User, error) {
	return r.user, r.err
}

func (r *fakeUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	return r.user, r.err
}

func (r *fakeUserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	return r.user, r.err
}

func TestServiceGetUser(t *testing.T) {
	want := &User{ID: "42", Name: "Ahmad"}
	service := NewUserService(&fakeUserRepository{user: want})

	got, err := service.GetUser(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if got != want {
		t.Errorf("GetUser() = %v, want %v", got, want)
	}
}
