package http

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

const dashboardRecentSignouts = 5

// dashboardResponse is the per-viewer aggregate for the SPA dashboard.
// Sections the viewer has no permission for are omitted entirely (nil), so
// the SPA can distinguish "0 items" from "not allowed to see this".
type dashboardResponse struct {
	OverdueSignouts           *int64       `json:"overdue_signouts,omitempty"`
	ActiveSignouts            *int64       `json:"active_signouts,omitempty"`
	PartiallyReturnedSignouts *int64       `json:"partially_returned_signouts,omitempty"`
	RecentSignouts            []signoutDTO `json:"recent_signouts,omitempty"`
	LowStockItems             *int64       `json:"low_stock_items,omitempty"`
	DamagedItems              *int64       `json:"damaged_items,omitempty"`
	OpenLostfoundReports      *int64       `json:"open_lostfound_reports,omitempty"`
}

// dashboardHandler serves GET /api/dashboard: pre-aggregated counts so the
// SPA no longer fetches items with limit=1000 to derive stats client-side.
func dashboardHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		u, ok := auth.FromContext(ctx)
		if !ok {
			httperror.Render(w, r, httperror.New(httperror.CodeUnauthorized, "not authenticated"))
			return
		}
		q := sqlcgen.New(pool)
		var resp dashboardResponse

		if u.Has(domain.PermSignoutView) {
			overdue, err := q.CountOverdueSignouts(ctx)
			if err != nil {
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			active, err := q.CountActiveSignouts(ctx)
			if err != nil {
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			partial, err := q.CountPartiallyReturnedSignouts(ctx)
			if err != nil {
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			recent, err := q.ListRecentActiveSignouts(ctx, dashboardRecentSignouts)
			if err != nil {
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			resp.OverdueSignouts = &overdue
			resp.ActiveSignouts = &active
			resp.PartiallyReturnedSignouts = &partial
			resp.RecentSignouts = make([]signoutDTO, 0, len(recent))
			for _, s := range recent {
				resp.RecentSignouts = append(resp.RecentSignouts, toSignoutDTO(s))
			}
		}

		if u.Has(domain.PermItemView) {
			lowStock, err := q.CountLowStockItems(ctx)
			if err != nil {
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			damaged, err := q.CountDamagedItems(ctx)
			if err != nil {
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			resp.LowStockItems = &lowStock
			resp.DamagedItems = &damaged
		}

		if u.Has(domain.PermLostfoundView) {
			open, err := q.CountOpenLostFoundReports(ctx)
			if err != nil {
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			resp.OpenLostfoundReports = &open
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
