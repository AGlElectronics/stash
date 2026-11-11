package http

import (
	"encoding/json"
	"net/http"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

// maxSettingsBody bounds PATCH /api/settings.
const maxSettingsBody = 64 << 10

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

// patch validates and merges a partial JSON object into system settings.
// Gated by access.manage at the router level.
func (h *settingsHandler) patch(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := decodeJSONLimit(r, &raw, maxSettingsBody); err != nil {
		httperror.Render(w, r, err)
		return
	}
	out, err := h.svc.PatchJSON(r.Context(), raw)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
