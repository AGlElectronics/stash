package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// PackingList is what goes to one event or department.
type PackingList struct {
	ID             uuid.UUID         `db:"id" json:"id"`
	Name           string            `db:"name" json:"name"`
	DepartmentID   *uuid.UUID        `db:"department_id" json:"department_id,omitempty"`
	DepartmentName *string           `db:"department_name" json:"department_name,omitempty"`
	EventDate      *string           `db:"event_date" json:"event_date,omitempty"`
	Notes          string            `db:"notes" json:"notes"`
	SignoutID      *uuid.UUID        `db:"signout_id" json:"signout_id,omitempty"`
	ItemCount      int32             `db:"item_count" json:"item_count"`
	PackedCount    int32             `db:"packed_count" json:"packed_count"`
	CreatedAt      time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time         `db:"updated_at" json:"updated_at"`
	Items          []PackingListItem `db:"-" json:"items,omitempty"`
}

// PackingListItem is one line: how many of an item, and how many are packed.
type PackingListItem struct {
	ItemID    uuid.UUID `db:"item_id" json:"item_id"`
	ItemName  string    `db:"item_name" json:"item_name"`
	AssetTag  *string   `db:"asset_tag" json:"asset_tag,omitempty"`
	Quantity  int32     `db:"quantity" json:"quantity"`
	Packed    int32     `db:"packed" json:"packed"`
	Note      string    `db:"note" json:"note"`
	Available int32     `db:"available" json:"available"`
}

const packingSelect = `SELECT p.id, p.name, p.department_id, d.name AS department_name,
	to_char(p.event_date, 'YYYY-MM-DD') AS event_date, p.notes, p.signout_id,
	(SELECT count(*) FROM packing_list_items WHERE list_id = p.id)::int AS item_count,
	(SELECT count(*) FROM packing_list_items WHERE list_id = p.id AND packed >= quantity)::int AS packed_count,
	p.created_at, p.updated_at
	FROM packing_lists p LEFT JOIN departments d ON d.id = p.department_id `

// PackingService manages packing lists. Viewing needs signout.view,
// changing them signout.create (checked in the HTTP layer).
type PackingService struct {
	pool     *pgxpool.Pool
	signouts *SignoutService
}

func NewPackingService(pool *pgxpool.Pool) *PackingService {
	return &PackingService{pool: pool, signouts: NewSignoutService(pool)}
}

// PackingListInput creates or renames a list.
type PackingListInput struct {
	Name         string     `json:"name"`
	DepartmentID *uuid.UUID `json:"department_id,omitempty"`
	EventDate    string     `json:"event_date"`
	Notes        string     `json:"notes"`
}

// PackingLineInput is one line of PUT /api/packing-lists/{id}/items.
type PackingLineInput struct {
	ItemID   uuid.UUID `json:"item_id"`
	Quantity int32     `json:"quantity"`
	Packed   int32     `json:"packed"`
	Note     string    `json:"note"`
}

// List returns every packing list, upcoming events first.
func (s *PackingService) List(ctx context.Context) ([]PackingList, error) {
	rows, err := s.pool.Query(ctx, packingSelect+`ORDER BY p.event_date NULLS LAST, lower(p.name) LIMIT 500`)
	if err != nil {
		return nil, httperror.Internal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[PackingList])
	if err != nil {
		return nil, httperror.Internal(err)
	}
	return out, nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Get returns a list with its lines and each item's available quantity.
func (s *PackingService) Get(ctx context.Context, id uuid.UUID) (PackingList, error) {
	return s.get(ctx, s.pool, id)
}

func (s *PackingService) get(ctx context.Context, q querier, id uuid.UUID) (PackingList, error) {
	rows, err := q.Query(ctx, packingSelect+`WHERE p.id = $1`, id)
	if err != nil {
		return PackingList{}, httperror.Internal(err)
	}
	pl, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[PackingList])
	if errors.Is(err, pgx.ErrNoRows) {
		return PackingList{}, httperror.NotFound("packing list")
	}
	if err != nil {
		return PackingList{}, httperror.Internal(err)
	}
	rows, err = q.Query(ctx, `SELECT l.item_id, i.name AS item_name, i.asset_tag, l.quantity, l.packed, l.note,
		(i.quantity_owned - i.quantity_damaged - i.quantity_lost - COALESCE((
			SELECT SUM(dsi.quantity_signed_out - dsi.quantity_returned - dsi.quantity_consumed - dsi.quantity_lost - dsi.quantity_damaged)
			  FROM department_signout_items dsi JOIN department_signouts ds ON ds.id = dsi.signout_id
			 WHERE dsi.item_id = i.id AND ds.status IN ('signed_out','partially_returned','prepared')), 0))::int AS available
		FROM packing_list_items l JOIN items i ON i.id = l.item_id
		WHERE l.list_id = $1 ORDER BY l.position, lower(i.name)`, id)
	if err != nil {
		return PackingList{}, httperror.Internal(err)
	}
	if pl.Items, err = pgx.CollectRows(rows, pgx.RowToStructByName[PackingListItem]); err != nil {
		return PackingList{}, httperror.Internal(err)
	}
	return pl, nil
}

func eventDate(s string) (*string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return nil, httperror.Validation("event_date must be a date like 2026-08-14", nil)
	}
	return &s, nil
}

// Create adds an empty list.
func (s *PackingService) Create(ctx context.Context, in PackingListInput) (PackingList, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return PackingList{}, httperror.Validation("name is required", nil)
	}
	date, err := eventDate(in.EventDate)
	if err != nil {
		return PackingList{}, err
	}
	var out PackingList
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO packing_lists (name, department_id, event_date, notes, created_by)
			VALUES ($1, $2, $3::date, $4, $5) RETURNING id`, in.Name, in.DepartmentID, date, in.Notes, auth.ActorID(ctx)).Scan(&id); err != nil {
			return mapPgError(err, "packing list", "")
		}
		var err error
		if out, err = s.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, sqlcgen.New(tx), "packing.create", "packing_list", id, nil, out, "")
	})
	return out, err
}

// Update changes the name, department, date or notes.
func (s *PackingService) Update(ctx context.Context, id uuid.UUID, in PackingListInput) (PackingList, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return PackingList{}, httperror.Validation("name is required", nil)
	}
	date, err := eventDate(in.EventDate)
	if err != nil {
		return PackingList{}, err
	}
	var out PackingList
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE packing_lists SET name = $2, department_id = $3, event_date = $4::date, notes = $5, updated_at = now()
			WHERE id = $1`, id, in.Name, in.DepartmentID, date, in.Notes); err != nil {
			return mapPgError(err, "packing list", "")
		}
		if out, err = s.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, sqlcgen.New(tx), "packing.update", "packing_list", id, before, out, "")
	})
	return out, err
}

// SetItems replaces all lines of a list.
func (s *PackingService) SetItems(ctx context.Context, id uuid.UUID, lines []PackingLineInput) (PackingList, error) {
	seen := map[uuid.UUID]bool{}
	for _, l := range lines {
		if l.Quantity <= 0 || l.Packed < 0 || l.Packed > l.Quantity {
			return PackingList{}, httperror.Validation("each line needs a quantity of at least 1 and 0 to quantity packed", nil)
		}
		if seen[l.ItemID] {
			return PackingList{}, httperror.Validation("an item may appear only once per list", nil)
		}
		seen[l.ItemID] = true
	}
	var out PackingList
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM packing_list_items WHERE list_id = $1`, id); err != nil {
			return httperror.Internal(err)
		}
		for pos, l := range lines {
			if _, err := tx.Exec(ctx, `INSERT INTO packing_list_items (list_id, item_id, quantity, packed, note, position)
				VALUES ($1, $2, $3, $4, $5, $6)`, id, l.ItemID, l.Quantity, l.Packed, strings.TrimSpace(l.Note), pos); err != nil {
				return mapPgError(err, "packing list item", "")
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE packing_lists SET updated_at = now() WHERE id = $1`, id); err != nil {
			return httperror.Internal(err)
		}
		if out, err = s.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, sqlcgen.New(tx), "packing.items", "packing_list", id, before.Items, out.Items, "")
	})
	return out, err
}

// Delete removes a list (not any sign-out made from it).
func (s *PackingService) Delete(ctx context.Context, id uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM packing_lists WHERE id = $1`, id); err != nil {
			return httperror.Internal(err)
		}
		return audit.Record(ctx, sqlcgen.New(tx), "packing.delete", "packing_list", id, before, nil, "")
	})
}

// CreateSignout turns the list into a draft sign-out to its department (or
// toDept), one line per item. The draft is confirmed on the sign-out page,
// which checks availability again.
func (s *PackingService) CreateSignout(ctx context.Context, id uuid.UUID, toDept *uuid.UUID, purpose string) (SignoutWithItems, error) {
	pl, err := s.Get(ctx, id)
	if err != nil {
		return SignoutWithItems{}, err
	}
	if toDept == nil {
		toDept = pl.DepartmentID
	}
	if toDept == nil {
		return SignoutWithItems{}, httperror.Validation("choose the department the gear goes to", nil)
	}
	if len(pl.Items) == 0 {
		return SignoutWithItems{}, httperror.Validation("the packing list is empty", nil)
	}
	if strings.TrimSpace(purpose) == "" {
		purpose = pl.Name
	}
	in := CreateSignoutInput{ToDepartmentID: *toDept, Purpose: purpose, Notes: "from packing list " + pl.Name}
	for _, l := range pl.Items {
		in.Items = append(in.Items, SignoutItemInput{ItemID: l.ItemID, Quantity: l.Quantity, Notes: l.Note})
	}
	so, err := s.signouts.Create(ctx, in)
	if err != nil {
		return SignoutWithItems{}, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE packing_lists SET signout_id = $2, updated_at = now() WHERE id = $1`, id, so.Signout.ID); err != nil {
		return so, httperror.Internal(err)
	}
	return so, nil
}
