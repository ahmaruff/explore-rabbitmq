package auth

import "context"

type Service struct {
	repository Repository
}

func NewUserService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
	return s.repository.FindById(ctx, id)
}
