package http

import (
	"net/http"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

type departmentHandler struct {
	svc *service.DepartmentService
}

type departmentDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toDepartmentDTO(r sqlcgen.Department) departmentDTO {
	return departmentDTO{
		ID:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   timestampString(r.CreatedAt),
		UpdatedAt:   timestampString(r.UpdatedAt),
	}
}

func (h *departmentHandler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]departmentDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDepartmentDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *departmentHandler) get(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, toDepartmentDTO(row))
}

type createDepartmentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *departmentHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createDepartmentRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Create(r.Context(), service.CreateDepartmentInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDepartmentDTO(row))
}

type updateDepartmentRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (h *departmentHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req updateDepartmentRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Update(r.Context(), id, service.UpdateDepartmentInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDepartmentDTO(row))
}

func (h *departmentHandler) delete(w http.ResponseWriter, r *http.Request) {
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
