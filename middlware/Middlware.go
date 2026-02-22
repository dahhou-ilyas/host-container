package middlware

import (
	"context"
	jwtSerivce "github.com/dahhou-ilyas/host-container/jwt"
	"github.com/dahhou-ilyas/host-container/utils"
	"net/http"
	"strings"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var tokenString string

        authHeader := r.Header.Get("Authorization")
        if authHeader != "" {
            parts := strings.Split(authHeader, " ")
            if len(parts) != 2 || parts[0] != "Bearer" {
                utils.RespondError(w, "invalid authorization header format", http.StatusUnauthorized)
                return
            }
            tokenString = parts[1]
        }

        if tokenString == "" {
            tokenString = r.URL.Query().Get("token")
        }

        if tokenString == "" {
            utils.RespondError(w, "authorization required", http.StatusUnauthorized)
            return
        }

        if err := jwtSerivce.VerifyToken(tokenString); err != nil {
            utils.RespondError(w, "invalid token", http.StatusUnauthorized)
            return
        }

        userId, err := jwtSerivce.GetUserIDFromToken(tokenString)
        if err != nil {
            utils.RespondError(w, "invalid or expired token", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), utils.UserIDKey, userId)
        next.ServeHTTP(w, r.WithContext(ctx))
    }
}





