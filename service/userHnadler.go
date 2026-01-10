package service

import (
	jwtSerivce "docker-wrapper/jwt"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)


type UserHandler struct {
	userService *UserService
}


type LoginRequest struct {
	Email  *string `json:email`
	Password *string `json:password`
}

func NewUserHandler(pool *pgxpool.Pool) *UserHandler{

	userService := NewUserService(pool)

	return &UserHandler{
		userService: userService,
	}
}


func (u *UserHandler) respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (u *UserHandler) respondError(w http.ResponseWriter, message string, status int) {
	u.respondJSON(w, APIResponse{Success: false, Error: message}, status)
}


func (u *UserHandler) Login(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		u.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}


	var login LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&login); err != nil {
		u.respondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if login.Email == nil || login.Password == nil {
		u.respondError(w, "email and password are required", http.StatusBadRequest)
		return
	}
	

	user, err := u.userService.GetUserByEmail(r.Context(), *login.Email)

	if err != nil {
		u.respondError(w, UserNotFound.Error(), http.StatusNotFound)
		return
	}


	tocken , err := jwtSerivce.CreateToken(user.Name,user.Email,user.Id)

	if err != nil {
		u.respondError(w, "internal server Eroor", http.StatusNotFound)
		fmt.Errorf("errow in creation of token",err)
		return
	}


	u.respondJSON(w, APIResponse{Success: true, Data: tocken}, http.StatusOK)

}
