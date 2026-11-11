package http

import (
	"errors"
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

// patch validates and merges a partial JSON object into system settings.
// Gated by access.manage at the router level.
func (h *settingsHandler) patch(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httperror.Render(w, r, httperror.BadRequest("settings patch is larger than 64 KiB"))
			return
		}
		httperror.Render(w, r, httperror.BadRequest("could not read body"))
		return
	}
	out, err := h.svc.PatchJSON(r.Context(), raw)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
