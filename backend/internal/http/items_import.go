package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// maxImportBody caps a CSV import upload.
const maxImportBody = 5 << 20

// importItems serves POST /api/items/import. The CSV comes as the request
// body (Content-Type text/csv) or as the "file" field of a multipart form.
// ?dry_run=true checks everything and saves nothing. A real import saves
// all rows or, when any row has a problem, none (422 with the report).
func (h *itemHandler) importItems(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBody)
	var src io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxImportBody); err != nil {
			renderImportReadError(w, r, err)
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()
		f, _, err := r.FormFile("file")
		if err != nil {
			httperror.Render(w, r, httperror.BadRequest("file field is required"))
			return
		}
		defer func() { _ = f.Close() }()
		src = f
	}
	data, err := io.ReadAll(src)
	if err != nil {
		renderImportReadError(w, r, err)
		return
	}
	res, err := h.svc.ImportCSV(r.Context(), strings.NewReader(string(data)), queryBool(r, "dry_run"))
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	if !res.DryRun && !res.Applied {
		httperror.Render(w, r, httperror.Validation(
			fmt.Sprintf("%d of %d rows have problems; nothing was saved", res.Failed, len(res.Rows)), res))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func renderImportReadError(w http.ResponseWriter, r *http.Request, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": map[string]string{
			"code": "payload_too_large", "message": "the CSV file is larger than 5 MiB; split it",
		}})
		return
	}
	httperror.Render(w, r, httperror.BadRequest("could not read the CSV file"))
}
