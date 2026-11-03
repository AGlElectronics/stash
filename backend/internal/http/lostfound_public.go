package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/files"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

type publicLostFoundHandler struct {
	pool  *pgxpool.Pool
	files *files.Service
}

type publicLostFoundDTO struct {
	ReportNumber string  `json:"report_number"`
	ItemName     string  `json:"item_name"`
	Description  string  `json:"description"`
	Location     string  `json:"location"`
	DateReported string  `json:"date_reported"`
	PhotoURL     *string `json:"photo_url,omitempty"`
}

func (h *publicLostFoundHandler) getByNumber(w http.ResponseWriter, r *http.Request) {
	reportNumber := chi.URLParam(r, "reportNumber")
	if reportNumber == "" {
		httperror.Render(w, r, httperror.BadRequest("report number is required"))
		return
	}
	q := sqlcgen.New(h.pool)
	row, err := q.GetPublicLostFoundByNumber(r.Context(), reportNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httperror.Render(w, r, httperror.NotFound("report"))
			return
		}
		httperror.Render(w, r, httperror.Internal(err))
		return
	}
	writeJSON(w, http.StatusOK, toPublicLostFoundDTO(row.ReportNumber, row.ItemName, row.Description, row.Location, row.DateReported))
}

func (h *publicLostFoundHandler) search(w http.ResponseWriter, r *http.Request) {
	search := queryString(r, "q")
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)
	if limit > 50 {
		limit = 50
	}

	q := sqlcgen.New(h.pool)
	var searchPtr *string
	if search != "" {
		searchPtr = &search
	}

	rows, err := q.SearchPublicLostFound(r.Context(), sqlcgen.SearchPublicLostFoundParams{
		Search: searchPtr,
		Off:    offset,
		Lim:    limit,
	})
	if err != nil {
		httperror.Render(w, r, httperror.Internal(err))
		return
	}
	total, err := q.CountPublicLostFound(r.Context(), searchPtr)
	if err != nil {
		httperror.Render(w, r, httperror.Internal(err))
		return
	}

	out := make([]publicLostFoundDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPublicLostFoundDTO(row.ReportNumber, row.ItemName, row.Description, row.Location, row.DateReported))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
		"total": total,
	})
}

func toPublicLostFoundDTO(reportNumber, itemName, description, location string, dateReported pgtype.Timestamptz) publicLostFoundDTO {
	photoURL := publicLostFoundPhotoURL(reportNumber)
	return publicLostFoundDTO{
		ReportNumber: reportNumber,
		ItemName:     itemName,
		Description:  description,
		Location:     location,
		DateReported: timestampString(dateReported),
		PhotoURL:     &photoURL,
	}
}

func publicLostFoundPhotoURL(reportNumber string) string {
	return "/api/public/lostfound/" + url.PathEscape(reportNumber) + "/photo"
}

func (h *publicLostFoundHandler) photo(w http.ResponseWriter, r *http.Request) {
	reportNumber := chi.URLParam(r, "reportNumber")
	if reportNumber == "" {
		httperror.Render(w, r, httperror.BadRequest("report number is required"))
		return
	}

	q := sqlcgen.New(h.pool)
	row, err := q.GetPublicLostFoundByNumber(r.Context(), reportNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httperror.Render(w, r, httperror.NotFound("report"))
			return
		}
		httperror.Render(w, r, httperror.Internal(err))
		return
	}

	fileID, err := resolvePublicLostFoundPhotoID(r.Context(), q, row.ID, row.PhotoFileID)
	if err != nil {
		httperror.Render(w, r, httperror.Internal(err))
		return
	}
	if fileID == nil {
		httperror.Render(w, r, httperror.NotFound("photo"))
		return
	}

	stream, rc, err := h.files.RawStream(r.Context(), *fileID)
	if err != nil {
		httperror.Render(w, r, err)
		return
	}
	defer func() { _ = rc.Close() }()

	if stream.ContentType != "" {
		w.Header().Set("Content-Type", stream.ContentType)
	}
	if stream.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(stream.SizeBytes, 10))
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if stream.OriginalFilename != "" {
		w.Header().Set("Content-Disposition", `inline; filename="`+sanitizeFilename(stream.OriginalFilename)+`"`)
	}
	if _, err := io.Copy(w, rc); err != nil {
		_ = err
	}
}

func resolvePublicLostFoundPhotoID(ctx context.Context, q *sqlcgen.Queries, reportID uuid.UUID, photoFileID *uuid.UUID) (*uuid.UUID, error) {
	imageType := string(domain.FileTypeImage)

	if photoFileID != nil {
		f, err := q.GetFile(ctx, *photoFileID)
		if err == nil &&
			f.EntityType == "lostfound" &&
			f.EntityID == reportID &&
			f.FileType == imageType {
			return photoFileID, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	files, err := q.ListFilesForEntity(ctx, sqlcgen.ListFilesForEntityParams{
		EntityType: "lostfound",
		EntityID:   reportID,
		Column3:    imageType,
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	id := files[0].ID
	return &id, nil
}
