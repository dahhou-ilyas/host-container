package middlware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dahhou-ilyas/host-container/utils"
	jwtService "github.com/dahhou-ilyas/host-container/jwt"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET_KEY", "test-secret-key-at-least-32-chars-long!!")
	os.Exit(m.Run())
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	w.Header().Set("X-User-ID", userID)
	w.WriteHeader(http.StatusOK)
}

func TestAuthMiddleware_NoToken_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	AuthMiddleware(okHandler)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	w := httptest.NewRecorder()
	AuthMiddleware(okHandler)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken_PassesUserID(t *testing.T) {
	token, err := jwtService.CreateToken("alice", "alice@example.com", "42")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	AuthMiddleware(okHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if uid := w.Header().Get("X-User-ID"); uid != "42" {
		t.Errorf("expected X-User-ID=42, got %q", uid)
	}
}

func TestAuthMiddleware_TokenViaQueryParam(t *testing.T) {
	token, _ := jwtService.CreateToken("bob", "bob@example.com", "7")
	req := httptest.NewRequest(http.MethodGet, "/test?token="+token, nil)
	w := httptest.NewRecorder()
	AuthMiddleware(okHandler)(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 via query param token, got %d", w.Code)
	}
}
