package service

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	jwtService "github.com/dahhou-ilyas/host-container/jwt"
	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type RegisterRequest struct {
	Name     string `json:"name"     validate:"required,min=2,max=100"`
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// AuthTokenResponse is returned by login, register, and refresh endpoints.
type AuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type UserHandler struct {
	userService *UserService
}

func NewUserHandler(pool *pgxpool.Pool) *UserHandler {
	return &UserHandler{userService: NewUserService(pool)}
}

func (u *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := utils.Validator().Struct(req); err != nil {
		utils.RespondError(w, "validation error: "+err.Error(), http.StatusBadRequest)
		return
	}

	user, err := u.userService.GetUserByEmailWithPassword(r.Context(), req.Email)
	if err != nil {
		utils.RespondError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := utils.VerifyPassword(user.Password, req.Password); err != nil {
		utils.RespondError(w, "invalid email or password", http.StatusUnauthorized)
		return
	}

	tokens, err := issueTokenPair(user.Name, user.Email, user.Id)
	if err != nil {
		slog.Error("failed to create tokens", "error", err)
		utils.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: tokens}, http.StatusOK)
}

func (u *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := utils.Validator().Struct(req); err != nil {
		utils.RespondError(w, "validation error: "+err.Error(), http.StatusBadRequest)
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		utils.RespondError(w, "error processing password", http.StatusInternalServerError)
		return
	}

	userId, err := u.userService.CreateUser(r.Context(), req.Name, req.Email, hashedPassword)
	if err != nil {
		utils.RespondError(w, "error creating user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tokens, err := issueTokenPair(req.Name, req.Email, userId)
	if err != nil {
		slog.Error("failed to create tokens", "error", err)
		utils.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: tokens}, http.StatusCreated)
}

func (u *UserHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		utils.RespondError(w, "refresh token required", http.StatusUnauthorized)
		return
	}

	userId, err := jwtService.VerifyRefreshToken(tokenStr)
	if err != nil {
		utils.RespondError(w, "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	user, err := u.userService.GetUser(r.Context(), userId)
	if err != nil {
		utils.RespondError(w, "user not found", http.StatusUnauthorized)
		return
	}

	accessToken, err := jwtService.CreateToken(user.Name, user.Email, user.Id)
	if err != nil {
		slog.Error("failed to create access token", "error", err)
		utils.RespondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: map[string]string{
		"access_token": accessToken,
	}}, http.StatusOK)
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

	if err := u.userService.DeleteUser(r.Context(), userId); err != nil {
		utils.RespondError(w, "error deleting user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "user deleted successfully"}, http.StatusOK)
}

// issueTokenPair creates both an access token and a refresh token.
func issueTokenPair(name, email, userId string) (AuthTokenResponse, error) {
	access, err := jwtService.CreateToken(name, email, userId)
	if err != nil {
		return AuthTokenResponse{}, err
	}
	refresh, err := jwtService.CreateRefreshToken(userId)
	if err != nil {
		return AuthTokenResponse{}, err
	}
	return AuthTokenResponse{AccessToken: access, RefreshToken: refresh}, nil
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return r.URL.Query().Get("token")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return ""
}
