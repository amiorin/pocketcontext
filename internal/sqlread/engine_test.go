package sqlread

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, cfg Config) (*Engine, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE people(id TEXT PRIMARY KEY,name TEXT,secret TEXT);CREATE TABLE deals(id TEXT,person TEXT,value INTEGER);CREATE TABLE private(token TEXT);INSERT INTO people VALUES('p1','Ada','secret');INSERT INTO deals VALUES('d1','p1',42),('d2','p1',58);INSERT INTO private VALUES('secret');CREATE VIEW leak AS SELECT * FROM private;`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tables == nil {
		cfg.Tables = map[string][]string{"people": {"id", "name"}, "deals": nil}
	}
	e, err := New(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	return e, db
}
func TestReads(t *testing.T) {
	e, db := fixture(t, Config{})
	ctx := context.Background()
	for _, query := range []string{
		`SELECT p.name,sum(d.value) FROM people p JOIN deals d ON p.id=d.person GROUP BY p.id`,
		`WITH c AS (SELECT value FROM deals) SELECT sum(value) FROM c`,
		`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n<10) SELECT sum(n) FROM seq`,
		`SELECT count(*) FROM people`,
		`SELECT ';', 'it''s;fine', 1 AS "semi;colon"; -- comment`,
		`SELECT value, row_number() OVER (ORDER BY value) FROM deals`,
		`SELECT sqrt(4), ceil(1.5), pow(2,3), mod(5,2)`,
		`SELECT '{"a":1}' ->> '$.a'`,
	} {
		t.Run(query, func(t *testing.T) {
			r, err := e.Query(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Rows) == 0 {
				t.Fatal("no rows")
			}
		})
	}
	if _, err := db.Exec(`INSERT INTO people VALUES('p2','Grace','hidden')`); err != nil {
		t.Fatal(err)
	}
	r, err := e.Query(ctx, `SELECT name FROM people ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) != 2 || r.Rows[1][0] != "Grace" {
		t.Fatalf("new commit invisible: %#v", r)
	}
	schema, err := e.Schema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema) != 2 || len(schema[1].Columns) != 2 {
		t.Fatalf("wrong schema %#v", schema)
	}
}
func TestDeniedQueries(t *testing.T) {
	e, _ := fixture(t, Config{})
	for _, query := range []string{
		`SELECT * FROM private`, `SELECT count(*) FROM private`, `SELECT secret FROM people`, `SELECT * FROM people`,
		`SELECT token FROM private UNION SELECT name FROM people`, `WITH x AS (SELECT token FROM private) SELECT * FROM x`,
		`SELECT (SELECT token FROM private) FROM people`, `SELECT name FROM people WHERE EXISTS(SELECT 1 FROM private)`,
		`SELECT * FROM sqlite_schema`, `SELECT * FROM leak`, `SELECT * FROM pragma_table_info('private')`, `PRAGMA table_info(people)`, `PRAGMA query_only=OFF`,
		`INSERT INTO people VALUES('p2','bad','bad') RETURNING name`, `UPDATE people SET name='bad' RETURNING name`, `DELETE FROM deals RETURNING id`,
		`CREATE TABLE bad(x)`, `DROP TABLE deals`, `ATTACH ':memory:' AS other`, `BEGIN`, `COMMIT`, `VACUUM`, `ANALYZE`,
		`SELECT load_extension('foo')`, `SELECT randomblob(1000000000)`, `SELECT readfile('/etc/passwd')`,
		`SELECT 1; SELECT 2`, `SELECT 1; /*x*/ DELETE FROM deals`, `SELECT 1;;`, `SELECT 1; ATTACH ':memory:' AS x`,
		"SELECT 1\x00; DELETE FROM deals", `-- empty`, `SELECT 'unterminated`, `SELECT 1 /*unterminated`,
	} {
		t.Run(query, func(t *testing.T) {
			if _, err := e.Query(context.Background(), query); err == nil {
				t.Fatal("query unexpectedly permitted")
			}
		})
	}
}
func TestLimitsAndCancellation(t *testing.T) {
	e, _ := fixture(t, Config{MaxRows: 3, MaxBytes: 512, Timeout: 20 * time.Millisecond})
	r, err := e.Query(context.Background(), `WITH RECURSIVE x(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM x WHERE n<100) SELECT n FROM x`)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Truncated || len(r.Rows) != 3 {
		t.Fatalf("row cap failed: %#v", r)
	}
	if _, err = e.Query(context.Background(), `WITH RECURSIVE x(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM x) SELECT sum(n) FROM x`); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unbounded recursion did not return deadline: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = e.Query(ctx, `SELECT 1`); err == nil {
		t.Fatal("cancel ignored")
	}
	if _, err = e.Query(context.Background(), `SELECT printf('%1000000000s','x')`); err == nil {
		t.Fatal("huge scalar allowed")
	}
	if _, err = e.Query(context.Background(), `SELECT 1`); err != nil {
		t.Fatalf("connection unusable after interrupt: %v", err)
	}
	e2, _ := fixture(t, Config{MaxBytes: 128})
	r, err = e2.Query(context.Background(), `WITH RECURSIVE x(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM x WHERE n<100) SELECT n,'abcdefghij' FROM x`)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(r)
	if !r.Truncated || len(data) > 128 {
		t.Fatalf("byte cap failed: %s", data)
	}
}
func TestSchemaConfiguration(t *testing.T) {
	e, db := fixture(t, Config{})
	_ = e
	var path string
	var seq int
	var name string
	if err := db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	for _, tables := range []map[string][]string{nil, {"private": {"missing"}}, {"sqlite_schema": nil}, {"leak": nil}, {"missing": nil}} {
		if engine, err := New(path, Config{Tables: tables}); err == nil {
			engine.Close()
			t.Fatalf("accepted bad config %v", tables)
		}
	}
}
func TestCSV(t *testing.T) {
	r := Result{Columns: []string{"name", "value"}, Rows: [][]any{{"a,b", nil}, {"line\nbreak", int64(3)}}}
	data, err := r.CSV()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"a,b",`) {
		t.Fatalf("invalid CSV: %s", data)
	}
}
