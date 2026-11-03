package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

type locationHandler struct {
	svc *service.LocationService
}

type locationDTO struct {
	ID          string  `json:"id"`
	ParentID    *string `json:"parent_id,omitempty"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func toLocationDTO(r sqlcgen.Location) locationDTO {
	dto := locationDTO{
		ID:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   timestampString(r.CreatedAt),
		UpdatedAt:   timestampString(r.UpdatedAt),
	}
	if r.ParentID != nil {
		s := r.ParentID.String()
		dto.ParentID = &s
	}
	return dto
}

func (h *locationHandler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]locationDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toLocationDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *locationHandler) get(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, toLocationDTO(row))
}

type locationCreateRequest struct {
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
}

func (h *locationHandler) create(w http.ResponseWriter, r *http.Request) {
	var req locationCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Create(r.Context(), service.CreateLocationInput{
		ParentID:    req.ParentID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLocationDTO(row))
}

type locationUpdateRequest struct {
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	ClearParent bool       `json:"clear_parent,omitempty"`
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
}

func (h *locationHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var req locationUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Update(r.Context(), id, service.UpdateLocationInput{
		ParentID:    req.ParentID,
		ClearParent: req.ClearParent,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLocationDTO(row))
}

func (h *locationHandler) delete(w http.ResponseWriter, r *http.Request) {
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
