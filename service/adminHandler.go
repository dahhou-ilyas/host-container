package service

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminHandler struct {
	pool *pgxpool.Pool
}

func NewAdminHandler(pool *pgxpool.Pool) *AdminHandler {
	return &AdminHandler{pool: pool}
}

type AdminStats struct {
	TotalUsers        int `json:"total_users"`
	TotalContainers   int `json:"total_containers"`
	RunningContainers int `json:"running_containers"`
	ContainersToday   int `json:"containers_today"`
	FreeUsers         int `json:"free_users"`
	ProUsers          int `json:"pro_users"`
	TeamUsers         int `json:"team_users"`
}

// GET /admin/stats
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	var s AdminStats
	h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM users`).Scan(&s.TotalUsers)
	h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM containers`).Scan(&s.TotalContainers)
	h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM containers WHERE status='running'`).Scan(&s.RunningContainers)
	h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM containers WHERE created_at >= CURRENT_DATE`).Scan(&s.ContainersToday)
	h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM users WHERE plan_id='free'`).Scan(&s.FreeUsers)
	h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM users WHERE plan_id='pro'`).Scan(&s.ProUsers)
	h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM users WHERE plan_id='team'`).Scan(&s.TeamUsers)
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: s}, http.StatusOK)
}

type AdminUser struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	PlanID         string `json:"plan_id"`
	ContainerCount int    `json:"container_count"`
	CreatedAt      string `json:"created_at"`
}

// GET /admin/users?page=1&search=...
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	search := "%" + strings.ToLower(r.URL.Query().Get("search")) + "%"
	offset := (page - 1) * 20

	rows, err := h.pool.Query(r.Context(), `
		SELECT u.id::text, u.name, u.email, u.role, u.plan_id,
		       COUNT(c.id)::int AS container_count,
		       u.created_at::text
		FROM users u
		LEFT JOIN containers c ON c.user_id = u.id
		WHERE LOWER(u.email) LIKE $1 OR LOWER(u.name) LIKE $1
		GROUP BY u.id
		ORDER BY u.id DESC
		LIMIT 20 OFFSET $2
	`, search, offset)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.PlanID, &u.ContainerCount, &u.CreatedAt)
		users = append(users, u)
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: users}, http.StatusOK)
}

// DELETE /admin/users/{id}
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := mux.Vars(r)["id"]

	tag, err := h.pool.Exec(r.Context(), `DELETE FROM users WHERE id=$1::bigint`, userID)
	if err != nil || tag.RowsAffected() == 0 {
		utils.RespondError(w, "user not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PATCH /admin/users/{id}/plan  body: {"plan_id":"pro"}
func (h *AdminHandler) UpdateUserPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := mux.Vars(r)["id"]
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.PlanID != "free" && req.PlanID != "pro" && req.PlanID != "team" {
		utils.RespondError(w, "invalid plan_id", http.StatusBadRequest)
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`UPDATE users SET plan_id=$1 WHERE id=$2::bigint`, req.PlanID, userID)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true}, http.StatusOK)
}
