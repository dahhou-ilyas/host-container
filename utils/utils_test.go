package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHashPassword_ProducesBcryptHash(t *testing.T) {
	hash, err := HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("HashPassword returned empty hash")
	}
}

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	hash, _ := HashPassword("correct-password")
	if err := VerifyPassword(hash, "correct-password"); err != nil {
		t.Errorf("VerifyPassword failed for correct password: %v", err)
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	hash, _ := HashPassword("correct-password")
	if err := VerifyPassword(hash, "wrong-password"); err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

func TestRespondJSON_WritesCorrectJSON(t *testing.T) {
	w := httptest.NewRecorder()
	payload := APIResponse{Success: true, Data: "hello"}
	RespondJSON(w, payload, http.StatusOK)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var got APIResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !got.Success {
		t.Error("expected success=true")
	}
}

func TestRespondError_WritesErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	RespondError(w, "something went wrong", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var got APIResponse
	json.NewDecoder(w.Body).Decode(&got)
	if got.Success {
		t.Error("expected success=false")
	}
	if got.Error != "something went wrong" {
		t.Errorf("unexpected error message: %q", got.Error)
	}
}
