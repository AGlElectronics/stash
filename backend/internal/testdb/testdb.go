// Package testdb gives database-backed tests their own freshly migrated
// PostgreSQL schema.
//
// Tests run only when STASH_TEST_DATABASE_URL is set, for example:
//
//	STASH_TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/stash_test?sslmode=disable go test ./...
//
// Every call to New creates a schema named t_<random> in that database, runs
// all migrations into it and drops it when the test ends, so tests never see
// each other's rows and need no CREATEDB privilege.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/db"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/repository/sqlcgen"
)

// EnvVar names the connection string used by database tests.
const EnvVar = "STASH_TEST_DATABASE_URL"

// goose keeps package-level state, so migrations run one at a time.
var migrateMu sync.Mutex

// New returns a pool bound to a new, migrated schema. It skips the test
// when STASH_TEST_DATABASE_URL is not set.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv(EnvVar)
	if base == "" {
		t.Skip(EnvVar + " not set; skipping database test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	admin, err := db.Open(ctx, base)
	if err != nil {
		t.Fatalf("testdb: connect: %v", err)
	}
	schema := "t_" + randomHex(6)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("testdb: create schema: %v", err)
	}
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		_, _ = admin.Exec(cctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		admin.Close()
	})

	pool, err := db.Open(ctx, withSearchPath(t, base, schema))
	if err != nil {
		t.Fatalf("testdb: connect to schema: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate(ctx, admin, pool); err != nil {
		t.Fatalf("testdb: migrate: %v", err)
	}
	return pool
}

// migrate runs the migrations while holding an advisory lock, because test
// packages run as parallel processes and CREATE EXTENSION is not safe to
// run concurrently.
func migrate(ctx context.Context, admin, pool *pgxpool.Pool) error {
	migrateMu.Lock()
	defer migrateMu.Unlock()
	conn, err := admin.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	const lockKey = 0x57a5_7e57
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockKey) }()
	return db.MigrateUp(ctx, pool)
}

func withSearchPath(t testing.TB, dsn, schema string) string {
	t.Helper()
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("testdb: parse %s: %v", EnvVar, err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		return u.String()
	}
	return dsn + " search_path=" + schema
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Actor inserts a user and returns a context authenticated as that user
// with the given global permissions. With no permissions the user gets
// every permission (admin).
func Actor(t testing.TB, pool *pgxpool.Pool, name string, perms ...string) context.Context {
	t.Helper()
	ctx := context.Background()
	subject := "test:" + name + ":" + randomHex(4)
	row, err := sqlcgen.New(pool).UpsertUserBySubject(ctx, sqlcgen.UpsertUserBySubjectParams{
		OidcSubject: &subject,
		DisplayName: name,
	})
	if err != nil {
		t.Fatalf("testdb: create user: %v", err)
	}
	if len(perms) == 0 {
		perms = domain.AllPermissions()
	}
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	return auth.WithUser(ctx, &auth.User{
		ID:          row.ID,
		OIDCSubject: subject,
		DisplayName: name,
		Permissions: set,
	})
}
