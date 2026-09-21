package sqlread

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// Commit a policy and data change during the first export, after the source
// snapshot has been acquired. Both exported tables must retain the old policy
// and data, even though the next request must see the new authorization.
func TestSnapshotPolicyAndDataShareReadTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`PRAGMA journal_mode=WAL;
		CREATE TABLE alpha (id TEXT, amount INTEGER);
		CREATE TABLE beta (id TEXT, amount INTEGER);
		CREATE TABLE grants (requester TEXT);
		INSERT INTO alpha VALUES ('alpha', 10);
		INSERT INTO beta VALUES ('beta', 10);
		INSERT INTO grants VALUES ('alice');`)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSnapshotSource(path, Config{
		// Reversed projection order also checks the destination insertion mapping.
		Tables: map[string][]string{"alpha": {"amount", "id"}, "beta": {"id", "amount"}},
	}, SnapshotConfig{
		PolicyTables: map[string][]string{"grants": {"requester"}},
		Filters: map[string]string{
			"alpha": "length(:requester) > 0 AND EXISTS (SELECT 1 FROM grants WHERE grants.requester = :requester)",
			"beta":  "EXISTS (SELECT 1 FROM grants WHERE grants.requester = :requester)",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.source.db.SetMaxOpenConns(1)
	conn, err := source.source.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var changed sync.Once
	var writeErr error
	err = conn.Raw(func(raw any) error {
		// A test-only callback synchronizes the writer without sleeps or races.
		return raw.(roConn).c.RegisterFunc("length", func(value string) (int, error) {
			changed.Do(func() {
				_, writeErr = db.Exec(`BEGIN;
					UPDATE grants SET requester = 'bob';
					UPDATE alpha SET amount = 20;
					UPDATE beta SET amount = 20;
					COMMIT;`)
			})
			return len(value), writeErr
		}, false)
	})
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	query := `SELECT alpha.id, alpha.amount, beta.id, beta.amount FROM alpha CROSS JOIN beta`
	result, _, err := source.Query(context.Background(), "alice", query)
	if err != nil {
		t.Fatalf("initial snapshot: %v (writer: %v)", err, writeErr)
	}
	if want := [][]any{{"alpha", int64(10), "beta", int64(10)}}; !reflect.DeepEqual(result.Rows, want) {
		t.Fatalf("mixed policy or data versions: got %#v, want %#v", result.Rows, want)
	}
	result, _, err = source.Query(context.Background(), "alice", query)
	if err != nil || len(result.Rows) != 0 {
		t.Fatalf("revoked requester: %#v, %v", result.Rows, err)
	}
	result, _, err = source.Query(context.Background(), "bob", query)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]any{{"alpha", int64(20), "beta", int64(20)}}; !reflect.DeepEqual(result.Rows, want) {
		t.Fatalf("new snapshot: got %#v, want %#v", result.Rows, want)
	}
}
