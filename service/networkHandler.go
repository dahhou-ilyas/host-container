package service

import (
	"encoding/json"
	"net/http"

	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/gorilla/mux"
)

type NetworkHandler struct {
	nm *NetworkManager
}

func NewNetworkHandler(nm *NetworkManager) *NetworkHandler {
	return &NetworkHandler{nm: nm}
}

func (h *NetworkHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.List(w, r)
	case http.MethodPost:
		h.Create(w, r)
	default:
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *NetworkHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		utils.RespondError(w, "name is required", http.StatusBadRequest)
		return
	}
	net, err := h.nm.CreateNetwork(r.Context(), userID, req.Name)
	if err != nil {
		utils.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: net}, http.StatusCreated)
}

func (h *NetworkHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	nets, err := h.nm.ListNetworks(r.Context(), userID)
	if err != nil {
		utils.RespondError(w, "internal error", http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: nets}, http.StatusOK)
}

func (h *NetworkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	netID := mux.Vars(r)["id"]
	if err := h.nm.DeleteNetwork(r.Context(), userID, netID); err != nil {
		utils.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NetworkHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	netID := mux.Vars(r)["id"]
	var req struct {
		ProjectID string `json:"project_id"`
		Alias     string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProjectID == "" || req.Alias == "" {
		utils.RespondError(w, "project_id and alias are required", http.StatusBadRequest)
		return
	}
	if err := h.nm.ConnectContainer(r.Context(), userID, netID, req.ProjectID, req.Alias); err != nil {
		utils.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}
	utils.RespondJSON(w, utils.APIResponse{Success: true}, http.StatusOK)
}

func (h *NetworkHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _ := r.Context().Value(utils.UserIDKey).(string)
	vars := mux.Vars(r)
	if err := h.nm.DisconnectContainer(r.Context(), userID, vars["id"], vars["projectId"]); err != nil {
		utils.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
