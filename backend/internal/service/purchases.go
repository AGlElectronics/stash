package service

import (
	"context"
	"strconv"
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

// Purchase is one purchase record of an item.
type Purchase struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	ItemID       uuid.UUID  `db:"item_id" json:"item_id"`
	PurchasedOn  string     `db:"purchased_on" json:"purchased_on"`
	Quantity     int32      `db:"quantity" json:"quantity"`
	UnitPrice    *string    `db:"unit_price" json:"unit_price,omitempty"`
	Currency     string     `db:"currency" json:"currency"`
	Supplier     string     `db:"supplier" json:"supplier"`
	Reference    string     `db:"reference" json:"reference"`
	AddedToStock bool       `db:"added_to_stock" json:"added_to_stock"`
	Note         string     `db:"note" json:"note"`
	CreatedBy    *uuid.UUID `db:"created_by" json:"created_by,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
}

const purchaseColumns = `id, item_id, to_char(purchased_on, 'YYYY-MM-DD') AS purchased_on, quantity,
	unit_price::text AS unit_price, currency, supplier, reference, added_to_stock, note, created_by, created_at`

// PurchaseService records what was bought for an item.
type PurchaseService struct{ pool *pgxpool.Pool }

func NewPurchaseService(pool *pgxpool.Pool) *PurchaseService { return &PurchaseService{pool: pool} }

// PurchaseInput is POST /api/items/{id}/purchases.
type PurchaseInput struct {
	PurchasedOn string `json:"purchased_on"`
	Quantity    int32  `json:"quantity"`
	UnitPrice   string `json:"unit_price"`
	Currency    string `json:"currency"`
	Supplier    string `json:"supplier"`
	Reference   string `json:"reference"`
	Note        string `json:"note"`
	// AddToStock raises the item's quantity_owned by Quantity (recorded as
	// an adjustment in its history).
	AddToStock bool `json:"add_to_stock"`
}

// List returns an item's purchases, newest first.
func (s *PurchaseService) List(ctx context.Context, itemID uuid.UUID) ([]Purchase, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+purchaseColumns+` FROM item_purchases WHERE item_id = $1 ORDER BY purchased_on DESC, created_at DESC`, itemID)
	if err != nil {
		return nil, httperror.Internal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[Purchase])
	if err != nil {
		return nil, httperror.Internal(err)
	}
	return out, nil
}

// Record saves a purchase and, with AddToStock, adds it to the stock.
func (s *PurchaseService) Record(ctx context.Context, itemID uuid.UUID, in PurchaseInput) (Purchase, error) {
	if in.Quantity <= 0 {
		return Purchase{}, httperror.Validation("quantity must be at least 1", nil)
	}
	on := time.Now().UTC().Format("2006-01-02")
	if strings.TrimSpace(in.PurchasedOn) != "" {
		if _, err := time.Parse("2006-01-02", in.PurchasedOn); err != nil {
			return Purchase{}, httperror.Validation("purchased_on must be a date like 2026-08-14", nil)
		}
		on = in.PurchasedOn
	}
	var price *string
	if p := strings.ReplaceAll(strings.TrimSpace(in.UnitPrice), ",", "."); p != "" {
		if f, err := strconv.ParseFloat(p, 64); err != nil || f < 0 {
			return Purchase{}, httperror.Validation("unit_price must be an amount like 12.50", nil)
		}
		price = &p
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "EUR"
	}
	if !currencyRE.MatchString(currency) {
		return Purchase{}, httperror.Validation("currency must be a three-letter code such as EUR", nil)
	}

	var out Purchase
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := sqlcgen.New(tx).GetItem(ctx, itemID); err != nil {
			return httperror.NotFound("item")
		}
		rows, err := tx.Query(ctx, `
			INSERT INTO item_purchases (item_id, purchased_on, quantity, unit_price, currency, supplier, reference, added_to_stock, note, created_by)
			VALUES ($1, $2, $3, $4::numeric, $5, $6, $7, $8, $9, $10)
			RETURNING `+purchaseColumns,
			itemID, on, in.Quantity, price, currency, strings.TrimSpace(in.Supplier), strings.TrimSpace(in.Reference),
			in.AddToStock, strings.TrimSpace(in.Note), auth.ActorID(ctx))
		if err != nil {
			return httperror.Internal(err)
		}
		out, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[Purchase])
		if err != nil {
			return httperror.Internal(err)
		}
		if in.AddToStock {
			if _, err := adjustInTx(ctx, tx, itemID, AdjustInput{Delta: in.Quantity, Note: "purchase " + out.Reference}); err != nil {
				return err
			}
		}
		return audit.Record(ctx, sqlcgen.New(tx), "item.purchase", "item", itemID, nil, out, "")
	})
	return out, err
}
