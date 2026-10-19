package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, e := openDB(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(db.Close)
	if e = migrate(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	return db
}
func TestBackupRestore(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	if _, e := db.Exec(ctx, "INSERT INTO organizations(name) VALUES(?1)", "Portable team"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "portable.db")
	if e := db.Backup(ctx, path); e != nil {
		t.Fatal(e)
	}
	if e := db.Backup(ctx, path); e == nil {
		t.Fatal("Backup overwrote existing file")
	}
	restored, e := openDB(path)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	if e = migrate(ctx, restored); e != nil {
		t.Fatal(e)
	}
	var name string
	if e = restored.QueryRow(ctx, "SELECT name FROM organizations").Scan(&name); e != nil || name != "Portable team" {
		t.Fatal("Backup lost data", name, e)
	}
	var fk int
	restored.raw.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Fatal("Foreign keys not enabled")
	}
}
func exportForTest(t *testing.T, db *DB) map[string][]map[string]any {
	t.Helper()
	ctx := context.Background()
	data := map[string][]map[string]any{}
	owner := &DB{raw: db.raw, columns: db.columns, admin: true}
	for _, table := range importTables {
		rows, e := owner.QueryMaps(ctx, "SELECT * FROM "+table)
		if e != nil {
			t.Fatal(e)
		}
		data[table] = rows
	}
	return data
}
func writeExport(t *testing.T, data map[string][]map[string]any) string {
	t.Helper()
	b, e := json.Marshal(data)
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "export.json")
	if e = os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestImportAtomicAndComplete(t *testing.T) {
	ctx := context.Background()
	source := newTestDB(t)
	var org string
	source.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('Imported team') RETURNING id").Scan(&org)
	data := exportForTest(t, source)
	dest := newTestDB(t)
	file := writeExport(t, data)
	if e := importPostgres(ctx, dest, file); e != nil {
		t.Fatal(e)
	}
	if e := importPostgres(ctx, dest, file); e == nil {
		t.Fatal("Import overwrote populated database")
	}
	bad := newTestDB(t)
	data["memberships"] = []map[string]any{{"org_id": org, "user_id": "missing", "role": "admin"}}
	if e := importPostgres(ctx, bad, writeExport(t, data)); e == nil {
		t.Fatal("Broken import accepted")
	}
	var n int
	bad.QueryRow(ctx, "SELECT count(*) FROM organizations").Scan(&n)
	if n != 0 {
		t.Fatal("Failed import left partial data")
	}
}
func TestConcurrentMagicLinkSingleUse(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	var org, user string
	db.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('Concurrent team') RETURNING id").Scan(&org)
	db.QueryRow(ctx, "INSERT INTO users(email) VALUES('admin@resourcemgmt.com') RETURNING id").Scan(&user)
	db.Exec(ctx, "INSERT INTO memberships VALUES(?1,?2,'admin')", org, user)
	tok := token()
	db.Exec(ctx, "INSERT INTO login_tokens(hash,email,expires_at) VALUES(?1,'admin@resourcemgmt.com',strftime('%Y-%m-%dT%H:%M:%fZ','now','+10 minutes'))", hash(tok))
	app := &App{db: db, origin: "http://localhost:3000"}
	h := app.handler()
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodPost, "/auth/verify", strings.NewReader(fmt.Sprintf(`{"token":%q}`, tok)))
			r.Header.Set("Origin", app.origin)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	wg.Wait()
	close(codes)
	good, bad := 0, 0
	for code := range codes {
		if code == 200 {
			good++
		} else if code == 401 {
			bad++
		} else {
			t.Fatal("Unexpected verification status", code)
		}
	}
	if good != 1 || bad != 1 {
		t.Fatal("Concurrent magic link reused", good, bad)
	}
}

func TestScopedUpdatesCannotEscapeThroughOr(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	owner := &DB{raw: db.raw, columns: db.columns, admin: true}
	var a, b, foreign string
	owner.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('A') RETURNING id").Scan(&a)
	owner.QueryRow(ctx, "INSERT INTO organizations(name) VALUES('B') RETURNING id").Scan(&b)
	owner.QueryRow(ctx, "INSERT INTO resources(org_id,name) VALUES(?1,'Keep foreign') RETURNING id", b).Scan(&foreign)
	owner.Exec(ctx, "INSERT INTO resources(org_id,name) VALUES(?1,'Local')", a)
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	tx.org = a
	if _, e = tx.Exec(ctx, "UPDATE resources SET name='Changed' WHERE id=?1 OR 1=1", foreign); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	var name string
	if e = owner.QueryRow(ctx, "SELECT name FROM resources WHERE id=?1", foreign).Scan(&name); e != nil || name != "Keep foreign" {
		t.Fatal("Organization predicate escaped", name, e)
	}
}
