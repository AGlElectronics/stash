package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/audit"
	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// Request statuses. open → approved → ordered → fulfilled, or rejected;
// the person who asked can cancel while it is open.
var requestStatuses = []string{"open", "approved", "ordered", "fulfilled", "rejected", "cancelled"}

// ItemRequest is someone asking logistics for gear.
type ItemRequest struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	Title           string     `db:"title" json:"title"`
	ItemID          *uuid.UUID `db:"item_id" json:"item_id,omitempty"`
	ItemName        *string    `db:"item_name" json:"item_name,omitempty"`
	Quantity        int32      `db:"quantity" json:"quantity"`
	DepartmentID    *uuid.UUID `db:"department_id" json:"department_id,omitempty"`
	DepartmentName  *string    `db:"department_name" json:"department_name,omitempty"`
	NeededBy        *string    `db:"needed_by" json:"needed_by,omitempty"`
	Reason          string     `db:"reason" json:"reason"`
	Status          string     `db:"status" json:"status"`
	RequestedBy     *uuid.UUID `db:"requested_by" json:"requested_by,omitempty"`
	RequestedByName *string    `db:"requested_by_name" json:"requested_by_name,omitempty"`
	DecidedAt       *time.Time `db:"decided_at" json:"decided_at,omitempty"`
	DecisionNote    string     `db:"decision_note" json:"decision_note"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
}

const requestSelect = `SELECT r.id, r.title, r.item_id, i.name AS item_name, r.quantity, r.department_id,
	d.name AS department_name, to_char(r.needed_by, 'YYYY-MM-DD') AS needed_by, r.reason, r.status,
	r.requested_by, u.display_name AS requested_by_name, r.decided_at, r.decision_note, r.created_at, r.updated_at
	FROM item_requests r
	LEFT JOIN items i ON i.id = r.item_id
	LEFT JOIN departments d ON d.id = r.department_id
	LEFT JOIN users u ON u.id = r.requested_by `

// RequestService handles item requests.
type RequestService struct{ pool *pgxpool.Pool }

func NewRequestService(pool *pgxpool.Pool) *RequestService { return &RequestService{pool: pool} }

// RequestInput is POST /api/requests.
type RequestInput struct {
	Title        string     `json:"title"`
	ItemID       *uuid.UUID `json:"item_id,omitempty"`
	Quantity     int32      `json:"quantity"`
	DepartmentID *uuid.UUID `json:"department_id,omitempty"`
	NeededBy     string     `json:"needed_by"`
	Reason       string     `json:"reason"`
}

// List returns requests, open ones first; status "" means all.
func (s *RequestService) List(ctx context.Context, status string) ([]ItemRequest, error) {
	if status != "" && !slices.Contains(requestStatuses, status) {
		return nil, httperror.Validation("status must be one of "+strings.Join(requestStatuses, ", "), nil)
	}
	rows, err := s.pool.Query(ctx, requestSelect+`WHERE ($1 = '' OR r.status = $1)
		ORDER BY (r.status IN ('open','approved','ordered')) DESC, r.needed_by NULLS LAST, r.created_at DESC LIMIT 500`, status)
	if err != nil {
		return nil, httperror.Internal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[ItemRequest])
	if err != nil {
		return nil, httperror.Internal(err)
	}
	return out, nil
}

func (s *RequestService) get(ctx context.Context, q pgx.Tx, id uuid.UUID) (ItemRequest, error) {
	rows, err := q.Query(ctx, requestSelect+`WHERE r.id = $1`, id)
	if err != nil {
		return ItemRequest{}, httperror.Internal(err)
	}
	r, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[ItemRequest])
	if errors.Is(err, pgx.ErrNoRows) {
		return ItemRequest{}, httperror.NotFound("request")
	}
	if err != nil {
		return ItemRequest{}, httperror.Internal(err)
	}
	return r, nil
}

// Create files a request.
func (s *RequestService) Create(ctx context.Context, in RequestInput) (ItemRequest, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" && in.ItemID == nil {
		return ItemRequest{}, httperror.Validation("say what you need: a title or an item", nil)
	}
	if in.Quantity == 0 {
		in.Quantity = 1
	}
	if in.Quantity < 0 {
		return ItemRequest{}, httperror.Validation("quantity must be at least 1", nil)
	}
	var neededBy *string
	if in.NeededBy != "" {
		if _, err := time.Parse("2006-01-02", in.NeededBy); err != nil {
			return ItemRequest{}, httperror.Validation("needed_by must be a date like 2026-08-14", nil)
		}
		neededBy = &in.NeededBy
	}
	var out ItemRequest
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if in.ItemID != nil && in.Title == "" {
			it, err := sqlcgen.New(tx).GetItem(ctx, *in.ItemID)
			if err != nil {
				return httperror.Validation("item not found", nil)
			}
			in.Title = it.Name
		}
		var id uuid.UUID
		err := tx.QueryRow(ctx, `INSERT INTO item_requests (title, item_id, quantity, department_id, needed_by, reason, requested_by)
			VALUES ($1, $2, $3, $4, $5::date, $6, $7) RETURNING id`,
			in.Title, in.ItemID, in.Quantity, in.DepartmentID, neededBy, strings.TrimSpace(in.Reason), auth.ActorID(ctx)).Scan(&id)
		if err != nil {
			return mapPgError(err, "request", "")
		}
		if out, err = s.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, sqlcgen.New(tx), "request.create", "request", id, nil, out, "")
	})
	return out, err
}

// Decide moves a request to a new status. request.manage may set any
// status; the person who asked may only cancel their own open request.
func (s *RequestService) Decide(ctx context.Context, id uuid.UUID, status, note string) (ItemRequest, error) {
	if !slices.Contains(requestStatuses, status) || status == "open" {
		return ItemRequest{}, httperror.Validation("status must be approved, ordered, fulfilled, rejected or cancelled", nil)
	}
	var out ItemRequest
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		before, err := s.get(ctx, tx, id)
		if err != nil {
			return err
		}
		manager := auth.HasPermission(ctx, domain.PermRequestManage)
		own := before.RequestedBy != nil && auth.ActorID(ctx) != nil && *before.RequestedBy == *auth.ActorID(ctx)
		switch {
		case manager:
		case own && status == "cancelled" && before.Status == "open":
		default:
			return httperror.New(httperror.CodeForbidden, "only logistics (request.manage) can decide requests; you can cancel your own open request")
		}
		if before.Status == "fulfilled" || before.Status == "rejected" || before.Status == "cancelled" {
			return httperror.Conflict(fmt.Sprintf("the request is already %s", before.Status))
		}
		if _, err := tx.Exec(ctx, `UPDATE item_requests SET status = $2, decision_note = $3, decided_by = $4,
			decided_at = now(), updated_at = now() WHERE id = $1`, id, status, strings.TrimSpace(note), auth.ActorID(ctx)); err != nil {
			return httperror.Internal(err)
		}
		if out, err = s.get(ctx, tx, id); err != nil {
			return err
		}
		return audit.Record(ctx, sqlcgen.New(tx), "request."+status, "request", id, before, out, note)
	})
	return out, err
}
