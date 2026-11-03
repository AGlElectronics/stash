package http

import (
	"net/http"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

type categoryHandler struct {
	svc *service.CategoryService
}

type categoryDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toCategoryDTO(r sqlcgen.ItemCategory) categoryDTO {
	return categoryDTO{
		ID:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   timestampString(r.CreatedAt),
		UpdatedAt:   timestampString(r.UpdatedAt),
	}
}

func (h *categoryHandler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]categoryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCategoryDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *categoryHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryDTO(row))
}

type categoryCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *categoryHandler) create(w http.ResponseWriter, r *http.Request) {
	var req categoryCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Create(r.Context(), service.CreateCategoryInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCategoryDTO(row))
}

type categoryUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (h *categoryHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req categoryUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Update(r.Context(), id, service.UpdateCategoryInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toCategoryDTO(row))
}

func (h *categoryHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		httperror.Render(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
