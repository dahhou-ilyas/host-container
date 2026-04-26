package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dahhou-ilyas/host-container/email"
	jwtService "github.com/dahhou-ilyas/host-container/jwt"
	"github.com/dahhou-ilyas/host-container/repository"
	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/jackc/pgx/v5"
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
	pool        *pgxpool.Pool
	tokenRepo   *repository.TokenRepo
	mailer      *email.Mailer
	baseURL     string
}

func NewUserHandler(pool *pgxpool.Pool) *UserHandler {
	return &UserHandler{
		userService: NewUserService(pool),
		pool:        pool,
		tokenRepo:   repository.NewTokenRepo(pool),
		mailer:      email.NewMailer(),
		baseURL:     os.Getenv("APP_BASE_URL"),
	}
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

	// Require email verification before allowing access
	if !user.EmailVerified {
		utils.RespondJSON(w, utils.APIResponse{
			Success: false,
			Error:   "email_not_verified",
		}, http.StatusForbidden)
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

	// Send verification email asynchronously — use Background ctx (request ctx dies after response)
	go func() {
		ctx := context.Background()
		userIDInt := int64(0)
		user, err := u.userService.GetUserByEmail(ctx, req.Email)
		if err == nil {
			userIDInt = user.IDInt()
		}
		if userIDInt == 0 {
			return
		}
		raw, err := u.tokenRepo.Create(ctx, userIDInt, "email_verification", 24*time.Hour)
		if err != nil {
			slog.Warn("failed to create verification token", "err", err)
			return
		}
		verifyURL := u.baseURL + "/verify-email?token=" + raw
		if err := u.mailer.SendVerificationEmail(req.Email, req.Name, verifyURL); err != nil {
			slog.Warn("failed to send verification email", "err", err)
		}
	}()

	_ = userId
	utils.RespondJSON(w, utils.APIResponse{
		Success: true,
		Data:    map[string]string{"message": "account created — please verify your email before logging in"},
	}, http.StatusCreated)
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
		utils.RespondError(w, "method not allowdoed", http.StatusMethodNotAllowed)
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

// POST /auth/verify-email  body: {"token":"<raw>"}
func (u *UserHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Token string `json:"token" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := utils.Validator().Struct(req); err != nil {
		utils.RespondError(w, "token required", http.StatusBadRequest)
		return
	}
	rec, err := u.tokenRepo.Consume(r.Context(), req.Token, "email_verification")
	if err != nil {
		utils.RespondError(w, "invalid or expired verification link", http.StatusBadRequest)
		return
	}
	_, err = u.pool.Exec(r.Context(),
		`UPDATE users SET email_verified_at = NOW() WHERE id = $1`, rec.UserID)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "email verified"}, http.StatusOK)
}

// POST /auth/resend-verification  body: {"email":"user@x.com"}
func (u *UserHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid body", http.StatusBadRequest)
		return
	}
	// Always return 200 — never reveal whether the email exists
	go func() {
		ctx := context.Background()
		user, err := u.userService.GetUserByEmail(ctx, req.Email)
		if err != nil || user.EmailVerified {
			return
		}
		raw, err := u.tokenRepo.Create(ctx, user.IDInt(), "email_verification", 24*time.Hour)
		if err != nil {
			slog.Warn("resend verification: failed to create token", "err", err)
			return
		}
		verifyURL := u.baseURL + "/verify-email?token=" + raw
		if err := u.mailer.SendVerificationEmail(user.Email, user.Name, verifyURL); err != nil {
			slog.Warn("resend verification: failed to send email", "err", err)
		}
	}()
	utils.RespondJSON(w, utils.APIResponse{
		Success: true,
		Data:    "if that email exists and is unverified, a new link was sent",
	}, http.StatusOK)
}

// POST /auth/forgot-password  body: {"email":"user@x.com"}
// Always returns 200 to prevent email enumeration.
func (u *UserHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid body", http.StatusBadRequest)
		return
	}
	go func() {
		ctx := context.Background()
		user, err := u.userService.GetUserByEmail(ctx, req.Email)
		if err != nil {
			return
		}
		raw, err := u.tokenRepo.Create(ctx, user.IDInt(), "password_reset", 1*time.Hour)
		if err != nil {
			slog.Warn("forgot-password: failed to create token", "err", err)
			return
		}
		resetURL := u.baseURL + "/reset-password?token=" + raw
		if err := u.mailer.SendPasswordResetEmail(user.Email, user.Name, resetURL); err != nil {
			slog.Warn("forgot-password: failed to send email", "err", err)
		}
	}()
	utils.RespondJSON(w, utils.APIResponse{
		Success: true,
		Data:    "if that email exists, a reset link was sent",
	}, http.StatusOK)
}

// POST /auth/reset-password  body: {"token":"<raw>","new_password":"<pass>"}
func (u *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Token       string `json:"token"        validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=8"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := utils.Validator().Struct(req); err != nil {
		utils.RespondError(w, "validation error: "+err.Error(), http.StatusBadRequest)
		return
	}
	rec, err := u.tokenRepo.Consume(r.Context(), req.Token, "password_reset")
	if err != nil {
		if err == pgx.ErrNoRows {
			utils.RespondError(w, "invalid or expired reset link", http.StatusBadRequest)
		} else {
			utils.RespondError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, err = u.pool.Exec(r.Context(),
		`UPDATE users SET password_hash = $1 WHERE id = $2`, hash, rec.UserID)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "password updated"}, http.StatusOK)
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
