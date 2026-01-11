package service

import (
	"context"
	jwtSerivce "docker-wrapper/jwt"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userIDKey contextKey = "userID"


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

func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
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


	user, err := u.userService.GetUserByEmailWithPassword(r.Context(), *login.Email)

	if err != nil {
		u.respondError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := VerifyPassword(user.Password, *login.Password); err != nil {
		u.respondError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}


	tocken , err := jwtSerivce.CreateToken(user.Name, user.Email, user.Id)

	if err != nil {
		u.respondError(w, "internal server error", http.StatusInternalServerError)
		fmt.Printf("error in creation of token: %v\n", err)
		return
	}


	u.respondJSON(w, APIResponse{Success: true, Data: tocken}, http.StatusOK)

}


func (u *UserHandler) Register(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		u.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}


	var register RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&register); err != nil {
		u.respondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if register.Name == nil || *register.Name == "" {
		u.respondError(w, "name is required", http.StatusBadRequest)
		return
	}

	if register.Email == nil || *register.Email == "" {
		u.respondError(w, "email is required", http.StatusBadRequest)
		return
	}

	if register.Password == nil || *register.Password == "" {
		u.respondError(w, "password is required", http.StatusBadRequest)
		return
	}

	hashedPassword, err := HashPassword(*register.Password)
	if err != nil {
		u.respondError(w, "error processing password", http.StatusInternalServerError)
		fmt.Printf("error hashing password: %v\n", err)
		return
	}

	userId, err := u.userService.CreateUser(r.Context(), *register.Name, *register.Email, hashedPassword)

	if err != nil {
		u.respondError(w, "error creating user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tocken, err := jwtSerivce.CreateToken(*register.Name, *register.Email, userId)

	if err != nil {
		u.respondError(w, "internal server error", http.StatusInternalServerError)
		fmt.Printf("error in creation of token: %v\n", err)
		return
	}


	u.respondJSON(w, APIResponse{Success: true, Data: tocken}, http.StatusCreated)

}


func (u *UserHandler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			u.respondError(w, "authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			u.respondError(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		err := jwtSerivce.VerifyToken(tokenString)

		if err != nil {
			u.respondError(w, "invalid Token", http.StatusUnauthorized)
			return
		}

		userId, err := jwtSerivce.GetUserIDFromToken(tokenString)
		if err != nil {
			u.respondError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userId)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}


func (u *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		u.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		u.respondError(w, "user id not found in context", http.StatusUnauthorized)
		return
	}

	user, err := u.userService.GetUser(r.Context(), userId)
	if err != nil {
		u.respondError(w, "user not found", http.StatusNotFound)
		return
	}

	u.respondJSON(w, APIResponse{Success: true, Data: user}, http.StatusOK)
}


func (u *UserHandler) GetUserWithContainers(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		u.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		u.respondError(w, "user id not found in context", http.StatusUnauthorized)
		return
	}

	user, containers, err := u.userService.GetUserWithContainers(r.Context(), userId)
	if err != nil {
		u.respondError(w, "user not found", http.StatusNotFound)
		return
	}

	user.Containers = &containers

	u.respondJSON(w, APIResponse{Success: true, Data: user}, http.StatusOK)
}


func (u *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodDelete {
		u.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userId, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		u.respondError(w, "user id not found in context", http.StatusUnauthorized)
		return
	}

	err := u.userService.DeleteUser(r.Context(), userId)
	if err != nil {
		u.respondError(w, "error deleting user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	u.respondJSON(w, APIResponse{Success: true, Data: "user deleted successfully"}, http.StatusOK)
}


