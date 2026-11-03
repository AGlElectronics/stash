package http

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

type settingsHandler struct {
	svc *service.SettingsService
}

// get returns the current system settings. Authenticated read so the SPA
// can adjust UI behaviour (e.g. require barcode scan on sign-out).
func (h *settingsHandler) get(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Get(r.Context())
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// patch merges a partial JSON object into system settings. Gated by
// access.manage at the router level, so only admins can flip toggles.
func (h *settingsHandler) patch(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httperror.Render(w, r, httperror.BadRequest("could not read body"))
		return
	}
	if !json.Valid(raw) {
		httperror.Render(w, r, httperror.BadRequest("invalid JSON"))
		return
	}
	out, err := h.svc.PatchJSON(r.Context(), raw)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
