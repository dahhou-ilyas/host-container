package jwtSerivce

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestMain(m *testing.M) {
	// Set secret before any test calls loadSecret() for the first time
	os.Setenv("JWT_SECRET_KEY", "test-secret-key-at-least-32-chars-long!!")
	os.Exit(m.Run())
}

func TestCreateToken_ValidClaims(t *testing.T) {
	token, err := CreateToken("alice", "alice@example.com", "42")
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("CreateToken returned empty token")
	}
}

func TestVerifyToken_ValidToken(t *testing.T) {
	token, _ := CreateToken("bob", "bob@example.com", "7")
	if err := VerifyToken(token); err != nil {
		t.Errorf("VerifyToken failed for valid token: %v", err)
	}
}

func TestVerifyToken_InvalidSignature(t *testing.T) {
	if err := VerifyToken("invalid.token.value"); err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	key := loadSecret()
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  "1",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	tokenStr, _ := expired.SignedString(key)
	if err := VerifyToken(tokenStr); err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestGetUserIDFromToken(t *testing.T) {
	token, _ := CreateToken("carol", "carol@example.com", "99")
	id, err := GetUserIDFromToken(token)
	if err != nil {
		t.Fatalf("GetUserIDFromToken error: %v", err)
	}
	if id != "99" {
		t.Errorf("expected id=99, got %q", id)
	}
}

func TestGetUserIDFromToken_Invalid(t *testing.T) {
	_, err := GetUserIDFromToken("not-a-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}
