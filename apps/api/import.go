package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var importTables = []string{"organizations", "users", "memberships", "invitations", "login_tokens", "sessions", "auth_limits", "resources", "projects", "sprints", "tasks", "allocations", "rates", "worklogs", "audit_events", "project_activities"}

// Import only into an empty destination and commit the complete validated copy.
func importPostgres(ctx context.Context, db *DB, path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	var data map[string][]map[string]any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if e = dec.Decode(&data); e != nil {
		return e
	}
	if len(data) != len(importTables) {
		return fmt.Errorf("export must contain all %d tables", len(importTables))
	}
	tx, e := db.raw.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); e != nil {
		return e
	}
	for _, table := range importTables {
		rows, ok := data[table]
		if !ok {
			return fmt.Errorf("missing export table %s", table)
		}
		target := table
		for _, t := range tenantTables {
			if t == table {
				target = "base_" + table
			}
		}
		var existing int
		if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+target).Scan(&existing); e != nil {
			return e
		}
		if existing != 0 {
			return fmt.Errorf("destination %s is not empty", table)
		}
		columnsRows, e := tx.QueryContext(ctx, "PRAGMA table_info("+target+")")
		if e != nil {
			return e
		}
		var cols []string
		for columnsRows.Next() {
			var cid, nn, pk int
			var col, typ string
			var def any
			if e = columnsRows.Scan(&cid, &col, &typ, &nn, &def, &pk); e != nil {
				columnsRows.Close()
				return e
			}
			cols = append(cols, col)
		}
		columnsRows.Close()
		for _, row := range rows {
			if len(row) != len(cols) {
				return fmt.Errorf("%s column count mismatch", table)
			}
			args := make([]any, len(cols))
			marks := make([]string, len(cols))
			for i, col := range cols {
				v, ok := row[col]
				if !ok {
					return fmt.Errorf("%s missing column %s", table, col)
				}
				switch x := v.(type) {
				case json.Number:
					if n, e := strconv.ParseInt(string(x), 10, 64); e == nil {
						v = n
					} else {
						n, e := strconv.ParseFloat(string(x), 64)
						if e != nil {
							return e
						}
						v = n
					}
				case map[string]any, []any:
					b, e := json.Marshal(x)
					if e != nil {
						return e
					}
					v = string(b)
				case string:
					if strings.HasSuffix(col, "_at") {
						if t, e := time.Parse(time.RFC3339Nano, x); e == nil {
							v = t.UTC().Format("2006-01-02T15:04:05.000000Z")
						}
					}
				}
				args[i] = v
				marks[i] = "?"
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO "+target+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", args...); e != nil {
				return fmt.Errorf("import %s: %w", table, e)
			}
		}
		var count int
		if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+target).Scan(&count); e != nil {
			return e
		}
		if count != len(rows) {
			return fmt.Errorf("%s import count mismatch", table)
		}
	}
	rows, e := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	bad := rows.Next()
	rows.Close()
	if bad {
		return fmt.Errorf("import has broken relationships")
	}
	var integrity string
	if e = tx.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); e != nil {
		return e
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check: %s", integrity)
	}
	return tx.Commit()
}
