package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// execContext is satisfied by both *sql.DB and *sql.Tx.
type execContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// WriteRun inserts or replaces a row in the runs table.
// If a row with the same id already exists it is replaced (idempotent).
func WriteRun(ctx context.Context, db *DB, run RunRecord) error {
	return doWriteRun(ctx, db.db, run)
}

// WriteRunTx inserts or replaces a row in the runs table within a transaction.
func WriteRunTx(ctx context.Context, tx *sql.Tx, run RunRecord) error {
	return doWriteRun(ctx, tx, run)
}

func doWriteRun(ctx context.Context, ex execContext, run RunRecord) error {
	_, err := ex.ExecContext(ctx,
		`INSERT OR REPLACE INTO runs
			(id, repo, milestone, base_branch, auto_merge_feature, auto_merge_rollup,
			 started_at, finished_at, total, implemented, failed, abort_reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID,
		run.Repo,
		run.Milestone,
		run.BaseBranch,
		run.AutoMergeFeature,
		run.AutoMergeRollup,
		run.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
		run.FinishedAt.UTC().Format("2006-01-02T15:04:05Z"),
		run.Total,
		run.Implemented,
		run.Failed,
		run.AbortReason,
	)
	if err != nil {
		return fmt.Errorf("write run %q: %w", run.ID, err)
	}
	return nil
}

// WriteIssueOutcome inserts or replaces a row in the issue_outcomes table.
// If a row with the same (run_id, issue_number) already exists it is replaced.
func WriteIssueOutcome(ctx context.Context, db *DB, outcome IssueOutcomeRecord) error {
	return doWriteIssueOutcome(ctx, db.db, outcome)
}

// WriteIssueOutcomeTx inserts or replaces a row in the issue_outcomes table within a transaction.
func WriteIssueOutcomeTx(ctx context.Context, tx *sql.Tx, outcome IssueOutcomeRecord) error {
	return doWriteIssueOutcome(ctx, tx, outcome)
}

func doWriteIssueOutcome(ctx context.Context, ex execContext, outcome IssueOutcomeRecord) error {
	_, err := ex.ExecContext(ctx,
		`INSERT OR REPLACE INTO issue_outcomes
			(run_id, issue_number, title, status, pr_number, error, trace_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		outcome.RunID,
		outcome.IssueNumber,
		outcome.Title,
		outcome.Status,
		outcome.PRNumber,
		outcome.Error,
		outcome.TraceID,
	)
	if err != nil {
		return fmt.Errorf("write issue outcome (run=%q issue=%d): %w", outcome.RunID, outcome.IssueNumber, err)
	}
	return nil
}

// WriteStepResult inserts or replaces a row in the step_results table.
// If a row with the same (run_id, issue_number, step_name) already exists it is replaced.
// Flags is serialized to a JSON array string for storage.
func WriteStepResult(ctx context.Context, db *DB, step StepResultRecord) error {
	return doWriteStepResult(ctx, db.db, step)
}

// WriteStepResultTx inserts or replaces a row in the step_results table within a transaction.
func WriteStepResultTx(ctx context.Context, tx *sql.Tx, step StepResultRecord) error {
	return doWriteStepResult(ctx, tx, step)
}

func doWriteStepResult(ctx context.Context, ex execContext, step StepResultRecord) error {
	flags := step.Flags
	if flags == nil {
		flags = []string{}
	}
	flagsJSON, err := json.Marshal(flags)
	if err != nil {
		return fmt.Errorf("marshal flags: %w", err)
	}

	_, err = ex.ExecContext(ctx,
		`INSERT OR REPLACE INTO step_results
			(run_id, issue_number, step_name, cost_usd, duration_seconds, flags,
			 started_at, finished_at, peak_memory_bytes, cpu_nanoseconds, trace_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.RunID,
		step.IssueNumber,
		step.StepName,
		step.CostUSD,
		step.DurationSeconds,
		string(flagsJSON),
		step.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
		step.FinishedAt.UTC().Format("2006-01-02T15:04:05Z"),
		step.PeakMemoryBytes,
		step.CPUNanoseconds,
		step.TraceID,
	)
	if err != nil {
		return fmt.Errorf("write step result (run=%q issue=%d step=%q): %w", step.RunID, step.IssueNumber, step.StepName, err)
	}
	return nil
}
