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
	"github.com/AGlElectronics/stash/backend/internal/events"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

type SignoutService struct {
	pool     *pgxpool.Pool
	settings *SettingsService
}

func NewSignoutService(pool *pgxpool.Pool) *SignoutService {
	return &SignoutService{pool: pool, settings: NewSettingsService(pool)}
}

// ----------------------------------------------------------------------------
// Inputs
// ----------------------------------------------------------------------------

type SignoutItemInput struct {
	ItemID             uuid.UUID
	Quantity           int32
	ConditionOnSignout *string
	Notes              string
	// ScannedCodes are the barcodes (asset_tag / qr_code / serial_number)
	// the operator scanned for this line, in the order scanned. When the
	// system setting force_barcode_on_signout is on, every line MUST have
	// at least one code that resolves to the same item_id.
	ScannedCodes []string
}

type CreateSignoutInput struct {
	FromDepartmentID *uuid.UUID
	ToDepartmentID   uuid.UUID
	FromLocationID   *uuid.UUID
	Purpose          string
	ExpectedReturnAt *time.Time
	ReceivedByUserID *uuid.UUID
	ReceivedByName   string
	Notes            string
	Items            []SignoutItemInput
	Confirm          bool // true means: jump straight from draft -> signed_out (fast flow)
}

type ListSignoutsInput struct {
	Search      string
	Status      string
	ToDeptID    *uuid.UUID
	FromDeptID  *uuid.UUID
	SignedOutBy *uuid.UUID
	Limit       int32
	Offset      int32
}

type ListSignoutsResult struct {
	Items []sqlcgen.DepartmentSignout
	Total int64
}

type SignoutWithItems struct {
	Signout sqlcgen.DepartmentSignout
	Items   []sqlcgen.DepartmentSignoutItem
}

type ReturnLine struct {
	SignoutItemID     uuid.UUID
	ReturnedDelta     int32
	ConsumedDelta     int32
	LostDelta         int32
	DamagedDelta      int32
	ConditionOnReturn *string
}

// ----------------------------------------------------------------------------
// Reads
// ----------------------------------------------------------------------------

func (s *SignoutService) Get(ctx context.Context, id uuid.UUID) (SignoutWithItems, error) {
	q := sqlcgen.New(s.pool)
	row, err := q.GetSignout(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SignoutWithItems{}, httperror.NotFound("signout")
		}
		return SignoutWithItems{}, httperror.Internal(err)
	}
	items, err := q.ListSignoutItems(ctx, id)
	if err != nil {
		return SignoutWithItems{}, httperror.Internal(err)
	}
	return SignoutWithItems{Signout: row, Items: items}, nil
}

func (s *SignoutService) List(ctx context.Context, in ListSignoutsInput) (ListSignoutsResult, error) {
	if in.Limit <= 0 {
		in.Limit = 50
	}
	if in.Limit > 200 {
		in.Limit = 200
	}
	q := sqlcgen.New(s.pool)
	rows, err := q.ListSignouts(ctx, sqlcgen.ListSignoutsParams{
		Status:      optString(in.Status),
		ToDeptID:    in.ToDeptID,
		FromDeptID:  in.FromDeptID,
		SignedOutBy: in.SignedOutBy,
		Search:      optString(in.Search),
		Lim:         in.Limit,
		Off:         in.Offset,
	})
	if err != nil {
		return ListSignoutsResult{}, httperror.Internal(err)
	}
	total, err := q.CountSignouts(ctx, sqlcgen.CountSignoutsParams{
		Status:      optString(in.Status),
		ToDeptID:    in.ToDeptID,
		FromDeptID:  in.FromDeptID,
		SignedOutBy: in.SignedOutBy,
		Search:      optString(in.Search),
	})
	if err != nil {
		return ListSignoutsResult{}, httperror.Internal(err)
	}
	return ListSignoutsResult{Items: rows, Total: total}, nil
}

// ----------------------------------------------------------------------------
// Writes
// ----------------------------------------------------------------------------

// Create creates a sign-out (draft or, with Confirm=true, signed_out) along
// with all line items in one atomic transaction. This powers both the slow
// "build a draft, then confirm" flow and the fast "sign out a power strip in
// 30 seconds" flow.
func (s *SignoutService) Create(ctx context.Context, in CreateSignoutInput) (SignoutWithItems, error) {
	if len(in.Items) == 0 {
		return SignoutWithItems{}, httperror.Validation("at least one item is required", nil)
	}
	if in.ToDepartmentID == uuid.Nil {
		return SignoutWithItems{}, httperror.Validation("to_department_id is required", nil)
	}

	settings, err := s.settings.Get(ctx)
	if err != nil {
		return SignoutWithItems{}, err
	}

	var out SignoutWithItems
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)

		number, err := qtx.NextSignoutNumber(ctx)
		if err != nil {
			return httperror.Internal(err)
		}

		initialStatus := string(domain.SignoutStatusDraft)
		if in.Confirm {
			initialStatus = string(domain.SignoutStatusSignedOut)
		}

		signout, err := qtx.CreateSignout(ctx, sqlcgen.CreateSignoutParams{
			SignoutNumber:    number,
			FromDepartmentID: in.FromDepartmentID,
			ToDepartmentID:   in.ToDepartmentID,
			FromLocationID:   in.FromLocationID,
			Status:           initialStatus,
			Purpose:          in.Purpose,
			ExpectedReturnAt: tsOrNullVal(in.ExpectedReturnAt),
			ReceivedByUserID: in.ReceivedByUserID,
			ReceivedByName:   strPtr(in.ReceivedByName),
			Notes:            in.Notes,
			CreatedBy:        auth.ActorID(ctx),
		})
		if err != nil {
			return mapPgError(err, "signout", "duplicate signout number")
		}

		// stamp signed_out_by_user_id when the fast-create-and-confirm path is used
		if in.Confirm {
			actor := auth.ActorID(ctx)
			signout, err = qtx.SetSignoutStatus(ctx, sqlcgen.SetSignoutStatusParams{
				ID:                signout.ID,
				Status:            string(domain.SignoutStatusSignedOut),
				SignedOutByUserID: actor,
			})
			if err != nil {
				return httperror.Internal(err)
			}
		}

		// validate availability against current state for each item, then add
		lineItems := make([]sqlcgen.DepartmentSignoutItem, 0, len(in.Items))
		for _, line := range in.Items {
			if line.Quantity <= 0 {
				return httperror.Validation("each line item quantity must be > 0", nil)
			}
			itemRow, err := qtx.GetItem(ctx, line.ItemID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return httperror.Validation(fmt.Sprintf("item %s not found", line.ItemID), nil)
				}
				return httperror.Internal(err)
			}
			if itemRow.RetiredAt.Valid {
				return httperror.Validation(fmt.Sprintf("item %q is retired", itemRow.Name), nil)
			}
			active, err := qtx.ItemSignedOutAggregate(ctx, line.ItemID)
			if err != nil {
				return httperror.Internal(err)
			}
			free := itemRow.QuantityOwned - itemRow.QuantityDamaged - itemRow.QuantityLost - active
			if line.Quantity > free {
				return httperror.Conflict(fmt.Sprintf(
					"item %q has only %d available (you asked for %d)", itemRow.Name, free, line.Quantity))
			}

			// Barcode validation: when forced, every line needs at least one
			// scanned code that matches THIS item. If the operator scanned a
			// different item's code, fail loudly. We don't require N codes for
			// quantity N because individual units share an asset_tag in the
			// current data model.
			if err := validateScannedCodes(itemRow, line.ScannedCodes, settings.ForceBarcodeOnSignout); err != nil {
				return err
			}

			row, err := qtx.AddSignoutItem(ctx, sqlcgen.AddSignoutItemParams{
				SignoutID:          signout.ID,
				ItemID:             line.ItemID,
				QuantitySignedOut:  line.Quantity,
				ConditionOnSignout: line.ConditionOnSignout,
				Notes:              mergeScannedNotes(line.Notes, line.ScannedCodes),
			})
			if err != nil {
				return mapPgError(err, "signout_item", "")
			}
			lineItems = append(lineItems, row)

			if in.Confirm {
				if _, err := events.Record(ctx, qtx, line.ItemID, domain.EventSignedOut, events.Input{
					Quantity:         line.Quantity,
					ToDepartmentID:   ptrUUID(in.ToDepartmentID),
					RelatedSignoutID: &signout.ID,
					Note:             in.Purpose,
				}); err != nil {
					return err
				}
			}
		}

		out = SignoutWithItems{Signout: signout, Items: lineItems}
		action := "signout.create"
		if in.Confirm {
			action = "signout.create_and_confirm"
		}
		return audit.Record(ctx, qtx, action, "signout", signout.ID, nil, signout, in.Purpose)
	})
	return out, err
}

// Confirm transitions a draft/prepared sign-out to signed_out. Items already
// have rows; we just emit the events and stamp the actor.
func (s *SignoutService) Confirm(ctx context.Context, id uuid.UUID) (SignoutWithItems, error) {
	var out SignoutWithItems
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetSignout(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("signout")
			}
			return httperror.Internal(err)
		}
		switch domain.SignoutStatus(before.Status) {
		case domain.SignoutStatusDraft, domain.SignoutStatusPrepared:
			// allowed
		default:
			return httperror.Conflict(fmt.Sprintf("cannot confirm a %s sign-out", before.Status))
		}
		items, err := qtx.ListSignoutItems(ctx, id)
		if err != nil {
			return httperror.Internal(err)
		}
		if len(items) == 0 {
			return httperror.Validation("sign-out has no items", nil)
		}

		actor := auth.ActorID(ctx)
		row, err := qtx.SetSignoutStatus(ctx, sqlcgen.SetSignoutStatusParams{
			ID:                id,
			Status:            string(domain.SignoutStatusSignedOut),
			SignedOutByUserID: actor,
		})
		if err != nil {
			return httperror.Internal(err)
		}

		for _, line := range items {
			if _, err := events.Record(ctx, qtx, line.ItemID, domain.EventSignedOut, events.Input{
				Quantity:         line.QuantitySignedOut,
				ToDepartmentID:   ptrUUID(before.ToDepartmentID),
				RelatedSignoutID: &id,
				Note:             before.Purpose,
			}); err != nil {
				return err
			}
		}

		out = SignoutWithItems{Signout: row, Items: items}
		return audit.Record(ctx, qtx, "signout.confirm", "signout", id, before, row, "")
	})
	return out, err
}

// Return processes a return for a sign-out. Each ReturnLine references one of
// the existing sign-out items; quantities for returned/consumed/lost/damaged
// are added incrementally. If everything is settled across all items, the
// sign-out moves to "returned"; otherwise to "partially_returned".
func (s *SignoutService) Return(ctx context.Context, id uuid.UUID, lines []ReturnLine) (SignoutWithItems, error) {
	if len(lines) == 0 {
		return SignoutWithItems{}, httperror.Validation("no return lines provided", nil)
	}
	var out SignoutWithItems
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetSignout(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("signout")
			}
			return httperror.Internal(err)
		}
		switch domain.SignoutStatus(before.Status) {
		case domain.SignoutStatusSignedOut, domain.SignoutStatusPartiallyReturned:
			// allowed
		default:
			return httperror.Conflict(fmt.Sprintf("cannot return on a %s sign-out", before.Status))
		}

		for _, line := range lines {
			if line.ReturnedDelta < 0 || line.ConsumedDelta < 0 || line.LostDelta < 0 || line.DamagedDelta < 0 {
				return httperror.Validation("return deltas must be non-negative", nil)
			}
			if line.ReturnedDelta+line.ConsumedDelta+line.LostDelta+line.DamagedDelta == 0 {
				continue
			}
			soItem, err := qtx.GetSignoutItem(ctx, line.SignoutItemID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return httperror.Validation(fmt.Sprintf("signout item %s not found", line.SignoutItemID), nil)
				}
				return httperror.Internal(err)
			}
			if soItem.SignoutID != id {
				return httperror.Validation("signout item does not belong to this signout", nil)
			}
			settledAfter := soItem.QuantityReturned + line.ReturnedDelta +
				soItem.QuantityConsumed + line.ConsumedDelta +
				soItem.QuantityLost + line.LostDelta +
				soItem.QuantityDamaged + line.DamagedDelta
			if settledAfter > soItem.QuantitySignedOut {
				return httperror.Validation("settled quantity would exceed signed-out quantity", nil)
			}

			updated, err := qtx.UpdateSignoutItemReturn(ctx, sqlcgen.UpdateSignoutItemReturnParams{
				ID:                line.SignoutItemID,
				ReturnedDelta:     line.ReturnedDelta,
				ConsumedDelta:     line.ConsumedDelta,
				LostDelta:         line.LostDelta,
				DamagedDelta:      line.DamagedDelta,
				ConditionOnReturn: line.ConditionOnReturn,
			})
			if err != nil {
				return httperror.Internal(err)
			}
			_ = updated

			// emit events: returned/consumed/lost/damaged as needed
			if line.ReturnedDelta > 0 {
				if _, err := events.Record(ctx, qtx, soItem.ItemID, domain.EventReturned, events.Input{
					Quantity:         line.ReturnedDelta,
					FromDepartmentID: ptrUUID(before.ToDepartmentID),
					RelatedSignoutID: &id,
				}); err != nil {
					return err
				}
			}
			if line.ConsumedDelta > 0 {
				// consumed = stock disappears permanently; reduce quantity_owned
				if _, err := qtx.AdjustItemQuantity(ctx, sqlcgen.AdjustItemQuantityParams{
					ID:    soItem.ItemID,
					Delta: -line.ConsumedDelta,
				}); err != nil {
					return mapPgError(err, "item", "")
				}
				if _, err := events.Record(ctx, qtx, soItem.ItemID, domain.EventConsumed, events.Input{
					Quantity:         line.ConsumedDelta,
					RelatedSignoutID: &id,
				}); err != nil {
					return err
				}
			}
			if line.LostDelta > 0 {
				if _, err := qtx.SetItemLostDelta(ctx, sqlcgen.SetItemLostDeltaParams{
					ID:    soItem.ItemID,
					Delta: line.LostDelta,
				}); err != nil {
					return mapPgError(err, "item", "")
				}
				if _, err := events.Record(ctx, qtx, soItem.ItemID, domain.EventMarkedLost, events.Input{
					Quantity:         line.LostDelta,
					RelatedSignoutID: &id,
					Note:             "lost on return",
				}); err != nil {
					return err
				}
			}
			if line.DamagedDelta > 0 {
				if _, err := qtx.SetItemDamagedDelta(ctx, sqlcgen.SetItemDamagedDeltaParams{
					ID:    soItem.ItemID,
					Delta: line.DamagedDelta,
				}); err != nil {
					return mapPgError(err, "item", "")
				}
				if _, err := events.Record(ctx, qtx, soItem.ItemID, domain.EventMarkedDamaged, events.Input{
					Quantity:         line.DamagedDelta,
					RelatedSignoutID: &id,
					ConditionAfter:   ptr("damaged"),
					Note:             "damaged on return",
				}); err != nil {
					return err
				}
			}
		}

		// recompute aggregate to decide signout status
		agg, err := qtx.SignoutItemsAggregate(ctx, id)
		if err != nil {
			return httperror.Internal(err)
		}
		nextStatus := string(domain.SignoutStatusPartiallyReturned)
		if agg.Settled >= agg.SignedOut {
			nextStatus = string(domain.SignoutStatusReturned)
		}

		row, err := qtx.SetSignoutStatus(ctx, sqlcgen.SetSignoutStatusParams{
			ID:     id,
			Status: nextStatus,
		})
		if err != nil {
			return httperror.Internal(err)
		}

		items, err := qtx.ListSignoutItems(ctx, id)
		if err != nil {
			return httperror.Internal(err)
		}

		out = SignoutWithItems{Signout: row, Items: items}
		return audit.Record(ctx, qtx, "signout.return", "signout", id, before, row, "")
	})
	return out, err
}

// Cancel a draft / prepared sign-out without affecting inventory.
func (s *SignoutService) Cancel(ctx context.Context, id uuid.UUID, note string) (SignoutWithItems, error) {
	var out SignoutWithItems
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetSignout(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("signout")
			}
			return httperror.Internal(err)
		}
		switch domain.SignoutStatus(before.Status) {
		case domain.SignoutStatusDraft, domain.SignoutStatusPrepared:
			// allowed
		default:
			return httperror.Conflict(fmt.Sprintf("cannot cancel a %s sign-out", before.Status))
		}
		row, err := qtx.SetSignoutStatus(ctx, sqlcgen.SetSignoutStatusParams{
			ID:     id,
			Status: string(domain.SignoutStatusCancelled),
		})
		if err != nil {
			return httperror.Internal(err)
		}
		items, err := qtx.ListSignoutItems(ctx, id)
		if err != nil {
			return httperror.Internal(err)
		}
		out = SignoutWithItems{Signout: row, Items: items}
		return audit.Record(ctx, qtx, "signout.cancel", "signout", id, before, row, note)
	})
	return out, err
}

// Close a returned sign-out (admin step to archive).
func (s *SignoutService) Close(ctx context.Context, id uuid.UUID) (SignoutWithItems, error) {
	var out SignoutWithItems
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetSignout(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("signout")
			}
			return httperror.Internal(err)
		}
		if before.Status != string(domain.SignoutStatusReturned) {
			return httperror.Conflict(fmt.Sprintf("cannot close a %s sign-out", before.Status))
		}
		row, err := qtx.SetSignoutStatus(ctx, sqlcgen.SetSignoutStatusParams{
			ID:     id,
			Status: string(domain.SignoutStatusClosed),
		})
		if err != nil {
			return httperror.Internal(err)
		}
		items, err := qtx.ListSignoutItems(ctx, id)
		if err != nil {
			return httperror.Internal(err)
		}
		out = SignoutWithItems{Signout: row, Items: items}
		return audit.Record(ctx, qtx, "signout.close", "signout", id, before, row, "")
	})
	return out, err
}

// helpers

func tsOrNullVal(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrUUID(u uuid.UUID) *uuid.UUID {
	v := u
	return &v
}

// validateScannedCodes checks the operator-provided codes against the line's
// item. The forced flag is the system setting; when on, each line must have
// at least one matching code. When off, codes are optional but those that
// are present must still match the line's item (otherwise we'd happily
// record nonsense in the audit trail).
func validateScannedCodes(item sqlcgen.Item, codes []string, forced bool) error {
	matches := func(code string) bool {
		if code == "" {
			return false
		}
		if item.AssetTag != nil && *item.AssetTag == code {
			return true
		}
		if item.QrCode != nil && *item.QrCode == code {
			return true
		}
		if item.SerialNumber != nil && *item.SerialNumber == code {
			return true
		}
		return false
	}
	matched := 0
	for _, code := range codes {
		if !matches(code) {
			return httperror.Validation(
				fmt.Sprintf("scanned code %q does not belong to item %q", code, item.Name), nil)
		}
		matched++
	}
	if forced && matched == 0 {
		return httperror.Validation(
			fmt.Sprintf("a barcode scan is required for %q", item.Name), nil)
	}
	return nil
}

// mergeScannedNotes appends "scanned: code1, code2" to the operator note so
// the codes survive in the existing notes column without a schema change.
func mergeScannedNotes(notes string, codes []string) string {
	if len(codes) == 0 {
		return notes
	}
	suffix := "scanned: "
	for i, c := range codes {
		if i > 0 {
			suffix += ", "
		}
		suffix += c
	}
	if notes == "" {
		return suffix
	}
	return notes + "\n" + suffix
}
