package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	Id         string           `json:"id"`
	Name       string           `json:"name,omitempty"`
	Email      string           `json:"email,omitempty"`
	Password   string           `json:"password,omitempty"`
	Containers *[]ContainerInfo `json:"project,omitempty"`
}

type UserService struct {
	repo *UserRepo
}

func NewUserService(pool *pgxpool.Pool) *UserService {
	return &UserService{
		repo: NewUserRepo(pool),
	}
}

func (s *UserService) CreateUser(ctx context.Context, name, email, password string) (string, error) {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	user := User{
		Name:     name,
		Email:    email,
		Password: password,
	}

	id, err := s.repo.CreateUser(ctx, tx, user)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return id, nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (User, error) {
	return s.repo.GetUserByID(ctx, s.repo.GetDB(), id)
}

func (s *UserService) GetUserWithContainers(ctx context.Context, id string) (User, []ContainerInfo, error) {
	return s.repo.GetContainersForUserByID(ctx, s.repo.GetDB(), id)
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.DeleteUserByID(ctx, tx, id); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
