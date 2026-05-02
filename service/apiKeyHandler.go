package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

const apiKeyPrefix = "dk_live_"
const maxApiKeysPerUser = 10

type ApiKeyHandler struct {
	pool *pgxpool.Pool
}

func NewApiKeyHandler(pool *pgxpool.Pool) *ApiKeyHandler {
	return &ApiKeyHandler{pool: pool}
}

// Dispatch routes GET and POST on /api-keys
func (h *ApiKeyHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.List(w, r)
	case http.MethodPost:
		h.Create(w, r)
	default:
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api-keys  body: {"name":"My CI Key"}
func (h *ApiKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(utils.UserIDKey).(string)

	var req struct {
		Name string `json:"name" validate:"required,min=1,max=50"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := utils.Validator().Struct(req); err != nil {
		utils.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var count int
	h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM api_keys WHERE user_id=$1::bigint`, userID).Scan(&count)
	if count >= maxApiKeysPerUser {
		utils.RespondError(w, "maximum API keys reached (10)", http.StatusBadRequest)
		return
	}

	rawBytes := make([]byte, 32)
	rand.Read(rawBytes)
	rawKey := apiKeyPrefix + hex.EncodeToString(rawBytes)

	h256 := sha256.Sum256([]byte(rawKey))
	hash := hex.EncodeToString(h256[:])

	// First 20 chars are safe to store as a recognizable prefix
	prefix := rawKey[:20]

	var keyID string
	var createdAt time.Time
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO api_keys (user_id, name, key_hash, key_prefix)
		 VALUES ($1::bigint, $2, $3, $4)
		 RETURNING id::text, created_at`,
		userID, req.Name, hash, prefix,
	).Scan(&keyID, &createdAt)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: map[string]any{
		"id":         keyID,
		"key":        rawKey,
		"name":       req.Name,
		"prefix":     prefix,
		"created_at": createdAt,
	}}, http.StatusCreated)
}

// GET /api-keys  — lists keys without raw values
func (h *ApiKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	rows, err := h.pool.Query(r.Context(),
		`SELECT id::text, name, key_prefix, last_used_at, created_at
		 FROM api_keys WHERE user_id=$1::bigint ORDER BY created_at DESC`, userID)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ApiKeyItem struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		Prefix     string  `json:"prefix"`
		LastUsedAt *string `json:"last_used_at"`
		CreatedAt  string  `json:"created_at"`
	}
	keys := make([]ApiKeyItem, 0)
	for rows.Next() {
		var k ApiKeyItem
		rows.Scan(&k.ID, &k.Name, &k.Prefix, &k.LastUsedAt, &k.CreatedAt)
		keys = append(keys, k)
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: keys}, http.StatusOK)
}

// DELETE /api-keys/{id}
func (h *ApiKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	keyID := mux.Vars(r)["id"]

	tag, err := h.pool.Exec(r.Context(),
		`DELETE FROM api_keys WHERE id=$1 AND user_id=$2::bigint`, keyID, userID)
	if err != nil || tag.RowsAffected() == 0 {
		utils.RespondError(w, "key not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
