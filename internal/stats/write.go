package stats

import (
	"encoding/json"
	"fmt"
)

// WriteRun inserts or replaces a row in the runs table.
// If a row with the same id already exists it is replaced (idempotent).
func WriteRun(db *DB, run RunRecord) error {
	_, err := db.db.Exec(
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
func WriteIssueOutcome(db *DB, outcome IssueOutcomeRecord) error {
	_, err := db.db.Exec(
		`INSERT OR REPLACE INTO issue_outcomes
			(run_id, issue_number, title, status, pr_number, error)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		outcome.RunID,
		outcome.IssueNumber,
		outcome.Title,
		outcome.Status,
		outcome.PRNumber,
		outcome.Error,
	)
	if err != nil {
		return fmt.Errorf("write issue outcome (run=%q issue=%d): %w", outcome.RunID, outcome.IssueNumber, err)
	}
	return nil
}

// WriteStepResult inserts or replaces a row in the step_results table.
// If a row with the same (run_id, issue_number, step_name) already exists it is replaced.
// Flags is serialized to a JSON array string for storage.
func WriteStepResult(db *DB, step StepResultRecord) error {
	flags := step.Flags
	if flags == nil {
		flags = []string{}
	}
	flagsJSON, err := json.Marshal(flags)
	if err != nil {
		return fmt.Errorf("marshal flags: %w", err)
	}

	_, err = db.db.Exec(
		`INSERT OR REPLACE INTO step_results
			(run_id, issue_number, step_name, cost_usd, duration_seconds, flags,
			 started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		step.RunID,
		step.IssueNumber,
		step.StepName,
		step.CostUSD,
		step.DurationSeconds,
		string(flagsJSON),
		step.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
		step.FinishedAt.UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		return fmt.Errorf("write step result (run=%q issue=%d step=%q): %w", step.RunID, step.IssueNumber, step.StepName, err)
	}
	return nil
}
