package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// LostFoundService implements the Lost & Found reports that replace the
// ConOps AppSheet/Google Sheets app. Reports are guest-facing paperwork
// logged by staff; they intentionally do not touch Stash inventory.
type LostFoundService struct {
	pool *pgxpool.Pool
}

func NewLostFoundService(pool *pgxpool.Pool) *LostFoundService {
	return &LostFoundService{pool: pool}
}

// ----------------------------------------------------------------------------
// Inputs
// ----------------------------------------------------------------------------

type ListLostFoundInput struct {
	Type   string
	Status string
	Search string
	Limit  int32
	Offset int32
}

type ListLostFoundResult struct {
	Items []sqlcgen.LostFoundReport
	Total int64
}

type CreateLostFoundInput struct {
	Type         string
	ItemName     string
	Description  string
	PhotoFileID  *uuid.UUID
	Location     string
	DateReported *time.Time // optional back-dating; defaults to now()
	ContactName  string
	ContactInfo  string
}

// UpdateLostFoundInput is a PATCH: nil pointer = leave unchanged.
// PhotoFileID uses ClearPhoto to distinguish "remove the photo" from
// "don't touch the photo".
type UpdateLostFoundInput struct {
	Type           *string
	Status         *string
	ItemName       *string
	Description    *string
	PhotoFileID    *uuid.UUID
	ClearPhoto     bool
	Location       *string
	ContactName    *string
	ContactInfo    *string
	ReturnedTo     *string
	ResolutionNote *string
}

// ----------------------------------------------------------------------------
// Reads
// ----------------------------------------------------------------------------

func (s *LostFoundService) Get(ctx context.Context, id uuid.UUID) (sqlcgen.LostFoundReport, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetLostFoundReport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.LostFoundReport{}, httperror.NotFound("lost & found report")
		}
		return sqlcgen.LostFoundReport{}, httperror.Internal(err)
	}
	return row, nil
}

func (s *LostFoundService) GetByReportNumber(ctx context.Context, reportNumber string) (sqlcgen.LostFoundReport, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetLostFoundByReportNumber(ctx, reportNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.LostFoundReport{}, httperror.NotFound("lost & found report")
		}
		return sqlcgen.LostFoundReport{}, httperror.Internal(err)
	}
	return row, nil
}

func (s *LostFoundService) List(ctx context.Context, in ListLostFoundInput) (ListLostFoundResult, error) {
	if in.Type != "" && !domain.LostFoundType(in.Type).Valid() {
		return ListLostFoundResult{}, httperror.Validation("type must be 'lost' or 'found'", nil)
	}
	if in.Status != "" && !domain.LostFoundStatus(in.Status).Valid() {
		return ListLostFoundResult{}, httperror.Validation("status must be 'open', 'returned' or 'closed'", nil)
	}
	if in.Limit <= 0 {
		in.Limit = 50
	}
	if in.Limit > 200 {
		in.Limit = 200
	}
	q := sqlcgen.New(s.pool)
	rows, err := q.ListLostFoundReports(ctx, sqlcgen.ListLostFoundReportsParams{
		Type:   optString(in.Type),
		Status: optString(in.Status),
		Search: optString(in.Search),
		Lim:    in.Limit,
		Off:    in.Offset,
	})
	if err != nil {
		return ListLostFoundResult{}, httperror.Internal(err)
	}
	total, err := q.CountLostFoundReports(ctx, sqlcgen.CountLostFoundReportsParams{
		Type:   optString(in.Type),
		Status: optString(in.Status),
		Search: optString(in.Search),
	})
	if err != nil {
		return ListLostFoundResult{}, httperror.Internal(err)
	}
	return ListLostFoundResult{Items: rows, Total: total}, nil
}

// ----------------------------------------------------------------------------
// Writes
// ----------------------------------------------------------------------------

func (s *LostFoundService) Create(ctx context.Context, in CreateLostFoundInput) (sqlcgen.LostFoundReport, error) {
	if !domain.LostFoundType(in.Type).Valid() {
		return sqlcgen.LostFoundReport{}, httperror.Validation("type must be 'lost' or 'found'", nil)
	}
	if in.ItemName == "" {
		return sqlcgen.LostFoundReport{}, httperror.Validation("item_name is required", nil)
	}

	var out sqlcgen.LostFoundReport
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)

		if err := validateLostFoundPhoto(ctx, qtx, in.PhotoFileID); err != nil {
			return err
		}

		number, err := qtx.NextLostFoundNumber(ctx)
		if err != nil {
			return httperror.Internal(err)
		}

		row, err := qtx.CreateLostFoundReport(ctx, sqlcgen.CreateLostFoundReportParams{
			ReportNumber:     number,
			Type:             in.Type,
			ItemName:         in.ItemName,
			Description:      in.Description,
			PhotoFileID:      in.PhotoFileID,
			Location:         in.Location,
			DateReported:     tsOrNullVal(in.DateReported),
			ReportedByUserID: auth.ActorID(ctx),
			ContactName:      in.ContactName,
			ContactInfo:      in.ContactInfo,
		})
		if err != nil {
			return mapPgError(err, "lostfound", "duplicate report number")
		}
		out = row
		return audit.Record(ctx, qtx, "lostfound.create", "lostfound", row.ID, nil, row, "")
	})
	return out, err
}

// Update applies a PATCH. Field edits require lostfound.edit; status
// transitions additionally require lostfound.resolve. Transitions to
// returned/closed stamp the resolution metadata; reopening clears it.
func (s *LostFoundService) Update(ctx context.Context, id uuid.UUID, in UpdateLostFoundInput) (sqlcgen.LostFoundReport, error) {
	var out sqlcgen.LostFoundReport
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetLostFoundReport(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("lost & found report")
			}
			return httperror.Internal(err)
		}

		next := before // value copy; mutate then persist
		fieldsEdited := false

		if in.Type != nil && *in.Type != before.Type {
			if !domain.LostFoundType(*in.Type).Valid() {
				return httperror.Validation("type must be 'lost' or 'found'", nil)
			}
			next.Type = *in.Type
			fieldsEdited = true
		}
		if in.ItemName != nil && *in.ItemName != before.ItemName {
			if *in.ItemName == "" {
				return httperror.Validation("item_name must not be empty", nil)
			}
			next.ItemName = *in.ItemName
			fieldsEdited = true
		}
		if in.Description != nil && *in.Description != before.Description {
			next.Description = *in.Description
			fieldsEdited = true
		}
		if in.Location != nil && *in.Location != before.Location {
			next.Location = *in.Location
			fieldsEdited = true
		}
		if in.ContactName != nil && *in.ContactName != before.ContactName {
			next.ContactName = *in.ContactName
			fieldsEdited = true
		}
		if in.ContactInfo != nil && *in.ContactInfo != before.ContactInfo {
			next.ContactInfo = *in.ContactInfo
			fieldsEdited = true
		}
		if in.ClearPhoto {
			next.PhotoFileID = nil
			fieldsEdited = fieldsEdited || before.PhotoFileID != nil
		} else if in.PhotoFileID != nil {
			if err := validateLostFoundPhoto(ctx, qtx, in.PhotoFileID); err != nil {
				return err
			}
			next.PhotoFileID = in.PhotoFileID
			fieldsEdited = fieldsEdited || before.PhotoFileID == nil || *before.PhotoFileID != *in.PhotoFileID
		}

		if fieldsEdited {
			sys, settingsErr := NewSettingsService(s.pool).Get(ctx)
			if settingsErr != nil {
				return settingsErr
			}
			if sys.PreserveSacredLostFound && domain.LostFoundReportIsSacred(before.ReportNumber) {
				return httperror.New(httperror.CodeForbidden, "LF-0001 is the sacred test item and cannot be edited")
			}
			if err := auth.RequireAny(ctx, domain.PermLostfoundEdit); err != nil {
				return err
			}
		}

		action := "lostfound.update"
		statusChanged := in.Status != nil && *in.Status != before.Status
		if statusChanged {
			if !domain.LostFoundStatus(*in.Status).Valid() {
				return httperror.Validation("status must be 'open', 'returned' or 'closed'", nil)
			}
			next.Status = *in.Status
			switch domain.LostFoundStatus(*in.Status) {
			case domain.LostFoundStatusOpen:
				// Reopening requires an elevated permission (lead-only).
				if err := auth.RequireAny(ctx, domain.PermLostfoundReopen); err != nil {
					return err
				}
				action = "lostfound.reopen"
				next.ReturnedTo = ""
				next.ResolvedByUserID = nil
				next.ResolvedAt = pgtype.Timestamptz{}
				next.ResolutionNote = ""
			case domain.LostFoundStatusReturned, domain.LostFoundStatusClosed:
				if err := auth.RequireAny(ctx, domain.PermLostfoundResolve); err != nil {
					return err
				}
				action = "lostfound.close"
				if domain.LostFoundStatus(*in.Status) == domain.LostFoundStatusReturned {
					action = "lostfound.return"
				}
				next.ResolvedByUserID = auth.ActorID(ctx)
				next.ResolvedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			}
		}
		// returned_to / resolution_note may come with the status change or
		// as a later correction (the latter counts as a resolve-level edit).
		if in.ReturnedTo != nil && *in.ReturnedTo != before.ReturnedTo {
			if !statusChanged {
				if err := auth.RequireAny(ctx, domain.PermLostfoundResolve); err != nil {
					return err
				}
			}
			next.ReturnedTo = *in.ReturnedTo
		}
		if in.ResolutionNote != nil && *in.ResolutionNote != before.ResolutionNote {
			if !statusChanged {
				if err := auth.RequireAny(ctx, domain.PermLostfoundResolve); err != nil {
					return err
				}
			}
			next.ResolutionNote = *in.ResolutionNote
		}

		row, err := qtx.UpdateLostFoundReport(ctx, sqlcgen.UpdateLostFoundReportParams{
			ID:               id,
			Type:             next.Type,
			Status:           next.Status,
			ItemName:         next.ItemName,
			Description:      next.Description,
			PhotoFileID:      next.PhotoFileID,
			Location:         next.Location,
			ContactName:      next.ContactName,
			ContactInfo:      next.ContactInfo,
			ReturnedTo:       next.ReturnedTo,
			ResolvedByUserID: next.ResolvedByUserID,
			ResolvedAt:       next.ResolvedAt,
			ResolutionNote:   next.ResolutionNote,
		})
		if err != nil {
			return mapPgError(err, "lostfound", "")
		}
		out = row
		return audit.Record(ctx, qtx, action, "lostfound", id, before, row, "")
	})
	return out, err
}

// Delete soft-removes a Lost & Found report (sets removed_at). Requires
// lostfound.delete. The row stays in the database but disappears from the UI.
func (s *LostFoundService) Delete(ctx context.Context, id uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetLostFoundReport(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("lost & found report")
			}
			return httperror.Internal(err)
		}
		sys, settingsErr := NewSettingsService(s.pool).Get(ctx)
		if settingsErr != nil {
			return settingsErr
		}
		if sys.PreserveSacredLostFound && domain.LostFoundReportIsSacred(before.ReportNumber) {
			return httperror.New(httperror.CodeForbidden, "LF-0001 is the sacred test item and cannot be removed")
		}
		row, err := qtx.RemoveLostFoundReport(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("lost & found report")
			}
			return httperror.Internal(err)
		}
		return audit.Record(ctx, qtx, "lostfound.remove", "lostfound", id, before, row, "")
	})
}

// validateLostFoundPhoto checks that a referenced photo file exists and is an
// image. Reports are created before the photo is uploaded (the file upload
// needs the report id as entity_id), so the reference arrives via PATCH.
func validateLostFoundPhoto(ctx context.Context, q *sqlcgen.Queries, fileID *uuid.UUID) error {
	if fileID == nil {
		return nil
	}
	row, err := q.GetFile(ctx, *fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httperror.Validation(fmt.Sprintf("photo file %s not found", fileID), nil)
		}
		return httperror.Internal(err)
	}
	if row.FileType != string(domain.FileTypeImage) {
		return httperror.Validation("photo_file_id must reference an image file", nil)
	}
	return nil
}
