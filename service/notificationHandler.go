package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationHandler struct {
	pool *pgxpool.Pool
	bus  *NotificationBus
}

func NewNotificationHandler(pool *pgxpool.Pool, bus *NotificationBus) *NotificationHandler {
	return &NotificationHandler{pool: pool, bus: bus}
}

// GET /notifications/stream — long-lived SSE connection.
// Requires AuthMiddleware to have injected userID into context.
func (h *NotificationHandler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(utils.UserIDKey).(string)
	if !ok || userID == "" {
		utils.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		utils.RespondError(w, "streaming not supported by server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	ch, cancel := h.bus.Subscribe(userID)
	defer cancel()

	// Heartbeat every 25 s keeps the connection alive through proxies/load-balancers.
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case n := <-ch:
			data, err := json.Marshal(n)
			if err != nil {
				slog.Warn("failed to marshal notification", "err", err)
				continue
			}
			fmt.Fprintf(w, "event: notification\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

type notificationRow struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// GET /notifications — last 20 notifications for the authenticated user.
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := r.Context().Value(utils.UserIDKey).(string)
	if !ok || userID == "" {
		utils.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, type, title, body, metadata, read_at, created_at
		 FROM notifications
		 WHERE user_id = $1::bigint
		 ORDER BY created_at DESC
		 LIMIT 20`,
		userID,
	)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	notifications := make([]notificationRow, 0)
	for rows.Next() {
		var n notificationRow
		var metaRaw []byte
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &metaRaw, &n.ReadAt, &n.CreatedAt); err != nil {
			utils.RespondError(w, "internal error", http.StatusInternalServerError)
			return
		}
		if metaRaw != nil {
			_ = json.Unmarshal(metaRaw, &n.Metadata)
		}
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: notifications}, http.StatusOK)
}

// POST /notifications/read-all — marks all unread notifications as read.
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := r.Context().Value(utils.UserIDKey).(string)
	if !ok || userID == "" {
		utils.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_, _ = h.pool.Exec(r.Context(),
		`UPDATE notifications SET read_at = NOW()
		 WHERE user_id = $1::bigint AND read_at IS NULL`,
		userID,
	)
	utils.RespondJSON(w, utils.APIResponse{Success: true}, http.StatusOK)
}

// PersistAndPublish saves a notification to DB and pushes it live via SSE.
func (h *NotificationHandler) PersistAndPublish(userID string, n Notification) {
	metaJSON, _ := json.Marshal(n.Metadata)
	_, err := h.pool.Exec(
		context.Background(),
		`INSERT INTO notifications (user_id, type, title, body, metadata)
		 VALUES ($1::bigint, $2, $3, $4, $5)`,
		userID, n.Type, n.Title, n.Body, metaJSON,
	)
	if err != nil {
		slog.Warn("failed to persist notification", "userID", userID, "err", err)
	}
	h.bus.Publish(userID, n)
}
