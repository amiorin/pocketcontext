package sqlread

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func TestClassifyBusy(t *testing.T) {
	for _, code := range []sqlite3.ErrNo{sqlite3.ErrBusy, sqlite3.ErrLocked} {
		err := classify(fmt.Errorf("prepare: %w", sqlite3.Error{Code: code}))
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("code %d not classified as busy: %v", code, err)
		}
	}
	plain := errors.New("no such table: x")
	if err := classify(plain); err != plain || errors.Is(err, ErrBusy) {
		t.Fatalf("plain error altered: %v", err)
	}
	if classify(nil) != nil {
		t.Fatal("nil error altered")
	}
}

func TestHeavyQueryIsBounded(t *testing.T) {
	e, _ := fixture(t, Config{Timeout: 500 * time.Millisecond})
	ctx := context.Background()
	_, err := e.Query(ctx, `WITH RECURSIVE x(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM x WHERE n<3000000) SELECT group_concat(n) FROM x`)
	if err == nil {
		t.Fatal("heavy query succeeded; expected size limit or deadline")
	}
	if errors.Is(err, ErrBusy) {
		t.Fatalf("heavy query misclassified as busy: %v", err)
	}
	if _, err = e.Query(ctx, `SELECT 1`); err != nil {
		t.Fatalf("connection unusable after heavy query: %v", err)
	}
	if _, err = e.Query(ctx, `PRAGMA query_only`); err == nil {
		t.Fatal("PRAGMA reachable through the engine")
	}
}
