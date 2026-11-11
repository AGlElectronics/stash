package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/httperror"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

func mustDept(t *testing.T, pool *pgxpool.Pool, name string) sqlcgen.Department {
	t.Helper()
	d, err := sqlcgen.New(pool).CreateDepartment(context.Background(), sqlcgen.CreateDepartmentParams{Name: name})
	if err != nil {
		t.Fatalf("create department %q: %v", name, err)
	}
	return d
}

type itemOpt func(*sqlcgen.CreateItemParams)

func withAssetTag(tag string) itemOpt {
	return func(p *sqlcgen.CreateItemParams) { p.AssetTag = &tag }
}

func withManagedBy(id uuid.UUID) itemOpt {
	return func(p *sqlcgen.CreateItemParams) { p.ManagedByDepartmentID = &id }
}

func mustItem(t *testing.T, pool *pgxpool.Pool, name string, qty int32, opts ...itemOpt) sqlcgen.Item {
	t.Helper()
	p := sqlcgen.CreateItemParams{
		Name:             name,
		ItemType:         "asset",
		QuantityOwned:    qty,
		Unit:             "pcs",
		Condition:        "good",
		Status:           "available",
		Owner:            "organization",
		PurchaseCurrency: "EUR",
	}
	for _, o := range opts {
		o(&p)
	}
	it, err := sqlcgen.New(pool).CreateItem(context.Background(), p)
	if err != nil {
		t.Fatalf("create item %q: %v", name, err)
	}
	return it
}

func mustGetItem(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) sqlcgen.Item {
	t.Helper()
	it, err := sqlcgen.New(pool).GetItem(context.Background(), id)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	return it
}

// wantCode fails unless err is an *httperror.Error with the given code.
func wantCode(t *testing.T, err error, code httperror.Code) {
	t.Helper()
	var he *httperror.Error
	if !errors.As(err, &he) {
		t.Fatalf("want %s error, got %v", code, err)
	}
	if he.Code != code {
		t.Fatalf("want %s error, got %s: %s", code, he.Code, he.Message)
	}
}

func countRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
