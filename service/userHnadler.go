package service

import (
	jwtSerivce "github.com/dahhou-ilyas/host-container/jwt"
	"github.com/dahhou-ilyas/host-container/utils"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)




type UserHandler struct {
	userService *UserService
}


type LoginRequest struct {
	Email  *string `json:"email"`
	Password *string `json:"password"`
}

type RegisterRequest struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
}



func NewUserHandler(pool *pgxpool.Pool) *UserHandler{

	userService := NewUserService(pool)

	return &UserHandler{
		userService: userService,
	}
}





func (u *UserHandler) Login(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}


	var login LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&login); err != nil {
		utils.RespondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if login.Email == nil || login.Password == nil {
		utils.RespondError(w, "email and password are required", http.StatusBadRequest)
		return
	}


	user, err := u.userService.GetUserByEmailWithPassword(r.Context(), *login.Email)

	if err != nil {
		utils.RespondError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := utils.VerifyPassword(user.Password, *login.Password); err != nil {
		utils.RespondError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}


	tocken , err := jwtSerivce.CreateToken(user.Name, user.Email, user.Id)

	if err != nil {
		utils.RespondError(w, "internal server error", http.StatusInternalServerError)
		fmt.Printf("error in creation of token: %v\n", err)
		return
	}


	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: tocken}, http.StatusOK)

}


func (u *UserHandler) Register(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}


	var register RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&register); err != nil {
		utils.RespondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if register.Name == nil || *register.Name == "" {
		utils.RespondError(w, "name is required", http.StatusBadRequest)
		return
	}

	if register.Email == nil || *register.Email == "" {
		utils.RespondError(w, "email is required", http.StatusBadRequest)
		return
	}

	if register.Password == nil || *register.Password == "" {
		utils.RespondError(w, "password is required", http.StatusBadRequest)
		return
	}

	hashedPassword, err := utils.HashPassword(*register.Password)
	if err != nil {
		utils.RespondError(w, "error processing password", http.StatusInternalServerError)
		fmt.Printf("error hashing password: %v\n", err)
		return
	}

	userId, err := u.userService.CreateUser(r.Context(), *register.Name, *register.Email, hashedPassword)

	if err != nil {
		utils.RespondError(w, "error creating user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tocken, err := jwtSerivce.CreateToken(*register.Name, *register.Email, userId)

	if err != nil {
		utils.RespondError(w, "internal server error", http.StatusInternalServerError)
		fmt.Printf("error in creation of token: %v\n", err)
		return
	}


	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: tocken}, http.StatusCreated)

}





func (u *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, ok := r.Context().Value(utils.UserIDKey).(string)
	if !ok {
		utils.RespondError(w, "user id not found in context", http.StatusUnauthorized)
		return
	}

	user, err := u.userService.GetUser(r.Context(), userId)
	if err != nil {
		utils.RespondError(w, "user not found", http.StatusNotFound)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: user}, http.StatusOK)
}


func (u *UserHandler) GetUserWithContainers(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, ok := r.Context().Value(utils.UserIDKey).(string)
	if !ok {
		utils.RespondError(w, "user id not found in context", http.StatusUnauthorized)
		return
	}

	user, containers, err := u.userService.GetUserWithContainers(r.Context(), userId)
	if err != nil {
		utils.RespondError(w, "user not found", http.StatusNotFound)
		return
	}

	user.Containers = &containers

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: user}, http.StatusOK)
}


func (u *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodDelete {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, ok := r.Context().Value(utils.UserIDKey).(string)
	if !ok {
		utils.RespondError(w, "user id not found in context", http.StatusUnauthorized)
		return
	}

	err := u.userService.DeleteUser(r.Context(), userId)
	if err != nil {
		utils.RespondError(w, "error deleting user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "user deleted successfully"}, http.StatusOK)
}


