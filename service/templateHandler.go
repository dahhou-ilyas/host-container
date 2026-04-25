package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dahhou-ilyas/host-container/repository"
	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TemplateHandler struct {
	pool *pgxpool.Pool
}

func NewTemplateHandler(pool *pgxpool.Pool) *TemplateHandler {
	return &TemplateHandler{pool: pool}
}

type portMapping struct {
	Internal int    `json:"internal"`
	External int    `json:"external"`
	Protocol string `json:"protocol"`
}

type templateResponse struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Image        string        `json:"image"`
	Category     string        `json:"category"`
	Tags         []string      `json:"tags"`
	Popular      bool          `json:"popular"`
	Icon         string        `json:"icon,omitempty"`
	DefaultPorts []portMapping `json:"defaultPorts"`
}

func toResponse(t repository.TemplateRow) (templateResponse, error) {
	var tags []string
	if err := json.Unmarshal(t.Tags, &tags); err != nil {
		tags = []string{}
	}

	var ports []portMapping
	if err := json.Unmarshal(t.DefaultPorts, &ports); err != nil {
		ports = []portMapping{}
	}

	return templateResponse{
		ID:           strconv.FormatInt(t.ID, 10),
		Name:         t.Name,
		Description:  t.Description,
		Image:        t.Image,
		Category:     t.Category,
		Tags:         tags,
		Popular:      t.Popular,
		Icon:         t.Icon,
		DefaultPorts: ports,
	}, nil
}

// GET /templates?category=web
func (th *TemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := r.URL.Query().Get("category")
	rows, err := repository.GetTemplates(r.Context(), th.pool, category)
	if err != nil {
		utils.RespondError(w, fmt.Sprintf("failed to fetch templates: %v", err), http.StatusInternalServerError)
		return
	}

	result := make([]templateResponse, 0, len(rows))
	for _, row := range rows {
		resp, err := toResponse(row)
		if err != nil {
			continue
		}
		result = append(result, resp)
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: result}, http.StatusOK)
}

// GET /templates/{id}
func (th *TemplateHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.RespondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := mux.Vars(r)["id"]
	if id == "" {
		utils.RespondError(w, "id is required", http.StatusBadRequest)
		return
	}

	row, err := repository.GetTemplateByID(r.Context(), th.pool, id)
	if err != nil {
		utils.RespondError(w, "template not found", http.StatusNotFound)
		return
	}

	resp, err := toResponse(*row)
	if err != nil {
		utils.RespondError(w, "failed to serialize template", http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, utils.APIResponse{Success: true, Data: resp}, http.StatusOK)
}
