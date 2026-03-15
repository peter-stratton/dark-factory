package stats

import (
	"testing"
)

func TestOpen_NewDatabase(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	tables := []string{"runs", "issue_outcomes", "step_results"}
	for _, table := range tables {
		var name string
		err := db.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
		if name != table {
			t.Errorf("expected table name %q, got %q", table, name)
		}
	}
}

func TestOpen_Idempotent(t *testing.T) {
	db1, err := Open(":memory:")
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	defer db1.Close()

	// Run migrate again on the same underlying connection to verify idempotency.
	if err := migrate(db1.db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestClose(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestTables_InsertAllColumns(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	_, err = db.db.Exec(`INSERT INTO runs
		(id, repo, milestone, base_branch, auto_merge_feature, auto_merge_rollup,
		 started_at, finished_at, total, implemented, failed, abort_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-1", "org/repo", "v1.0", "main", "feature", "rollup",
		"2026-01-01T00:00:00Z", "2026-01-01T01:00:00Z", 5, 4, 1, "",
	)
	if err != nil {
		t.Fatalf("insert into runs: %v", err)
	}

	_, err = db.db.Exec(`INSERT INTO issue_outcomes
		(run_id, issue_number, title, status, pr_number, error)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"run-1", 42, "Fix bug", "implemented", 99, "",
	)
	if err != nil {
		t.Fatalf("insert into issue_outcomes: %v", err)
	}

	_, err = db.db.Exec(`INSERT INTO step_results
		(run_id, issue_number, step_name, cost_usd, duration_seconds, flags,
		 started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-1", 42, "implement", 0.05, 30.5, "[]",
		"2026-01-01T00:00:00Z", "2026-01-01T00:00:30Z",
	)
	if err != nil {
		t.Fatalf("insert into step_results: %v", err)
	}
}
