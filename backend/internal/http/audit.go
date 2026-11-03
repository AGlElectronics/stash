package http

import (
	"encoding/csv"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

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

func listAuditLog(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.RequireGlobal(r.Context(), domain.PermAuditView); err != nil {
			httperror.Render(w, r, err)
			return
		}
		params := sqlcgen.ListAuditLogParams{
			Lim: queryInt(r, "limit", 100),
			Off: queryInt(r, "offset", 0),
		}
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
		out := make([]map[string]any, 0, len(rows))
		for _, a := range rows {
			out = append(out, map[string]any{
				"id":                 a.ID.String(),
				"actor_user_id":      uuidPtrToString2(a.ActorUserID),
				"actor_oidc_subject": strOrEmpty(a.ActorOidcSubject),
				"actor_display_name": strOrEmpty(a.ActorDisplayName),
				"action":             a.Action,
				"entity_type":        a.EntityType,
				"entity_id":          a.EntityID.String(),
				"before":             a.BeforeJson,
				"after":              a.AfterJson,
				"note":               a.Note,
				"created_at":         timestampString(a.CreatedAt),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
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
		if err := auth.RequireGlobal(r.Context(), domain.PermExportCreate); err != nil {
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

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="items-`+time.Now().Format("20060102")+`.csv"`)
		cw := csv.NewWriter(w)
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

// exportSignoutsCSV streams a CSV of all sign-outs.
func exportSignoutsCSV(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.RequireGlobal(r.Context(), domain.PermExportCreate); err != nil {
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
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="signouts-`+time.Now().Format("20060102")+`.csv"`)
		cw := csv.NewWriter(w)
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
