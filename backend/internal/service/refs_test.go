package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// --- mocks ---

type mockDeptResolver struct {
	getByName    func(ctx context.Context, name string) (sqlcgen.Department, error)
	createDept   func(ctx context.Context, arg sqlcgen.CreateDepartmentParams) (sqlcgen.Department, error)
	getCalls     int
	createCalls  int
}

func (m *mockDeptResolver) GetDepartmentByName(ctx context.Context, name string) (sqlcgen.Department, error) {
	m.getCalls++
	return m.getByName(ctx, name)
}

func (m *mockDeptResolver) CreateDepartment(ctx context.Context, arg sqlcgen.CreateDepartmentParams) (sqlcgen.Department, error) {
	m.createCalls++
	return m.createDept(ctx, arg)
}

type mockCategoryResolver struct {
	getByName   func(ctx context.Context, name string) (sqlcgen.ItemCategory, error)
	createCat   func(ctx context.Context, arg sqlcgen.CreateCategoryParams) (sqlcgen.ItemCategory, error)
	getCalls    int
	createCalls int
}

func (m *mockCategoryResolver) GetCategoryByName(ctx context.Context, name string) (sqlcgen.ItemCategory, error) {
	m.getCalls++
	return m.getByName(ctx, name)
}

func (m *mockCategoryResolver) CreateCategory(ctx context.Context, arg sqlcgen.CreateCategoryParams) (sqlcgen.ItemCategory, error) {
	m.createCalls++
	return m.createCat(ctx, arg)
}

type mockLocationResolver struct {
	getByName   func(ctx context.Context, name string) (sqlcgen.Location, error)
	createLoc   func(ctx context.Context, arg sqlcgen.CreateLocationParams) (sqlcgen.Location, error)
	getCalls    int
	createCalls int
}

func (m *mockLocationResolver) GetLocationByName(ctx context.Context, name string) (sqlcgen.Location, error) {
	m.getCalls++
	return m.getByName(ctx, name)
}

func (m *mockLocationResolver) CreateLocation(ctx context.Context, arg sqlcgen.CreateLocationParams) (sqlcgen.Location, error) {
	m.createCalls++
	return m.createLoc(ctx, arg)
}

// --- tests ---

func TestResolveDepartmentRefTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns id when already set", func(t *testing.T) {
		t.Parallel()
		existing := uuid.New()
		mock := &mockDeptResolver{}
		got, err := resolveDepartmentRefTx(ctx, mock, &existing, "ignored")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != existing {
			t.Fatalf("got %v, want %v", *got, existing)
		}
		if mock.getCalls != 0 || mock.createCalls != 0 {
			t.Fatal("should not query DB when id is provided")
		}
	})

	t.Run("returns nil for empty name and nil id", func(t *testing.T) {
		t.Parallel()
		mock := &mockDeptResolver{}
		got, err := resolveDepartmentRefTx(ctx, mock, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("returns nil for whitespace-only name", func(t *testing.T) {
		t.Parallel()
		mock := &mockDeptResolver{}
		got, err := resolveDepartmentRefTx(ctx, mock, nil, "   ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("finds existing department by name", func(t *testing.T) {
		t.Parallel()
		existingID := uuid.New()
		mock := &mockDeptResolver{
			getByName: func(_ context.Context, name string) (sqlcgen.Department, error) {
				if name != "Engineering" {
					t.Fatalf("expected trimmed name 'Engineering', got %q", name)
				}
				return sqlcgen.Department{ID: existingID, Name: "Engineering"}, nil
			},
		}
		got, err := resolveDepartmentRefTx(ctx, mock, nil, "  Engineering  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != existingID {
			t.Fatalf("got %v, want %v", *got, existingID)
		}
		if mock.createCalls != 0 {
			t.Fatal("should not create when name exists")
		}
	})

	t.Run("creates new department when not found", func(t *testing.T) {
		t.Parallel()
		newID := uuid.New()
		mock := &mockDeptResolver{
			getByName: func(_ context.Context, _ string) (sqlcgen.Department, error) {
				return sqlcgen.Department{}, pgx.ErrNoRows
			},
			createDept: func(_ context.Context, arg sqlcgen.CreateDepartmentParams) (sqlcgen.Department, error) {
				if arg.Name != "New Dept" {
					t.Fatalf("expected 'New Dept', got %q", arg.Name)
				}
				if arg.Description != "" {
					t.Fatalf("expected empty description, got %q", arg.Description)
				}
				return sqlcgen.Department{ID: newID, Name: arg.Name}, nil
			},
		}
		got, err := resolveDepartmentRefTx(ctx, mock, nil, "New Dept")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != newID {
			t.Fatalf("got %v, want %v", *got, newID)
		}
		if mock.getCalls != 1 || mock.createCalls != 1 {
			t.Fatalf("expected 1 get + 1 create, got %d/%d", mock.getCalls, mock.createCalls)
		}
	})

	t.Run("handles race condition on create (retry get)", func(t *testing.T) {
		t.Parallel()
		raceID := uuid.New()
		getCall := 0
		mock := &mockDeptResolver{
			getByName: func(_ context.Context, _ string) (sqlcgen.Department, error) {
				getCall++
				if getCall == 1 {
					return sqlcgen.Department{}, pgx.ErrNoRows
				}
				return sqlcgen.Department{ID: raceID, Name: "Racing"}, nil
			},
			createDept: func(_ context.Context, _ sqlcgen.CreateDepartmentParams) (sqlcgen.Department, error) {
				return sqlcgen.Department{}, &pgconn.PgError{Code: "23505", Message: "unique_violation"}
			},
		}
		got, err := resolveDepartmentRefTx(ctx, mock, nil, "Racing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != raceID {
			t.Fatalf("got %v, want %v", *got, raceID)
		}
	})

	t.Run("propagates internal error from GetDepartmentByName", func(t *testing.T) {
		t.Parallel()
		mock := &mockDeptResolver{
			getByName: func(_ context.Context, _ string) (sqlcgen.Department, error) {
				return sqlcgen.Department{}, pgx.ErrTxClosed
			},
		}
		_, err := resolveDepartmentRefTx(ctx, mock, nil, "test")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestResolveCategoryRefTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns id when set", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		mock := &mockCategoryResolver{}
		got, err := resolveCategoryRefTx(ctx, mock, &id, "ignored")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != id {
			t.Fatalf("got %v, want %v", *got, id)
		}
	})

	t.Run("returns nil for empty name", func(t *testing.T) {
		t.Parallel()
		mock := &mockCategoryResolver{}
		got, err := resolveCategoryRefTx(ctx, mock, nil, "  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("finds existing category", func(t *testing.T) {
		t.Parallel()
		catID := uuid.New()
		mock := &mockCategoryResolver{
			getByName: func(_ context.Context, _ string) (sqlcgen.ItemCategory, error) {
				return sqlcgen.ItemCategory{ID: catID, Name: "Electronics"}, nil
			},
		}
		got, err := resolveCategoryRefTx(ctx, mock, nil, "Electronics")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != catID {
			t.Fatalf("got %v, want %v", *got, catID)
		}
	})

	t.Run("creates when not found", func(t *testing.T) {
		t.Parallel()
		newID := uuid.New()
		mock := &mockCategoryResolver{
			getByName: func(_ context.Context, _ string) (sqlcgen.ItemCategory, error) {
				return sqlcgen.ItemCategory{}, pgx.ErrNoRows
			},
			createCat: func(_ context.Context, arg sqlcgen.CreateCategoryParams) (sqlcgen.ItemCategory, error) {
				return sqlcgen.ItemCategory{ID: newID, Name: arg.Name}, nil
			},
		}
		got, err := resolveCategoryRefTx(ctx, mock, nil, "Furniture")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != newID {
			t.Fatalf("got %v, want %v", *got, newID)
		}
	})
}

func TestResolveLocationRefTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns id when set", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		mock := &mockLocationResolver{}
		got, err := resolveLocationRefTx(ctx, mock, &id, "ignored")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != id {
			t.Fatalf("got %v, want %v", *got, id)
		}
	})

	t.Run("returns nil for empty name", func(t *testing.T) {
		t.Parallel()
		mock := &mockLocationResolver{}
		got, err := resolveLocationRefTx(ctx, mock, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("finds existing location", func(t *testing.T) {
		t.Parallel()
		locID := uuid.New()
		mock := &mockLocationResolver{
			getByName: func(_ context.Context, _ string) (sqlcgen.Location, error) {
				return sqlcgen.Location{ID: locID, Name: "Warehouse A"}, nil
			},
		}
		got, err := resolveLocationRefTx(ctx, mock, nil, " Warehouse A ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != locID {
			t.Fatalf("got %v, want %v", *got, locID)
		}
	})

	t.Run("creates when not found with nil parent", func(t *testing.T) {
		t.Parallel()
		newID := uuid.New()
		mock := &mockLocationResolver{
			getByName: func(_ context.Context, _ string) (sqlcgen.Location, error) {
				return sqlcgen.Location{}, pgx.ErrNoRows
			},
			createLoc: func(_ context.Context, arg sqlcgen.CreateLocationParams) (sqlcgen.Location, error) {
				if arg.ParentID != nil {
					t.Fatal("expected nil parent_id for new location")
				}
				return sqlcgen.Location{ID: newID, Name: arg.Name}, nil
			},
		}
		got, err := resolveLocationRefTx(ctx, mock, nil, "New Room")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *got != newID {
			t.Fatalf("got %v, want %v", *got, newID)
		}
	})
}

