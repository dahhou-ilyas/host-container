package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/dahhou-ilyas/host-container/repository"
	"github.com/dahhou-ilyas/host-container/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Project struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	UserId string `json:"userId"`
}

type CreateContainerRequest struct {
	Project Project `json:"Project"`
	Image   string  `json:"image,omitempty"`
	Port    string  `json:"port,omitempty"`
}

type ExecRequest struct {
	Commande []string `json:"commande"`
}



type Handler struct {
	manager  *ContainerManager
	pool     *pgxpool.Pool
	recovery *RecoveryService
}

func NewHandler(basePath string, pool *pgxpool.Pool, autoStopTimeout time.Duration) (*Handler, error) {
	manager, err := NewContainerManager(basePath, pool, autoStopTimeout)
	if err != nil {
		return nil, err
	}
	return &Handler{manager: manager, pool: pool}, nil
}

// SetRecovery injects the RecoveryService after construction (avoids circular init).
func (h *Handler) SetRecovery(r *RecoveryService) {
	h.recovery = r
}

func (h *Handler) Manager() *ContainerManager {
	return h.manager
}

func (h *Handler) StartAutoStopWatcher() {
	h.manager.StartAutoStopWatcher()
}

func (h *Handler) StopAutoStopWatcher() {
	h.manager.StopAutoStopWatcher()
}

func (h *Handler) CreateContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Project.Name == "" {
		utils.RespondError(w, "project name is required", http.StatusBadRequest)
		return
	}

	if req.Project.UserId == "" {
		utils.RespondError(w, "user Id is required", http.StatusBadRequest)
		return
	}

	if req.Project.ID == "" {
		req.Project.ID = uuid.New().String()
	}

	if req.Image == "" {
		req.Image = "alpine:latest"
	}

	// Validate image against the predefined templates whitelist
	if allowed, err := repository.GetAllowedImages(r.Context(), h.pool); err == nil && len(allowed) > 0 {
		found := false
		for _, img := range allowed {
			if img == req.Image {
				found = true
				break
			}
		}
		if !found {
			utils.RespondError(w, "image not allowed — choose a predefined template", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	//user, ok := Users[req.Project.UserId]
	//
	//if !ok {
	//	h.respondError(w, "user Not found", http.StatusMethodNotAllowed)
	//	return
	//}
	//

	var info *ContainerInfo
	var err error

	if req.Port != "" {
		info, err = h.manager.CreateContainerWithPort(ctx, req.Project, req.Image, req.Port)
	} else {
		info, err = h.manager.CreateContainer(ctx, req.Project, req.Image)
	}

	if err != nil {
		log.Printf("Failed to create container: %v", err)
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: info}, http.StatusCreated)
}

func (h *Handler) GetContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	info, err := h.manager.GetContainerInfo(r.Context(),projectID)
	if err != nil {
		utils.RespondError(w, err.Error(), http.StatusNotFound)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: info}, http.StatusOK)
}



func (h *Handler) StartContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.StartContainer(ctx, projectID); err != nil {
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "container started"}, http.StatusOK)
}

func (h *Handler) StopContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.StopContainer(ctx, projectID); err != nil {
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "container stopped"}, http.StatusOK)
}

func (h *Handler) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	// Option pour supprimer le dossier aussi
	removeFolder := r.URL.Query().Get("remove_folder") == "true"

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.RemoveContainer(ctx, projectID, removeFolder); err != nil {
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "container removed"}, http.StatusOK)
}

func (h *Handler) ExecCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Commande) == 0 {
		utils.RespondError(w, "command is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	stdout, stderr, err := h.manager.ExecCommand(ctx, projectID, req.Commande)

	if err != nil {
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stderr != "" {
    	log.Printf("Commande a produit une erreur: %s", stderr)
		utils.RespondJSON(w, utils.APIResponse{Success: true, Data: map[string]string{"output": stderr}}, http.StatusOK)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: map[string]string{"output": stdout}}, http.StatusOK)
}


func (h *Handler) TreeFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	stdout, _, err := h.manager.ExecCommand(ctx, projectID, []string{"sh", "-c", "find /workspace -maxdepth 10 2>/dev/null | sort"})

	if err != nil {
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	node := utils.ParserTreeFolder(stdout)


	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: node}, http.StatusOK)
}

type ReadFileRequest struct {
	Path string `json:"path"`
}

type WriteFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (h *Handler) ReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		utils.RespondError(w, "path is required", http.StatusBadRequest)
		return
	}

	// Prevent path traversal: path must be within /workspace
	cleanPath := filepath.Clean(filePath)
	if !strings.HasPrefix(cleanPath, "/workspace") {
		utils.RespondError(w, "access denied: path must be within /workspace", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	stdout, stderr, err := h.manager.ExecCommand(ctx, projectID, []string{"cat", cleanPath})
	if err != nil {
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stderr != "" {
		utils.RespondError(w, stderr, http.StatusNotFound)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: map[string]string{"content": stdout}}, http.StatusOK)
}

func (h *Handler) WriteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	var req WriteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		utils.RespondError(w, "path is required", http.StatusBadRequest)
		return
	}

	// Prevent path traversal: path must be within /workspace
	cleanPath := filepath.Clean(req.Path)
	if !strings.HasPrefix(cleanPath, "/workspace") {
		utils.RespondError(w, "access denied: path must be within /workspace", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Use Docker CopyToContainer (tar stream) to avoid shell command injection
	if err := h.manager.WriteFileToContainer(ctx, projectID, cleanPath, req.Content); err != nil {
		utils.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: "file saved"}, http.StatusOK)
}

// POST /containers/file/upload?project_id=<id>
// multipart/form-data: field "path" = target folder inside container, field "file" = the file
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id is required", http.StatusBadRequest)
		return
	}
	// 10 MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondError(w, "file too large (max 10 MB)", http.StatusRequestEntityTooLarge)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		utils.RespondError(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	targetFolder := r.FormValue("path")
	if targetFolder == "" {
		targetFolder = "/workspace"
	}
	cleanFolder := filepath.Clean(targetFolder)
	if !strings.HasPrefix(cleanFolder, "/workspace") {
		utils.RespondError(w, "access denied: path must be within /workspace", http.StatusForbidden)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		utils.RespondError(w, "failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	destPath := filepath.Join(cleanFolder, filepath.Base(header.Filename))

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.WriteFileToContainer(ctx, projectID, destPath, string(data)); err != nil {
		utils.RespondError(w, "failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: map[string]string{"path": destPath}}, http.StatusOK)
}

// GET /containers/file/download?project_id=<id>&path=<path>
func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projectID := r.URL.Query().Get("project_id")
	filePath := r.URL.Query().Get("path")
	if projectID == "" || filePath == "" {
		utils.RespondError(w, "project_id and path are required", http.StatusBadRequest)
		return
	}
	cleanPath := filepath.Clean(filePath)
	if !strings.HasPrefix(cleanPath, "/workspace") {
		utils.RespondError(w, "access denied: path must be within /workspace", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	stdout, stderr, err := h.manager.ExecCommand(ctx, projectID, []string{"cat", cleanPath})
	if err != nil || stderr != "" {
		utils.RespondError(w, "file not found", http.StatusNotFound)
		return
	}

	filename := filepath.Base(cleanPath)
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(stdout))
}

func (h *Handler) Close() error {
	return h.manager.Close()
}

// GET /containers/health?project_id=<id>
// Returns restart_count, health_status, oom_killed, circuit_open for the authenticated user's container.
func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id required", http.StatusBadRequest)
		return
	}

	var health struct {
		RestartCount  int     `json:"restart_count"`
		LastRestartAt *string `json:"last_restart_at"`
		HealthStatus  string  `json:"health_status"`
		OOMKilled     bool    `json:"oom_killed"`
		CircuitOpen   bool    `json:"circuit_open"`
		MaxRestarts   int     `json:"max_auto_restarts"`
	}
	err := h.pool.QueryRow(r.Context(), `
		SELECT c.restart_count, c.last_restart_at::text, c.health_status,
		       c.oom_killed, c.circuit_open, p.max_auto_restarts
		FROM containers c
		JOIN users u ON u.id = c.user_id
		JOIN plans p ON p.id = u.plan_id
		WHERE c.id=$1::bigint AND c.user_id=$2::bigint`,
		projectID, userID).Scan(
		&health.RestartCount, &health.LastRestartAt, &health.HealthStatus,
		&health.OOMKilled, &health.CircuitOpen, &health.MaxRestarts)
	if err != nil {
		utils.RespondError(w, "container not found", http.StatusNotFound)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: health}, http.StatusOK)
}

// POST /containers/health/reset?project_id=<id>
// Clears circuit_open, restart_count and oom_killed so auto-recovery can resume.
func (h *Handler) ResetCircuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.recovery == nil {
		utils.RespondError(w, "recovery service unavailable", http.StatusInternalServerError)
		return
	}
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		utils.RespondError(w, "project_id required", http.StatusBadRequest)
		return
	}
	if err := h.recovery.ResetCircuit(r.Context(), userID, projectID); err != nil {
		utils.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true}, http.StatusOK)
}
