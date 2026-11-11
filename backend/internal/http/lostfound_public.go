package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// publicLostFoundHandler serves the guest claim lookup. It only shows open
// "found" reports and only what a guest needs to recognise their item and
// come to the desk: no description, location, photo, contact or notes,
// which could identify someone or help a false claim.
type publicLostFoundHandler struct {
	pool *pgxpool.Pool
}

type publicLostFoundDTO struct {
	ReportNumber string `json:"report_number"`
	ItemName     string `json:"item_name"`
	// DateReported is the day only (YYYY-MM-DD).
	DateReported string `json:"date_reported"`
}

func (h *publicLostFoundHandler) getByNumber(w http.ResponseWriter, r *http.Request) {
	reportNumber := chi.URLParam(r, "reportNumber")
	if reportNumber == "" {
		httperror.Render(w, r, httperror.BadRequest("report number is required"))
		return
	}
	row, err := sqlcgen.New(h.pool).GetPublicLostFoundByNumber(r.Context(), reportNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httperror.Render(w, r, httperror.NotFound("report"))
			return
		}
		httperror.Render(w, r, httperror.Internal(err))
		return
	}
	writeJSON(w, http.StatusOK, toPublicLostFoundDTO(row.ReportNumber, row.ItemName, row.DateReported))
}

func (h *publicLostFoundHandler) search(w http.ResponseWriter, r *http.Request) {
	search := queryString(r, "q")
	limit, offset := clampPage(queryInt(r, "limit", 20), queryInt(r, "offset", 0), 50)

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
		out = append(out, toPublicLostFoundDTO(row.ReportNumber, row.ItemName, row.DateReported))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
		"total": total,
	})
}

func toPublicLostFoundDTO(reportNumber, itemName string, dateReported pgtype.Timestamptz) publicLostFoundDTO {
	return publicLostFoundDTO{
		ReportNumber: reportNumber,
		ItemName:     itemName,
		DateReported: dateString(pgtype.Date{Time: dateReported.Time, Valid: dateReported.Valid}),
	}
}
