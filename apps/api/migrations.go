package main

import (
	"context"
	_ "embed"
	"fmt"
)

//go:embed sqlite_schema.sql
var schema string

func migrate(ctx context.Context, db *DB) error {
	tx, e := db.raw.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var exists int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_version'").Scan(&exists); e != nil {
		return e
	}
	if exists == 0 {
		if _, e = tx.ExecContext(ctx, schema); e != nil {
			return fmt.Errorf("SQLite schema: %w", e)
		}
	} else {
		var version int
		if e = tx.QueryRowContext(ctx, "SELECT max(version) FROM schema_version").Scan(&version); e != nil {
			return e
		}
		if version != 1 {
			return fmt.Errorf("unsupported SQLite schema version %d", version)
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	return db.loadColumns(ctx)
}
