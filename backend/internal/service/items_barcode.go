package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

const (
	// maxAssignAttempts is how many times we retry on a unique-violation when
	// assigning. Conflicts only happen if the next sequence value collides
	// with a manually-entered tag, so 5 attempts is plenty in practice.
	maxAssignAttempts = 5
)

// FormatItemBarcode renders a sequence value using the configured prefix/width.
func FormatItemBarcode(n int64, prefix string, width int) string {
	if strings.TrimSpace(prefix) == "" {
		prefix = "ST"
	}
	if width <= 0 {
		width = 6
	}
	return fmt.Sprintf("%s%0*d", prefix, width, n)
}

// FormatDefaultBarcode formats with the default ST + 6-digit width.
func FormatDefaultBarcode(n int64) string {
	return FormatItemBarcode(n, "ST", 6)
}

// FormatAWBarcode is a deprecated alias for FormatDefaultBarcode.
func FormatAWBarcode(n int64) string {
	return FormatDefaultBarcode(n)
}

// PeekNextAWBarcode returns what the next generated item barcode WOULD be without
// consuming a sequence value. It is intended for UI previews. Any consumer
// must still call AssignAWBarcode to actually claim a code.
func (s *ItemService) PeekNextAWBarcode(ctx context.Context) (string, error) {
	const q = `
SELECT COALESCE(
    (SELECT last_value + (CASE WHEN is_called THEN 1 ELSE 0 END)
       FROM aw_barcode_seq),
    1
)::bigint
`
	var next int64
	if err := s.pool.QueryRow(ctx, q).Scan(&next); err != nil {
		return "", httperror.Internal(err)
	}
	sys, err := NewSettingsService(s.pool).Get(ctx)
	if err != nil {
		return "", err
	}
	prefix, width := sys.BarcodeFormat()
	return FormatItemBarcode(next, prefix, width), nil
}

// hasItemScanCode reports whether the item already has a barcode or QR identifier.
func hasItemScanCode(assetTag, qrCode *string) bool {
	if assetTag != nil && strings.TrimSpace(*assetTag) != "" {
		return true
	}
	if qrCode != nil && strings.TrimSpace(*qrCode) != "" {
		return true
	}
	return false
}

// assignItemBarcodeTx claims the next sequence value and writes it to asset_tag.
func assignItemBarcodeTx(
	ctx context.Context,
	tx pgx.Tx,
	qtx *sqlcgen.Queries,
	id uuid.UUID,
	before sqlcgen.Item,
	prefix string,
	width int,
) (sqlcgen.Item, string, error) {
	for attempt := 0; attempt < maxAssignAttempts; attempt++ {
		var next int64
		if err := tx.QueryRow(ctx, "SELECT nextval('aw_barcode_seq')").Scan(&next); err != nil {
			return sqlcgen.Item{}, "", httperror.Internal(err)
		}
		candidate := FormatItemBarcode(next, prefix, width)

		tag := candidate
		row, err := qtx.SetItemAssetTag(ctx, sqlcgen.SetItemAssetTagParams{
			ID:       id,
			AssetTag: &tag,
		})
		if err == nil {
			note := "assigned barcode " + candidate
			if err := audit.Record(ctx, qtx, "item.assign_barcode", "item", id, before, row, note); err != nil {
				return sqlcgen.Item{}, "", err
			}
			return row, candidate, nil
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return sqlcgen.Item{}, "", mapPgError(err, "item", "duplicate asset tag")
		}
	}
	return sqlcgen.Item{}, "", httperror.Conflict("could not allocate a unique barcode after several attempts")
}

// AssignAWBarcode generates the next item barcode and writes it to asset_tag.
// Prefix/width come from system_settings (defaults AW / 6).
func (s *ItemService) AssignAWBarcode(ctx context.Context, id uuid.UUID) (sqlcgen.Item, string, error) {
	sys, err := NewSettingsService(s.pool).Get(ctx)
	if err != nil {
		return sqlcgen.Item{}, "", err
	}
	prefix, width := sys.BarcodeFormat()

	var (
		out  sqlcgen.Item
		code string
	)

	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}

		var assignErr error
		out, code, assignErr = assignItemBarcodeTx(ctx, tx, qtx, id, before, prefix, width)
		return assignErr
	})

	return out, code, err
}

// SetItemBarcode writes a caller-provided code (e.g. a pre-printed sticker)
// to the item's asset_tag, with optional case normalisation. Validation:
// non-empty, no whitespace inside, length 1..64.
func (s *ItemService) SetItemBarcode(ctx context.Context, id uuid.UUID, code string) (sqlcgen.Item, error) {
	code = strings.TrimSpace(code)
	code = strings.TrimRight(code, "\r\n")
	if code == "" {
		return sqlcgen.Item{}, httperror.Validation("barcode is required", nil)
	}
	if strings.ContainsAny(code, " \t") {
		return sqlcgen.Item{}, httperror.Validation("barcode must not contain spaces or tabs", nil)
	}
	if len(code) > 64 {
		return sqlcgen.Item{}, httperror.Validation("barcode is too long (max 64 chars)", nil)
	}

	var out sqlcgen.Item
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := sqlcgen.New(tx)
		before, err := qtx.GetItem(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httperror.NotFound("item")
			}
			return httperror.Internal(err)
		}
		tag := code
		row, err := qtx.SetItemAssetTag(ctx, sqlcgen.SetItemAssetTagParams{
			ID:       id,
			AssetTag: &tag,
		})
		if err != nil {
			return mapPgError(err, "item", "barcode is already used by another item")
		}
		out = row
		return audit.Record(ctx, qtx, "item.set_barcode", "item", id, before, row, "barcode set to "+code)
	})
	return out, err
}
