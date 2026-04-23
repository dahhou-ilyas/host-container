package jwtSerivce

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	secretOnce sync.Once
	secretKey  []byte
)

func loadSecret() []byte {
	secretOnce.Do(func() {
		key := os.Getenv("JWT_SECRET_KEY")
		if key == "" {
			log.Fatal("JWT_SECRET_KEY environment variable must be set")
		}
		if len(key) < 32 {
			log.Fatal("JWT_SECRET_KEY must be at least 32 characters")
		}
		secretKey = []byte(key)
		slog.Info("JWT secret loaded", "length", len(key))
	})
	return secretKey
}

// CreateToken creates a short-lived access token (15 minutes).
func CreateToken(username, email, userId string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"email":    email,
		"id":       userId,
		"type":     "access",
		"exp":      time.Now().Add(15 * time.Minute).Unix(),
	})
	return token.SignedString(loadSecret())
}

// CreateRefreshToken creates a long-lived refresh token (7 days).
func CreateRefreshToken(userId string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   userId,
		"type": "refresh",
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
	})
	return token.SignedString(loadSecret())
}

// VerifyRefreshToken validates a refresh token and returns the user ID.
func VerifyRefreshToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return loadSecret(), nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}
	if claims["type"] != "refresh" {
		return "", fmt.Errorf("not a refresh token")
	}
	userId, ok := claims["id"].(string)
	if !ok {
		return "", fmt.Errorf("user id not found in token")
	}
	return userId, nil
}

func VerifyToken(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return loadSecret(), nil
	})
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("invalid token")
	}
	return nil
}

func GetUserIDFromToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return loadSecret(), nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}
	userId, ok := claims["id"].(string)
	if !ok {
		return "", fmt.Errorf("user id not found in token")
	}
	return userId, nil
}
