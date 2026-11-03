package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

type lostFoundHandler struct {
	svc *service.LostFoundService
}

type lostFoundDTO struct {
	ID               string  `json:"id"`
	ReportNumber     string  `json:"report_number"`
	Type             string  `json:"type"`
	Status           string  `json:"status"`
	ItemName         string  `json:"item_name"`
	Description      string  `json:"description"`
	PhotoFileID      *string `json:"photo_file_id,omitempty"`
	Location         string  `json:"location"`
	DateReported     string  `json:"date_reported"`
	ReportedByUserID *string `json:"reported_by_user_id,omitempty"`
	ContactName      string  `json:"contact_name"`
	ContactInfo      string  `json:"contact_info"`
	ReturnedTo       string  `json:"returned_to"`
	ResolvedByUserID *string `json:"resolved_by_user_id,omitempty"`
	ResolvedAt       string  `json:"resolved_at,omitempty"`
	ResolutionNote   string  `json:"resolution_note"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func toLostFoundDTO(r sqlcgen.LostFoundReport) lostFoundDTO {
	return lostFoundDTO{
		ID:               r.ID.String(),
		ReportNumber:     r.ReportNumber,
		Type:             r.Type,
		Status:           r.Status,
		ItemName:         r.ItemName,
		Description:      r.Description,
		PhotoFileID:      uuidToStrPtr(r.PhotoFileID),
		Location:         r.Location,
		DateReported:     timestampString(r.DateReported),
		ReportedByUserID: uuidToStrPtr(r.ReportedByUserID),
		ContactName:      r.ContactName,
		ContactInfo:      r.ContactInfo,
		ReturnedTo:       r.ReturnedTo,
		ResolvedByUserID: uuidToStrPtr(r.ResolvedByUserID),
		ResolvedAt:       timestampString(r.ResolvedAt),
		ResolutionNote:   r.ResolutionNote,
		CreatedAt:        timestampString(r.CreatedAt),
		UpdatedAt:        timestampString(r.UpdatedAt),
	}
}

// routes: reads need lostfound.view, creating needs lostfound.create; status
// transitions inside PATCH additionally require lostfound.resolve (enforced
// in the service where the before/after diff is known).
func (h *lostFoundHandler) routes(r chi.Router) {
	r.Group(func(g chi.Router) {
		g.Use(requireAny(domain.PermLostfoundView))
		g.Get("/", h.list)
		g.Get("/by-number/{reportNumber}", h.getByReportNumber)
		g.Get("/{id}", h.get)
		g.Patch("/{id}", h.update)
	})
	r.Group(func(g chi.Router) {
		g.Use(requireAny(domain.PermLostfoundCreate))
		g.Post("/", h.create)
	})
	r.Group(func(g chi.Router) {
		g.Use(requireAny(domain.PermLostfoundDelete))
		g.Delete("/{id}", h.delete)
	})
}

func (h *lostFoundHandler) list(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.List(r.Context(), service.ListLostFoundInput{
		Type:   queryString(r, "type"),
		Status: queryString(r, "status"),
		Search: queryString(r, "q"),
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]lostFoundDTO, 0, len(res.Items))
	for _, it := range res.Items {
		out = append(out, toLostFoundDTO(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
		"total": res.Total,
	})
}

func (h *lostFoundHandler) get(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, toLostFoundDTO(row))
}

func (h *lostFoundHandler) getByReportNumber(w http.ResponseWriter, r *http.Request) {
	reportNumber := chi.URLParam(r, "reportNumber")
	if reportNumber == "" {
		httperror.Render(w, r, httperror.BadRequest("report number is required"))
		return
	}
	row, err := h.svc.GetByReportNumber(r.Context(), reportNumber)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLostFoundDTO(row))
}

type createLostFoundReq struct {
	Type         string  `json:"type"`
	ItemName     string  `json:"item_name"`
	Description  string  `json:"description"`
	PhotoFileID  *string `json:"photo_file_id,omitempty"`
	Location     string  `json:"location"`
	DateReported *string `json:"date_reported,omitempty"`
	ContactName  string  `json:"contact_name"`
	ContactInfo  string  `json:"contact_info"`
}

func (h *lostFoundHandler) create(w http.ResponseWriter, r *http.Request) {
	var body createLostFoundReq
	if err := decodeJSON(r, &body); err != nil {
		httperror.Render(w, r, err)
		return
	}
	in := service.CreateLostFoundInput{
		Type:        body.Type,
		ItemName:    body.ItemName,
		Description: body.Description,
		Location:    body.Location,
		ContactName: body.ContactName,
		ContactInfo: body.ContactInfo,
	}
	if body.PhotoFileID != nil && *body.PhotoFileID != "" {
		id, err := uuid.Parse(*body.PhotoFileID)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid photo_file_id", nil))
			return
		}
		in.PhotoFileID = &id
	}
	if body.DateReported != nil && *body.DateReported != "" {
		t, err := time.Parse(time.RFC3339, *body.DateReported)
		if err != nil {
			httperror.Render(w, r, httperror.Validation("invalid date_reported (use RFC3339)", nil))
			return
		}
		in.DateReported = &t
	}

	row, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLostFoundDTO(row))
}

func (h *lostFoundHandler) delete(w http.ResponseWriter, r *http.Request) {
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

type updateLostFoundReq struct {
	Type        *string `json:"type,omitempty"`
	Status      *string `json:"status,omitempty"`
	ItemName    *string `json:"item_name,omitempty"`
	Description *string `json:"description,omitempty"`
	// PhotoFileID: omit = unchanged, "" = remove photo, uuid = set photo.
	PhotoFileID    *string `json:"photo_file_id,omitempty"`
	Location       *string `json:"location,omitempty"`
	ContactName    *string `json:"contact_name,omitempty"`
	ContactInfo    *string `json:"contact_info,omitempty"`
	ReturnedTo     *string `json:"returned_to,omitempty"`
	ResolutionNote *string `json:"resolution_note,omitempty"`
}

func (h *lostFoundHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var body updateLostFoundReq
	if err := decodeJSON(r, &body); err != nil {
		httperror.Render(w, r, err)
		return
	}
	in := service.UpdateLostFoundInput{
		Type:           body.Type,
		Status:         body.Status,
		ItemName:       body.ItemName,
		Description:    body.Description,
		Location:       body.Location,
		ContactName:    body.ContactName,
		ContactInfo:    body.ContactInfo,
		ReturnedTo:     body.ReturnedTo,
		ResolutionNote: body.ResolutionNote,
	}
	if body.PhotoFileID != nil {
		if *body.PhotoFileID == "" {
			in.ClearPhoto = true
		} else {
			fid, err := uuid.Parse(*body.PhotoFileID)
			if err != nil {
				httperror.Render(w, r, httperror.Validation("invalid photo_file_id", nil))
				return
			}
			in.PhotoFileID = &fid
		}
	}

	row, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLostFoundDTO(row))
}
