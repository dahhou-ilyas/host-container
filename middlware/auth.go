package middlware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	jwtSerivce "github.com/dahhou-ilyas/host-container/jwt"
	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Auth is an injectable auth middleware that supports both JWT and API keys.
type Auth struct {
	pool *pgxpool.Pool
}

func NewAuth(pool *pgxpool.Pool) *Auth {
	return &Auth{pool: pool}
}

// Middleware accepts JWT (Bearer header or ?token= param) or X-API-Key header.
// X-API-Key is checked first; if absent, falls through to JWT.
func (a *Auth) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			userID, err := a.resolveApiKey(r.Context(), apiKey)
			if err != nil {
				utils.RespondError(w, "invalid API key", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), utils.UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		userID, ok := resolveJWT(w, r)
		if !ok {
			return
		}
		ctx := context.WithValue(r.Context(), utils.UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// AdminOnly wraps an already-authenticated handler and enforces role='admin'.
// Use as: auth.Middleware(auth.AdminOnly(handler))
func (a *Auth) AdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(utils.UserIDKey).(string)
		var role string
		err := a.pool.QueryRow(r.Context(),
			`SELECT role FROM users WHERE id=$1::bigint`, userID,
		).Scan(&role)
		if err != nil || role != "admin" {
			utils.RespondError(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (a *Auth) resolveApiKey(ctx context.Context, rawKey string) (string, error) {
	h := sha256.Sum256([]byte(rawKey))
	hash := hex.EncodeToString(h[:])
	var userID string
	err := a.pool.QueryRow(ctx,
		`UPDATE api_keys SET last_used_at=NOW()
		 WHERE key_hash=$1
		 RETURNING user_id::text`,
		hash,
	).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// resolveJWT extracts and validates a JWT from the request, writes an error
// response and returns false on failure.
func resolveJWT(w http.ResponseWriter, r *http.Request) (string, bool) {
	var token string
	if h := r.Header.Get("Authorization"); h != "" {
		parts := splitTwo(h, " ")
		if parts == nil || parts[0] != "Bearer" {
			utils.RespondError(w, "invalid authorization header format", http.StatusUnauthorized)
			return "", false
		}
		token = parts[1]
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		utils.RespondError(w, "authorization required", http.StatusUnauthorized)
		return "", false
	}
	if err := jwtSerivce.VerifyToken(token); err != nil {
		utils.RespondError(w, "invalid token", http.StatusUnauthorized)
		return "", false
	}
	userID, err := jwtSerivce.GetUserIDFromToken(token)
	if err != nil {
		utils.RespondError(w, "invalid or expired token", http.StatusUnauthorized)
		return "", false
	}
	return userID, true
}

func splitTwo(s, sep string) []string {
	idx := -1
	for i := 0; i < len(s)-len(sep)+1; i++ {
		if s[i:i+len(sep)] == sep {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	return []string{s[:idx], s[idx+len(sep):]}
}
