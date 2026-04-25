package service

import (
	"context"
	"encoding/json"
	"log"
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
	manager *ContainerManager
	pool    *pgxpool.Pool
}

func NewHandler(basePath string, pool *pgxpool.Pool, autoStopTimeout time.Duration) (*Handler, error) {
	manager, err := NewContainerManager(basePath, pool, autoStopTimeout)
	if err != nil {
		return nil, err
	}
	return &Handler{manager: manager, pool: pool}, nil
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

func (h *Handler) Close() error {
	return h.manager.Close()
}
