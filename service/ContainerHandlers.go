package service

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

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

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type Handler struct {
	manager *ContainerManager
}

func NewHandler(basePath string, pool *pgxpool.Pool) (*Handler, error) {
	manager, err := NewContainerManager(basePath,pool)
	if err != nil {
		return nil, err
	}
	return &Handler{manager: manager}, nil
}

func (h *Handler) CreateContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Project.Name == "" {
		h.respondError(w, "project name is required", http.StatusBadRequest)
		return
	}

	if req.Project.UserId == "" {
		h.respondError(w, "user Id is required", http.StatusBadRequest)
		return
	}

	if req.Project.ID == "" {
		req.Project.ID = uuid.New().String()
	}

	if req.Image == "" {
		req.Image = "alpine:latest"
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
		h.respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, APIResponse{Success: true, Data: info}, http.StatusCreated)
}

func (h *Handler) GetContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		h.respondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	info, err := h.manager.GetContainerInfo(r.Context(),projectID)
	if err != nil {
		h.respondError(w, err.Error(), http.StatusNotFound)
		return
	}

	h.respondJSON(w, APIResponse{Success: true, Data: info}, http.StatusOK)
}



func (h *Handler) StartContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		h.respondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.StartContainer(ctx, projectID); err != nil {
		h.respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, APIResponse{Success: true, Data: "container started"}, http.StatusOK)
}

func (h *Handler) StopContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		h.respondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.StopContainer(ctx, projectID); err != nil {
		h.respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, APIResponse{Success: true, Data: "container stopped"}, http.StatusOK)
}

func (h *Handler) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		h.respondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	// Option pour supprimer le dossier aussi
	removeFolder := r.URL.Query().Get("remove_folder") == "true"

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.manager.RemoveContainer(ctx, projectID, removeFolder); err != nil {
		h.respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, APIResponse{Success: true, Data: "container removed"}, http.StatusOK)
}

func (h *Handler) ExecCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		h.respondError(w, "project_id is required", http.StatusBadRequest)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Commande) == 0 {
		h.respondError(w, "command is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	output, err := h.manager.ExecCommand(ctx, projectID, req.Commande)
	log.Printf(output)
	if err != nil {
		h.respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, APIResponse{Success: true, Data: map[string]string{"output": output}}, http.StatusOK)
}

func (h *Handler) respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) respondError(w http.ResponseWriter, message string, status int) {
	h.respondJSON(w, APIResponse{Success: false, Error: message}, status)
}

func (h *Handler) Close() error {
	return h.manager.Close()
}
