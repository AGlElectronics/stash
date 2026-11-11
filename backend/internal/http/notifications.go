package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

// notificationsHandler serves GET /api/notifications: what the signed-in
// person should look at now. Today that is overdue sign-outs, for people
// with signout.view.
func notificationsHandler(overdue *service.OverdueService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list := []service.OverdueSignout{}
		if auth.HasPermission(r.Context(), domain.PermSignoutView) {
			var err error
			if list, err = overdue.List(r.Context(), time.Now()); err != nil {
				httperror.Render(w, r, err)
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"overdue_signouts": list})
	}
}

type telegramHandler struct {
	bot *service.LostFoundTelegram
}

func (h *telegramHandler) status(w http.ResponseWriter, r *http.Request) {
	st, err := h.bot.Status(r.Context())
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type notifyReq struct {
	Message string `json:"message"`
}

func (h *telegramHandler) notify(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	var body notifyReq
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &body); err != nil {
			httperror.Render(w, r, err)
			return
		}
	}
	if err := h.bot.Notify(r.Context(), id, body.Message); err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

type usersHandler struct {
	svc *service.UserService
}

func (h *usersHandler) routes(r chi.Router) {
	r.Use(require(domain.PermUserManage))
	r.Get("/", h.list)
	r.Post("/{id}/anonymise", h.anonymise)
}

func (h *usersHandler) list(w http.ResponseWriter, r *http.Request) {
	lim, off := clampPage(queryInt(r, "limit", 100), queryInt(r, "offset", 0), 500)
	items, total, err := h.svc.List(r.Context(), lim, off)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *usersHandler) anonymise(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	if u, ok := auth.FromContext(r.Context()); ok && u.ID == id {
		httperror.Render(w, r, httperror.Conflict("you cannot anonymise yourself"))
		return
	}
	if err := h.svc.Anonymise(r.Context(), id, "anonymised by an admin"); err != nil {
		httperror.Render(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
