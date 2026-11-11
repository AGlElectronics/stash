package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// ObjectRemover deletes stored objects of an entity after their rows are
// gone (files.Service in production).
type ObjectRemover interface {
	RemoveEntityObject(ctx context.Context, key, entityType string, entityID uuid.UUID)
}

// RetentionService deletes Lost & Found reports once their retention has
// passed, together with their photos and Telegram link, and redacts guest
// data in the audit history of those reports.
type RetentionService struct {
	pool     *pgxpool.Pool
	settings *SettingsService
	objects  ObjectRemover
}

func NewRetentionService(pool *pgxpool.Pool, settings *SettingsService, objects ObjectRemover) *RetentionService {
	return &RetentionService{pool: pool, settings: settings, objects: objects}
}

// PurgeResult says what one run did.
type PurgeResult struct {
	Mode    string
	Deleted []string // report numbers
}

// PurgeLostFound applies the retention setting at time now.
//
//   - after_close: reports returned or closed (or removed) more than
//     lostfound_retention_hours ago are deleted;
//   - after_con: once lostfound_retention_days have passed since the end of
//     con_end_date, every report is deleted (nothing happens while the date
//     is not set);
//   - off: nothing is deleted.
//
// LF-0001 is skipped while the sacred-report easter egg is on.
func (s *RetentionService) PurgeLostFound(ctx context.Context, now time.Time) (PurgeResult, error) {
	set, err := s.settings.Get(ctx)
	if err != nil {
		return PurgeResult{}, err
	}
	res := PurgeResult{Mode: set.LostFoundRetentionMode}
	var rows pgx.Rows
	switch set.LostFoundRetentionMode {
	case RetentionAfterClose:
		cutoff := now.Add(-time.Duration(set.LostFoundRetentionHours) * time.Hour)
		rows, err = s.pool.Query(ctx, `
			SELECT id, report_number FROM lost_found_reports
			 WHERE (status IN ('returned','closed') AND resolved_at < $1)
			    OR (removed_at IS NOT NULL AND removed_at < $1)`, cutoff)
	case RetentionAfterCon:
		end, ok := set.ConEnd()
		if !ok || now.Before(end.AddDate(0, 0, set.LostFoundRetentionDays)) {
			return res, nil
		}
		rows, err = s.pool.Query(ctx, `SELECT id, report_number FROM lost_found_reports`)
	default:
		return res, nil
	}
	if err != nil {
		return res, httperror.Internal(err)
	}
	type report struct {
		id     uuid.UUID
		number string
	}
	var due []report
	for rows.Next() {
		var r report
		if err := rows.Scan(&r.id, &r.number); err != nil {
			rows.Close()
			return res, httperror.Internal(err)
		}
		if set.PreserveSacredLostFound && domain.LostFoundReportIsSacred(r.number) {
			continue
		}
		due = append(due, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, httperror.Internal(err)
	}

	note := retentionNote(set)
	for _, r := range due {
		if err := s.purgeReport(ctx, r.id, note); err != nil {
			return res, fmt.Errorf("purge %s: %w", r.number, err)
		}
		res.Deleted = append(res.Deleted, r.number)
	}
	return res, nil
}

func retentionNote(set SystemSettings) string {
	if set.LostFoundRetentionMode == RetentionAfterCon {
		return fmt.Sprintf("retention: %d days after the convention (%s)", set.LostFoundRetentionDays, set.ConEndDate)
	}
	return fmt.Sprintf("retention: %d hours after closing", set.LostFoundRetentionHours)
}

// purgeReport deletes one report and its files, and redacts guest data in
// every earlier audit entry about it, in one transaction. The objects in
// storage are removed after the commit.
func (s *RetentionService) purgeReport(ctx context.Context, id uuid.UUID, note string) error {
	type object struct{ key string }
	var objects []object
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetLostFoundReportAny(ctx, id)
		if err != nil {
			return httperror.Internal(err)
		}
		if err := RedactAuditHistory(ctx, tx, id); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `DELETE FROM files WHERE entity_type = 'lostfound' AND entity_id = $1 RETURNING storage_key`, id)
		if err != nil {
			return httperror.Internal(err)
		}
		for rows.Next() {
			var o object
			if err := rows.Scan(&o.key); err != nil {
				rows.Close()
				return httperror.Internal(err)
			}
			objects = append(objects, o)
		}
		rows.Close()
		if _, err := tx.Exec(ctx, `DELETE FROM lost_found_reports WHERE id = $1`, id); err != nil {
			return httperror.Internal(err)
		}
		snap, _ := toMap(reportForAudit(before))
		redacted := audit.RedactGuestData(mustJSON(snap))
		return audit.RecordRaw(ctx, qtx, "lostfound.purge", "lostfound", id, redacted, nil, note)
	})
	if err != nil {
		return err
	}
	if s.objects != nil {
		for _, o := range objects {
			s.objects.RemoveEntityObject(ctx, o.key, "lostfound", id)
		}
	}
	return nil
}

// RedactAuditHistory replaces guest data in every audit entry about a Lost
// & Found report and its files.
func RedactAuditHistory(ctx context.Context, tx pgx.Tx, reportID uuid.UUID) error {
	rows, err := tx.Query(ctx, `
		SELECT id, before_json, after_json FROM audit_log
		 WHERE (entity_type = 'lostfound' AND entity_id = $1)
		    OR (entity_type = 'file' AND (before_json->>'entity_id' = $2 OR after_json->>'entity_id' = $2))`,
		reportID, reportID.String())
	if err != nil {
		return httperror.Internal(err)
	}
	type entry struct {
		id            uuid.UUID
		before, after []byte
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.before, &e.after); err != nil {
			rows.Close()
			return httperror.Internal(err)
		}
		entries = append(entries, e)
	}
	rows.Close()
	for _, e := range entries {
		if _, err := tx.Exec(ctx, `UPDATE audit_log SET before_json = $2, after_json = $3,
		        note = CASE WHEN entity_type = 'lostfound' AND note <> '' THEN $4 ELSE note END
		  WHERE id = $1`,
			e.id, nullJSON(audit.RedactGuestData(e.before)), nullJSON(audit.RedactGuestData(e.after)), audit.Redacted); err != nil {
			return httperror.Internal(err)
		}
	}
	return nil
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
