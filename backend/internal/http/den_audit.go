package http

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/hlog"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// auditDen records a successful Den pair, sync or unpair in the audit log
// (entity_type "den"). The pairing URL and credential are never logged;
// before and after hold the connection status (Den address, module id,
// times).
func auditDen(pool *pgxpool.Pool, den *denlink.Client, action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		before, _ := den.Status(r.Context())
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next(ww, r)
		if ww.Status() < 200 || ww.Status() > 299 {
			return
		}
		after, _ := den.Status(r.Context())
		if err := audit.Record(r.Context(), sqlcgen.New(pool), action, "den", uuid.Nil, before, after, ""); err != nil {
			hlog.FromRequest(r).Error().Err(err).Str("action", action).Msg("audit den change")
		}
	}
}
