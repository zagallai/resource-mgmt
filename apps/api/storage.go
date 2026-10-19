package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// All organization-owned reads use scoped CTEs. Mutations target the underlying
// table only after checking scope and add an organization predicate independently.
// These helpers accept application-owned SQL, never SQL supplied by an HTTP client.
var tenantTables = []string{"resources", "projects", "sprints", "tasks", "allocations", "rates", "worklogs", "audit_events", "project_activities"}

type DB struct {
	raw     *sql.DB
	columns map[string][]string
	admin   bool
	path    string
}
type Tx struct {
	db  *DB
	raw *sql.Tx
	org string
}
type rowResult struct {
	row *sql.Row
	err error
}

func (r rowResult) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return r.row.Scan(dest...)
}
func openDB(path string) (*DB, error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(abs), 0700); e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "file", Path: abs}
	q := u.Query()
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	raw, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	raw.SetMaxOpenConns(1)
	db := &DB{raw: raw, path: abs, columns: map[string][]string{}}
	if e = raw.Ping(); e != nil {
		raw.Close()
		return nil, e
	}
	if e = os.Chmod(abs, 0600); e != nil {
		raw.Close()
		return nil, e
	}
	return db, nil
}
func (db *DB) Close()                         { db.raw.Close() }
func (db *DB) Ping(ctx context.Context) error { return db.raw.PingContext(ctx) }
func (db *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, e := db.raw.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	return &Tx{db: db, raw: tx}, nil
}
func (tx *Tx) Commit(ctx context.Context) error   { return tx.raw.Commit() }
func (tx *Tx) Rollback(ctx context.Context) error { return tx.raw.Rollback() }
func quote(s string) string                       { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func (db *DB) loadColumns(ctx context.Context) error {
	for _, table := range tenantTables {
		rows, e := db.raw.QueryContext(ctx, "PRAGMA table_info(base_"+table+")")
		if e != nil {
			return e
		}
		var names []string
		for rows.Next() {
			var cid, notnull, pk int
			var name, typ string
			var def any
			if e = rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); e != nil {
				rows.Close()
				return e
			}
			names = append(names, name)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()
		db.columns[table] = names
	}
	return nil
}

var snapshotRE = regexp.MustCompile(`row_to_json\(([a-z]+)\)`)
var fromRE = regexp.MustCompile(`(?i)\bFROM\s+([a-z_]+)\s+([a-z]+)\b`)
var mutationRE = regexp.MustCompile(`(?i)^(INSERT INTO|UPDATE|DELETE FROM)\s+([a-z_]+)`)
var paramRE = regexp.MustCompile(`\$(\d+)`)

func (db *DB) prepare(q, org string, args []any) (string, []any, error) {
	// Numbered SQLite parameters keep parameter order independent of SQL position.
	q = paramRE.ReplaceAllString(q, "?$1")
	for i, a := range args {
		if b, ok := a.([]byte); ok {
			args[i] = string(b)
		}
	}
	if snapshotRE.MatchString(q) {
		match := fromRE.FindStringSubmatch(q)
		if len(match) != 3 {
			return "", nil, errors.New("invalid snapshot query")
		}
		cols := db.columns[match[1]]
		if len(cols) == 0 {
			return "", nil, errors.New("unknown snapshot table")
		}
		var fields []string
		for _, col := range cols {
			fields = append(fields, quote(col), match[2]+"."+col)
		}
		q = snapshotRE.ReplaceAllString(q, "json_object("+strings.Join(fields, ",")+")")
	}
	match := mutationRE.FindStringSubmatch(q)
	if len(match) == 3 {
		scoped := false
		for _, t := range tenantTables {
			if match[2] == t {
				scoped = true
			}
		}
		if scoped {
			if !db.admin && org == "" {
				return "", nil, errors.New("organization scope required")
			}
			if strings.EqualFold(match[1], "INSERT INTO") && !db.admin {
				if len(args) == 0 || fmt.Sprint(args[0]) != org {
					return "", nil, errors.New("insert organization mismatch")
				}
			}
			q = strings.Replace(q, match[0], match[1]+" base_"+match[2], 1)
			if !strings.EqualFold(match[1], "INSERT INTO") && !db.admin {
				tail := ""
				if pos := topLevelKeyword(q, "RETURNING"); pos >= 0 {
					tail = q[pos:]
					q = q[:pos]
				}
				if pos := topLevelKeyword(q, "WHERE"); pos >= 0 {
					q = q[:pos] + "WHERE (" + strings.TrimSpace(q[pos+5:]) + ") AND org_id=" + quote(org)
				} else {
					q += " WHERE org_id=" + quote(org)
				}
				q += " " + tail
			}
		}
	}
	var ctes []string
	for _, t := range tenantTables {
		filter := ""
		if !db.admin {
			filter = " WHERE org_id=" + quote(org)
		}
		ctes = append(ctes, t+" AS (SELECT * FROM base_"+t+filter+")")
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(q)), "WITH ") {
		return "", nil, errors.New("top-level CTEs require explicit scoped implementation")
	}
	return "WITH " + strings.Join(ctes, ",") + " " + q, args, nil
}
func (tx *Tx) QueryRow(ctx context.Context, q string, args ...any) rowResult {
	q, args, e := tx.db.prepare(q, tx.org, args)
	if e != nil {
		return rowResult{err: e}
	}
	return rowResult{row: tx.raw.QueryRowContext(ctx, q, args...)}
}
func (tx *Tx) Exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	q, args, e := tx.db.prepare(q, tx.org, args)
	if e != nil {
		return nil, e
	}
	return tx.raw.ExecContext(ctx, q, args...)
}
func (db *DB) QueryRow(ctx context.Context, q string, args ...any) rowResult {
	q, args, e := db.prepare(q, "", args)
	if e != nil {
		return rowResult{err: e}
	}
	return rowResult{row: db.raw.QueryRowContext(ctx, q, args...)}
}
func (db *DB) Exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	q, args, e := db.prepare(q, "", args)
	if e != nil {
		return nil, e
	}
	return db.raw.ExecContext(ctx, q, args...)
}
func maps(rows *sql.Rows) ([]map[string]any, error) {
	defer rows.Close()
	names, e := rows.Columns()
	if e != nil {
		return nil, e
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(names))
		pointers := make([]any, len(names))
		for i := range values {
			pointers[i] = &values[i]
		}
		if e = rows.Scan(pointers...); e != nil {
			return nil, e
		}
		row := map[string]any{}
		for i, name := range names {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			if name == "detail" {
				var parsed any
				if str, ok := v.(string); ok && json.Unmarshal([]byte(str), &parsed) == nil {
					v = parsed
				}
			}
			if name == "overloaded" {
				if n, ok := v.(int64); ok {
					v = n != 0
				}
			}
			row[name] = v
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
func (tx *Tx) QueryMaps(ctx context.Context, q string, args ...any) ([]map[string]any, error) {
	q, args, e := tx.db.prepare(q, tx.org, args)
	if e != nil {
		return nil, e
	}
	rows, e := tx.raw.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	return maps(rows)
}
func (db *DB) QueryMaps(ctx context.Context, q string, args ...any) ([]map[string]any, error) {
	q, args, e := db.prepare(q, "", args)
	if e != nil {
		return nil, e
	}
	rows, e := db.raw.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	return maps(rows)
}
func (db *DB) Backup(ctx context.Context, dest string) error {
	abs, e := filepath.Abs(dest)
	if e != nil {
		return e
	}
	if _, e = os.Stat(abs); !os.IsNotExist(e) {
		return errors.New("backup destination must not exist")
	}
	if _, e = db.raw.ExecContext(ctx, "VACUUM INTO "+quote(abs)); e != nil {
		return e
	}
	return os.Chmod(abs, 0600)
}

// Find a keyword outside strings and subqueries before adding the scope predicate.
func topLevelKeyword(q, word string) int {
	depth := 0
	quoted := false
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == '\'' {
			if quoted && i+1 < len(q) && q[i+1] == '\'' {
				i++
				continue
			}
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if c == '(' {
			depth++
			continue
		}
		if c == ')' {
			depth--
			continue
		}
		if depth == 0 && i+len(word) <= len(q) && strings.EqualFold(q[i:i+len(word)], word) && (i == 0 || q[i-1] == ' ') && (i+len(word) == len(q) || q[i+len(word)] == ' ') {
			return i
		}
	}
	return -1
}

func defaultDBPath() string {
	cwd, e := os.Getwd()
	if e != nil {
		return "data/resource-compass.db"
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, e := os.Stat(filepath.Join(dir, "apps", "api", "go.mod")); e == nil {
			return filepath.Join(dir, "data", "resource-compass.db")
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return filepath.Join(cwd, "data", "resource-compass.db")
}
