package http

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/files"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

type fileHandler struct {
	svc *files.Service
}

type fileDTO struct {
	ID               string  `json:"id"`
	EntityType       string  `json:"entity_type"`
	EntityID         string  `json:"entity_id"`
	FileType         string  `json:"file_type"`
	StorageKey       string  `json:"storage_key"`
	OriginalFilename string  `json:"original_filename"`
	ContentType      string  `json:"content_type"`
	SizeBytes        int64   `json:"size_bytes"`
	UploadedBy       *string `json:"uploaded_by,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

func toFileDTO(f sqlcgen.File) fileDTO {
	return fileDTO{
		ID:               f.ID.String(),
		EntityType:       f.EntityType,
		EntityID:         f.EntityID.String(),
		FileType:         f.FileType,
		StorageKey:       f.StorageKey,
		OriginalFilename: f.OriginalFilename,
		ContentType:      f.ContentType,
		SizeBytes:        f.SizeBytes,
		UploadedBy:       uuidToStrPtr(f.UploadedBy),
		CreatedAt:        timestampString(f.CreatedAt),
	}
}

func (h *fileHandler) routes(r chi.Router) {
	upload := r.With(requireAny(domain.PermFileUpload))
	upload.Post("/presign-upload", h.presignUpload)
	upload.Post("/upload", h.upload)
	upload.Post("/", h.register)
	r.Get("/", h.list)
	r.With(h.onFile(false)).Get("/{id}/download", h.download)
	r.With(h.onFile(false)).Get("/{id}/raw", h.raw)
	r.With(requireAny(domain.PermFileUpload), h.onFile(true)).Delete("/{id}", h.delete)
}

// entityAccess checks the permissions a file inherits from what it is
// attached to: view access to read, create/edit access to change.
func entityAccess(ctx context.Context, entityType string, write bool) error {
	var perms []string
	switch entityType {
	case "item":
		perms = []string{domain.PermItemView}
		if write {
			perms = []string{domain.PermItemCreate, domain.PermItemEdit}
		}
	case "lostfound":
		perms = []string{domain.PermLostfoundView}
		if write {
			perms = []string{domain.PermLostfoundCreate, domain.PermLostfoundEdit}
		}
	default:
		return httperror.BadRequest("unsupported entity_type: " + entityType)
	}
	return auth.RequireAnyOf(ctx, perms...)
}

// onFile loads the {id} file and applies entityAccess for its owner.
func (h *fileHandler) onFile(write bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			if err := entityAccess(r.Context(), row.EntityType, write); err != nil {
				httperror.Render(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// raw streams the object bytes through the backend so the browser never
// has to hit the storage host directly. The response has Cache-Control:
// private so authenticated previews are not shared by intermediate caches.
func (h *fileHandler) raw(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, rc, err := h.svc.RawStream(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	defer func() { _ = rc.Close() }()

	if row.ContentType != "" {
		w.Header().Set("Content-Type", row.ContentType)
	}
	if row.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(row.SizeBytes, 10))
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	if row.OriginalFilename != "" {
		// inline so <img src> works; the filename is a hint only.
		w.Header().Set("Content-Disposition", `inline; filename="`+sanitizeFilename(row.OriginalFilename)+`"`)
	}
	if _, err := io.Copy(w, rc); err != nil {
		// connection closed mid-stream is normal; only log surprises
		_ = err
	}
}

// sanitizeFilename strips quotes so we can safely inline it in a header.
func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// upload accepts a multipart/form-data POST and streams the file to object
// storage via the backend, then registers the metadata. Form fields:
//   - file (required): the file part
//   - entity_type (required): "item", "signout", ...
//   - entity_id   (required): UUID
//   - file_type   (optional): defaults to "image"
//   - set_as_primary (optional): "true"/"1" to flag the resulting file as
//     the entity's primary image (only honored for entity_type=item +
//     file_type=image).
func (h *fileHandler) upload(w http.ResponseWriter, r *http.Request) {
	// 32 MiB in memory; anything larger spills to disk via mime/multipart.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httperror.Render(w, r, httperror.BadRequest("invalid multipart form: "+err.Error()))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httperror.Render(w, r, httperror.BadRequest("file field is required"))
		return
	}
	defer func() { _ = file.Close() }()

	entityType := strings.TrimSpace(r.FormValue("entity_type"))
	if entityType == "" {
		httperror.Render(w, r, httperror.BadRequest("entity_type is required"))
		return
	}
	entityID, parseErr := uuid.Parse(strings.TrimSpace(r.FormValue("entity_id")))
	if parseErr != nil {
		httperror.Render(w, r, httperror.BadRequest("entity_id must be a UUID"))
		return
	}
	if err := entityAccess(r.Context(), entityType, true); err != nil {
		httperror.Render(w, r, err)
		return
	}

	fileType := strings.TrimSpace(r.FormValue("file_type"))
	if fileType == "" {
		fileType = string(domain.FileTypeImage)
	}

	setPrimary := false
	switch strings.ToLower(strings.TrimSpace(r.FormValue("set_as_primary"))) {
	case "1", "true", "yes", "on":
		setPrimary = true
	}

	contentType := header.Header.Get("Content-Type")

	row, err := h.svc.Upload(r.Context(), files.UploadInput{
		EntityType:       entityType,
		EntityID:         entityID,
		FileType:         domain.FileType(fileType),
		OriginalFilename: header.Filename,
		ContentType:      contentType,
		SetAsPrimary:     setPrimary,
		Body:             file,
		Size:             header.Size,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toFileDTO(row))
}

type presignRequest struct {
	EntityType       string    `json:"entity_type"`
	EntityID         uuid.UUID `json:"entity_id"`
	FileType         string    `json:"file_type"`
	OriginalFilename string    `json:"original_filename"`
}

func (h *fileHandler) presignUpload(w http.ResponseWriter, r *http.Request) {
	var req presignRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	if err := entityAccess(r.Context(), req.EntityType, true); err != nil {
		httperror.Render(w, r, err)
		return
	}
	res, err := h.svc.PresignUpload(r.Context(), files.PresignUploadInput{
		EntityType:       req.EntityType,
		EntityID:         req.EntityID,
		FileType:         domain.FileType(req.FileType),
		OriginalFilename: req.OriginalFilename,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type registerRequest struct {
	EntityType       string    `json:"entity_type"`
	EntityID         uuid.UUID `json:"entity_id"`
	FileType         string    `json:"file_type"`
	StorageKey       string    `json:"storage_key"`
	OriginalFilename string    `json:"original_filename"`
	ContentType      string    `json:"content_type"`
	SizeBytes        int64     `json:"size_bytes"`
	SetAsPrimary     bool      `json:"set_as_primary"`
}

func (h *fileHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		httperror.Render(w, r, err)
		return
	}
	if err := entityAccess(r.Context(), req.EntityType, true); err != nil {
		httperror.Render(w, r, err)
		return
	}
	row, err := h.svc.Register(r.Context(), files.RegisterInput{
		EntityType:       req.EntityType,
		EntityID:         req.EntityID,
		FileType:         domain.FileType(req.FileType),
		StorageKey:       req.StorageKey,
		OriginalFilename: req.OriginalFilename,
		ContentType:      req.ContentType,
		SizeBytes:        req.SizeBytes,
		SetAsPrimary:     req.SetAsPrimary,
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toFileDTO(row))
}

func (h *fileHandler) list(w http.ResponseWriter, r *http.Request) {
	entityType := queryString(r, "entity_type")
	entityID, err := queryUUID(r, "entity_id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	if entityType == "" || entityID == nil {
		httperror.Render(w, r, httperror.BadRequest("entity_type and entity_id are required"))
		return
	}
	if err := entityAccess(r.Context(), entityType, false); err != nil {
		httperror.Render(w, r, err)
		return
	}
	rows, err := h.svc.List(r.Context(), files.ListFilesInput{
		EntityType: entityType,
		EntityID:   *entityID,
		FileType:   queryString(r, "file_type"),
	})
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	out := make([]fileDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toFileDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *fileHandler) download(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	res, err := h.svc.Download(r.Context(), id)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *fileHandler) delete(w http.ResponseWriter, r *http.Request) {
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
