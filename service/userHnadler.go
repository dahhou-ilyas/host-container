package service

import "github.com/jackc/pgx/v5/pgxpool"


type UserHandler struct {
	userService *UserService
}


func newUserHandler(pool *pgxpool.Pool) *UserHandler{

	userService := NewUserService(pool)

	return &UserHandler{
		userService: userService,
	}
}
