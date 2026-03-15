package stats

import (
	"database/sql"
	"fmt"
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

	return tx.Commit()
}
