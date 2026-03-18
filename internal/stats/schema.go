package stats

import (
	"database/sql"
	"fmt"
	"strings"
)

// migrate creates the stats tables if they do not already exist.
// It is idempotent: calling it on an existing database does not modify data.
func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS runs (
			id                 TEXT PRIMARY KEY,
			repo               TEXT,
			milestone          TEXT,
			base_branch        TEXT,
			auto_merge_feature TEXT,
			auto_merge_rollup  TEXT,
			started_at         TIMESTAMP,
			finished_at        TIMESTAMP,
			total              INT,
			implemented        INT,
			failed             INT,
			abort_reason       TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS issue_outcomes (
			run_id       TEXT,
			issue_number INT,
			title        TEXT,
			status       TEXT,
			pr_number    INT,
			error        TEXT,
			UNIQUE (run_id, issue_number)
		)`,
		`CREATE TABLE IF NOT EXISTS step_results (
			run_id           TEXT,
			issue_number     INT,
			step_name        TEXT,
			cost_usd         REAL,
			duration_seconds REAL,
			flags            TEXT,
			started_at       TIMESTAMP,
			finished_at      TIMESTAMP,
			UNIQUE (run_id, issue_number, step_name)
		)`,
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}

	// ALTER TABLE statements must run outside a transaction so that
	// "duplicate column name" errors can be suppressed per-statement for idempotency.
	alterStmts := []string{
		`ALTER TABLE step_results ADD COLUMN peak_memory_bytes INTEGER DEFAULT 0`,
		`ALTER TABLE step_results ADD COLUMN cpu_nanoseconds INTEGER DEFAULT 0`,
	}
	for _, stmt := range alterStmts {
		if _, err := db.Exec(stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("execute migration: %w", err)
			}
			// column already exists — idempotent
		}
	}

	return nil
}
