package http

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/hlog"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

func auditLogHandler(pool *pgxpool.Pool) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", listAuditLog(pool))
	}
}

// clampPage keeps limit within 1..max and offset at 0 or more.
func clampPage(limit, offset, max int32) (int32, int32) {
	if limit <= 0 {
		limit = 1
	}
	if limit > max {
		limit = max
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func listAuditLog(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Require(r.Context(), domain.PermAuditView); err != nil {
			httperror.Render(w, r, err)
			return
		}
		lim, off := clampPage(queryInt(r, "limit", 100), queryInt(r, "offset", 0), 500)
		params := sqlcgen.ListAuditLogParams{Lim: lim, Off: off}
		if v, err := queryUUID(r, "actor"); err != nil {
			httperror.Render(w, r, err)
			return
		} else if v != nil {
			params.Actor = v
		}
		if v, err := queryUUID(r, "entity_id"); err != nil {
			httperror.Render(w, r, err)
			return
		} else if v != nil {
			params.EntityID = v
		}
		if v := r.URL.Query().Get("action"); v != "" {
			params.Action = &v
		}
		if v := r.URL.Query().Get("entity_type"); v != "" {
			params.EntityType = &v
		}

		q := sqlcgen.New(pool)
		rows, err := q.ListAuditLog(r.Context(), params)
		if err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}
		// Guest personal data from Lost & Found stays hidden from people who
		// cannot see Lost & Found reports themselves.
		canSeeGuests := auth.HasPermission(r.Context(), domain.PermLostfoundView)
		out := make([]map[string]any, 0, len(rows))
		for _, a := range rows {
			if !canSeeGuests && audit.HasGuestData(a.EntityType, a.BeforeJson, a.AfterJson) {
				a.BeforeJson = audit.RedactGuestData(a.BeforeJson)
				a.AfterJson = audit.RedactGuestData(a.AfterJson)
				if a.EntityType == "lostfound" && a.Note != "" {
					a.Note = audit.Redacted
				}
			}
			out = append(out, map[string]any{
				"id":                 a.ID.String(),
				"actor_user_id":      uuidPtrToString2(a.ActorUserID),
				"actor_oidc_subject": strOrEmpty(a.ActorOidcSubject),
				"actor_display_name": strOrEmpty(a.ActorDisplayName),
				"action":             a.Action,
				"entity_type":        a.EntityType,
				"entity_id":          a.EntityID.String(),
				"before":             rawJSON(a.BeforeJson),
				"after":              rawJSON(a.AfterJson),
				"note":               a.Note,
				"created_at":         timestampString(a.CreatedAt),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out, "limit": lim, "offset": off})
	}
}

// rawJSON returns a stored JSONB snapshot as JSON (not base64), or null.
func rawJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}

// csvSafe neutralises spreadsheet formulas: a text cell starting with
// = + - @ or a tab/carriage return gets a leading apostrophe so Excel and
// LibreOffice show it as text. Plain numbers are left alone.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			return s
		}
		return "'" + s
	}
	return s
}

// safeCSV writes CSV rows with every cell passed through csvSafe.
type safeCSV struct{ *csv.Writer }

func newSafeCSV(w io.Writer) safeCSV { return safeCSV{csv.NewWriter(w)} }

func (c safeCSV) Write(row []string) error {
	out := make([]string, len(row))
	for i, v := range row {
		out[i] = csvSafe(v)
	}
	return c.Writer.Write(out)
}

// auditExport records who downloaded which export.
func auditExport(r *http.Request, pool *pgxpool.Pool, action string, rows int, filters string) {
	note := fmt.Sprintf("%d rows", rows)
	if filters != "" {
		note += "; " + filters
	}
	if err := audit.Record(r.Context(), sqlcgen.New(pool), action, "export", uuid.Nil, nil, nil, note); err != nil {
		hlog.FromRequest(r).Error().Err(err).Str("action", action).Msg("audit export")
	}
}

func locationExportPath(locs []sqlcgen.Location, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	byID := make(map[uuid.UUID]sqlcgen.Location, len(locs))
	for _, l := range locs {
		byID[l.ID] = l
	}
	var parts []string
	cur, ok := byID[*id]
	for ok {
		parts = append([]string{cur.Name}, parts...)
		if cur.ParentID == nil {
			break
		}
		cur, ok = byID[*cur.ParentID]
	}
	return strings.Join(parts, " / ")
}

// exportItemsCSV streams a CSV of (non-retired) items with resolved labels.
// Optional query: parent_container_id — export only direct contents of that container.
func exportItemsCSV(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Require(r.Context(), domain.PermExportCreate); err != nil {
			httperror.Render(w, r, err)
			return
		}
		parentContainerID, err := queryUUID(r, "parent_container_id")
		if err != nil {
			httperror.Render(w, r, err)
			return
		}
		q := sqlcgen.New(pool)
		incRet := false
		rows, err := q.ListItems(r.Context(), sqlcgen.ListItemsParams{
			ParentContainerID: parentContainerID,
			IncludeRetired:    &incRet,
			Lim:               100000,
			Off:               0,
		})
		if err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}

		depts, err := q.ListDepartments(r.Context())
		if err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}
		deptName := make(map[uuid.UUID]string, len(depts))
		for _, d := range depts {
			deptName[d.ID] = d.Name
		}

		locs, err := q.ListLocations(r.Context())
		if err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}

		itemName := make(map[uuid.UUID]string, len(rows))
		for _, it := range rows {
			itemName[it.ID] = it.Name
		}
		parentIDs := make(map[uuid.UUID]struct{})
		for _, it := range rows {
			if it.ParentContainerID != nil {
				parentIDs[*it.ParentContainerID] = struct{}{}
			}
		}
		for pid := range parentIDs {
			if _, ok := itemName[pid]; ok {
				continue
			}
			row, err := q.GetItem(r.Context(), pid)
			if err != nil {
				continue
			}
			itemName[pid] = row.Name
		}

		filters := ""
		if parentContainerID != nil {
			filters = "parent_container_id=" + parentContainerID.String()
		}
		auditExport(r, pool, "export.items", len(rows), filters)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="items-`+time.Now().Format("20060102")+`.csv"`)
		cw := newSafeCSV(w)
		_ = cw.Write([]string{
			"id", "name", "type", "status", "condition",
			"quantity_owned", "quantity_damaged", "quantity_lost",
			"unit", "category_id",
			"managed_by_department_id", "managed_by_department_name",
			"needed_by_department_id", "needed_by_department_name",
			"location_id", "location_path",
			"parent_container_id", "parent_container_name",
			"serial_number", "asset_tag", "qr_code",
			"purchase_date", "expiry_date", "purchase_price", "purchase_currency",
			"created_at",
		})
		for _, it := range rows {
			var parentContainerName string
			if it.ParentContainerID != nil {
				parentContainerName = itemName[*it.ParentContainerID]
			}
			_ = cw.Write([]string{
				it.ID.String(),
				it.Name,
				it.ItemType,
				it.Status,
				it.Condition,
				itoa32(it.QuantityOwned),
				itoa32(it.QuantityDamaged),
				itoa32(it.QuantityLost),
				it.Unit,
				uuidStr(it.CategoryID),
				uuidStr(it.ManagedByDepartmentID),
				deptLookup(deptName, it.ManagedByDepartmentID),
				uuidStr(it.NeededByDepartmentID),
				deptLookup(deptName, it.NeededByDepartmentID),
				uuidStr(it.LocationID),
				locationExportPath(locs, it.LocationID),
				uuidStr(it.ParentContainerID),
				parentContainerName,
				strOrEmpty(it.SerialNumber),
				strOrEmpty(it.AssetTag),
				strOrEmpty(it.QrCode),
				dateString(it.PurchaseDate),
				dateString(it.ExpiryDate),
				numericString(it.PurchasePrice),
				it.PurchaseCurrency,
				timestampString(it.CreatedAt),
			})
		}
		cw.Flush()
	}
}

func deptLookup(m map[uuid.UUID]string, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return m[*id]
}

// exportLostFoundCSV streams the Lost & Found datasheet: every report that
// has not been removed, including guest contact details. It needs both
// export.create and lostfound.view, and every download is audited.
func exportLostFoundCSV(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Require(r.Context(), domain.PermExportCreate); err != nil {
			httperror.Render(w, r, err)
			return
		}
		if err := auth.Require(r.Context(), domain.PermLostfoundView); err != nil {
			httperror.Render(w, r, err)
			return
		}
		rows, err := sqlcgen.New(pool).ListLostFoundReports(r.Context(), sqlcgen.ListLostFoundReportsParams{Lim: 100000})
		if err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}
		auditExport(r, pool, "export.lostfound", len(rows), "")
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="lost-and-found-`+time.Now().Format("20060102")+`.csv"`)
		_, _ = io.WriteString(w, "\ufeff")
		cw := newSafeCSV(w)
		_ = cw.Write([]string{
			"report_number", "type", "status", "item_name", "description", "location",
			"date_reported", "contact_name", "contact_info", "returned_to", "resolved_at", "resolution_note",
		})
		for _, rep := range rows {
			_ = cw.Write([]string{
				rep.ReportNumber, rep.Type, rep.Status, rep.ItemName, rep.Description, rep.Location,
				timestampString(rep.DateReported), rep.ContactName, rep.ContactInfo, rep.ReturnedTo,
				timestampString(rep.ResolvedAt), rep.ResolutionNote,
			})
		}
		cw.Flush()
	}
}

// exportSignoutsCSV streams a CSV of all sign-outs.
func exportSignoutsCSV(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Require(r.Context(), domain.PermExportCreate); err != nil {
			httperror.Render(w, r, err)
			return
		}
		q := sqlcgen.New(pool)
		rows, err := q.ListSignouts(r.Context(), sqlcgen.ListSignoutsParams{
			Lim: 100000,
			Off: 0,
		})
		if err != nil {
			httperror.Render(w, r, httperror.Internal(err))
			return
		}
		auditExport(r, pool, "export.signouts", len(rows), "")
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="signouts-`+time.Now().Format("20060102")+`.csv"`)
		cw := newSafeCSV(w)
		_ = cw.Write([]string{
			"signout_number", "status", "to_department_id", "from_department_id",
			"received_by_name", "purpose", "signed_out_at", "returned_at", "closed_at", "cancelled_at",
		})
		for _, s := range rows {
			_ = cw.Write([]string{
				s.SignoutNumber,
				s.Status,
				s.ToDepartmentID.String(),
				uuidStr(s.FromDepartmentID),
				strOrEmpty(s.ReceivedByName),
				s.Purpose,
				timestampString(s.SignedOutAt),
				timestampString(s.ReturnedAt),
				timestampString(s.ClosedAt),
				timestampString(s.CancelledAt),
			})
		}
		cw.Flush()
	}
}
