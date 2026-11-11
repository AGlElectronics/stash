package http

import (
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// activeSignedOutSQL is the number of an item's units out on sign-outs,
// the same sum the availability check at sign-out time uses.
const activeSignedOutSQL = `COALESCE((
	SELECT SUM(dsi.quantity_signed_out - dsi.quantity_returned - dsi.quantity_consumed - dsi.quantity_lost - dsi.quantity_damaged)
	  FROM department_signout_items dsi
	  JOIN department_signouts ds ON ds.id = dsi.signout_id
	 WHERE dsi.item_id = i.id AND ds.status IN ('signed_out','partially_returned','prepared')), 0)`

const itemReportColumns = `i.id::text, i.name, coalesce(i.asset_tag, ''), coalesce(c.name, ''), coalesce(d.name, ''),
	i.quantity_owned::text, i.quantity_damaged::text, i.quantity_lost::text,
	(` + activeSignedOutSQL + `)::text,
	(i.quantity_owned - i.quantity_damaged - i.quantity_lost - ` + activeSignedOutSQL + `)::text,
	i.minimum_quantity::text, coalesce(to_char(i.expiry_date, 'YYYY-MM-DD'), ''), i.condition`

const itemReportFrom = ` FROM items i
	LEFT JOIN item_categories c ON c.id = i.category_id
	LEFT JOIN departments d ON d.id = i.managed_by_department_id `

var itemReportHeader = []string{
	"id", "name", "asset_tag", "category", "managed_by_department",
	"quantity_owned", "quantity_damaged", "quantity_lost", "quantity_signed_out",
	"quantity_available", "minimum_quantity", "expiry_date", "condition",
}

// plannedReport is one ready-made export.
type plannedReport struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	header      []string
	query       string
}

var plannedReports = map[string]plannedReport{
	"low-stock": {
		Title: "Low stock", Description: "Items with fewer available units than their minimum quantity.",
		header: itemReportHeader,
		query: `SELECT ` + itemReportColumns + itemReportFrom + `
			WHERE i.retired_at IS NULL AND i.minimum_quantity > 0
			  AND (i.quantity_owned - i.quantity_damaged - i.quantity_lost - ` + activeSignedOutSQL + `) < i.minimum_quantity
			ORDER BY lower(i.name)`,
	},
	"damaged": {
		Title: "Damaged", Description: "Items with damaged units waiting for repair.",
		header: itemReportHeader,
		query:  `SELECT ` + itemReportColumns + itemReportFrom + ` WHERE i.retired_at IS NULL AND i.quantity_damaged > 0 ORDER BY lower(i.name)`,
	},
	"lost": {
		Title: "Lost", Description: "Items with units marked lost.",
		header: itemReportHeader,
		query:  `SELECT ` + itemReportColumns + itemReportFrom + ` WHERE i.retired_at IS NULL AND i.quantity_lost > 0 ORDER BY lower(i.name)`,
	},
	"expiring": {
		Title: "Expiring", Description: "Items that expire within 60 days or have expired.",
		header: itemReportHeader,
		query: `SELECT ` + itemReportColumns + itemReportFrom + `
			WHERE i.retired_at IS NULL AND i.expiry_date IS NOT NULL AND i.expiry_date < current_date + 60
			ORDER BY i.expiry_date`,
	},
	"retired": {
		Title: "Retired", Description: "Items taken out of use.",
		header: itemReportHeader,
		query:  `SELECT ` + itemReportColumns + itemReportFrom + ` WHERE i.retired_at IS NOT NULL ORDER BY lower(i.name)`,
	},
	"overdue-signouts": {
		Title: "Overdue sign-outs", Description: "Sign-outs past their expected return.",
		header: []string{"signout_number", "status", "department", "received_by", "expected_return_at", "items_outstanding"},
		query: `SELECT s.signout_number, s.status, d.name, coalesce(s.received_by_name, ''),
			       to_char(s.expected_return_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			       (SELECT coalesce(sum(quantity_signed_out - quantity_returned - quantity_consumed - quantity_lost - quantity_damaged), 0)
			          FROM department_signout_items WHERE signout_id = s.id)::text
			  FROM department_signouts s JOIN departments d ON d.id = s.to_department_id
			 WHERE s.status IN ('signed_out','partially_returned') AND s.expected_return_at < now()
			 ORDER BY s.expected_return_at`,
	},
}

func init() {
	for k, r := range plannedReports {
		r.Key = k
		plannedReports[k] = r
	}
}

// listPlannedReports serves GET /api/export/reports.
func listPlannedReports(w http.ResponseWriter, r *http.Request) {
	out := make([]plannedReport, 0, len(plannedReports))
	for _, rep := range plannedReports {
		out = append(out, rep)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// exportPlannedReport serves GET /api/export/reports/{report}.csv. It
// needs export.create and is audited like every export.
func exportPlannedReport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Require(r.Context(), domain.PermExportCreate); err != nil {
			httperror.Render(w, r, err)
			return
		}
		rep, ok := plannedReports[chi.URLParam(r, "report")]
		if !ok {
			httperror.Render(w, r, httperror.NotFound("report"))
			return
		}
		rows, err := pool.Query(r.Context(), rep.query)
		if err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}
		var data [][]string
		for rows.Next() {
			vals := make([]string, len(rep.header))
			ptrs := make([]any, len(vals))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				httperror.Render(w, r, httperror.Internal(err))
				return
			}
			data = append(data, vals)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}
		auditExport(r, pool, "export.report", len(data), "report="+rep.Key)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+rep.Key+`-`+time.Now().Format("20060102")+`.csv"`)
		cw := newSafeCSV(w)
		_ = cw.Write(rep.header)
		for _, row := range data {
			_ = cw.Write(row)
		}
		cw.Flush()
	}
}
