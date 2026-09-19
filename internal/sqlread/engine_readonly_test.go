package sqlread

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// VACUUM passes the authorizer, so only the read-only check stops it.
func TestNonReadOnlyStatementsRejected(t *testing.T) {
	e, _ := fixture(t, Config{})
	ctx := context.Background()
	out := filepath.Join(t.TempDir(), "copy.db")
	// More iterations than pooled connections: a rejection must release its connection.
	for i := 0; i < 8; i++ {
		for _, query := range []string{`VACUUM INTO '` + out + `'`, `EXPLAIN VACUUM INTO '` + out + `'`, `VACUUM`, `EXPLAIN VACUUM`, `VACUUM; -- comment`} {
			if _, err := e.Query(ctx, query); !errors.Is(err, ErrNotReadOnly) {
				t.Fatalf("%q: expected ErrNotReadOnly, got %v", query, err)
			}
		}
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("VACUUM INTO wrote %s: %v", out, err)
	}
	if _, err := e.Query(ctx, `SELECT 1`); err != nil {
		t.Fatalf("engine unusable after rejections: %v", err)
	}
	if _, err := e.db.BeginTx(ctx, nil); err == nil {
		t.Fatal("transaction opened on reader")
	}
	if _, err := e.db.ExecContext(ctx, `VACUUM`); !errors.Is(err, ErrNotReadOnly) {
		t.Fatalf("direct exec bypassed the read-only check: %v", err)
	}
}

// EXPLAIN of a read-only statement is read-only and stays allowed.
func TestReadOnlyStatementsPass(t *testing.T) {
	e, _ := fixture(t, Config{})
	for _, query := range []string{
		`EXPLAIN SELECT 1`,
		`EXPLAIN QUERY PLAN SELECT name FROM people`,
		`SELECT name FROM people WHERE id='p1'`,
		`WITH c AS (SELECT person, value FROM deals) SELECT person, sum(value) FROM c GROUP BY person`,
		`SELECT id, sum(value) OVER (PARTITION BY person ORDER BY value) FROM deals`,
	} {
		t.Run(query, func(t *testing.T) {
			r, err := e.Query(context.Background(), query)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Rows) == 0 {
				t.Fatal("no rows")
			}
		})
	}
	if _, err := e.Query(context.Background(), `EXPLAIN SELECT token FROM private`); err == nil || errors.Is(err, ErrNotReadOnly) {
		t.Fatalf("EXPLAIN bypassed the authorizer: %v", err)
	}
}
