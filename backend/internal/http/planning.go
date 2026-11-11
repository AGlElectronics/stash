package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

// --- purchases (under /api/items/{id}/purchases) ---------------------------

type purchaseHandler struct{ svc *service.PurchaseService }

func (h *purchaseHandler) list(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.List(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *purchaseHandler) create(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var in service.PurchaseInput
	if err := decodeJSON(r, &in); err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Record(r.Context(), id, in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// --- requests (/api/requests) ----------------------------------------------

type requestHandler struct{ svc *service.RequestService }

func (h *requestHandler) routes(r chi.Router) {
	r.Use(requireAnyOf(domain.PermRequestCreate, domain.PermRequestManage))
	r.Get("/", h.list)
	r.With(require(domain.PermRequestCreate)).Post("/", h.create)
	r.Post("/{id}/decide", h.decide)
}

func (h *requestHandler) list(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.List(r.Context(), queryString(r, "status"))
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *requestHandler) create(w http.ResponseWriter, r *http.Request) {
	var in service.RequestInput
	if err := decodeJSON(r, &in); err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

type decideReq struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func (h *requestHandler) decide(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var in decideReq
	if err := decodeJSON(r, &in); err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Decide(r.Context(), id, in.Status, in.Note)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- packing lists (/api/packing-lists) --------------------------------------

type packingHandler struct{ svc *service.PackingService }

func (h *packingHandler) routes(r chi.Router) {
	r.Use(require(domain.PermSignoutView))
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Group(func(g chi.Router) {
		g.Use(require(domain.PermSignoutCreate))
		g.Post("/", h.create)
		g.Patch("/{id}", h.update)
		g.Delete("/{id}", h.delete)
		g.Put("/{id}/items", h.setItems)
		g.Post("/{id}/signout", h.signout)
	})
}

func (h *packingHandler) list(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.List(r.Context())
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *packingHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *packingHandler) create(w http.ResponseWriter, r *http.Request) {
	var in service.PackingListInput
	if err := decodeJSON(r, &in); err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *packingHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var in service.PackingListInput
	if err := decodeJSON(r, &in); err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *packingHandler) delete(w http.ResponseWriter, r *http.Request) {
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

func (h *packingHandler) setItems(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var in struct {
		Items []service.PackingLineInput `json:"items"`
	}
	if err := decodeJSON(r, &in); err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.SetItems(r.Context(), id, in.Items)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *packingHandler) signout(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var in struct {
		ToDepartmentID *uuid.UUID `json:"to_department_id,omitempty"`
		Purpose        string     `json:"purpose"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &in); err != nil {
			httperror.Render(w, r, err)
			return
		}
	}
	out, err := h.svc.CreateSignout(r.Context(), id, in.ToDepartmentID, in.Purpose)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSignoutWithItemsDTO(out))
}
