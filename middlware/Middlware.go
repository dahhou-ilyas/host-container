package middlware

import (
	"context"
	jwtSerivce "docker-wrapper/jwt"
	"docker-wrapper/utils"
	"net/http"
	"strings"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.RespondError(w, "authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.RespondError(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		err := jwtSerivce.VerifyToken(tokenString)

		if err != nil {
			utils.RespondError(w, "invalid Token", http.StatusUnauthorized)
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