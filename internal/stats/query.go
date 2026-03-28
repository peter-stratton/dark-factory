package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const timeLayout = "2006-01-02T15:04:05Z"

// RunFilter specifies optional constraints for query functions.
// Zero values mean no filter is applied for that field.
type RunFilter struct {
	Repo      string
	Milestone string
	Since     time.Time
	Until     time.Time
}

// QueryRuns returns all runs matching the filter, sorted by started_at ascending.
// An empty filter returns all runs. A filter that matches nothing returns an empty slice.
func QueryRuns(ctx context.Context, db *DB, filter RunFilter) ([]RunRecord, error) {
	where, args := buildRunWhere(filter)
	const runsBase = `SELECT id, repo, milestone, base_branch, auto_merge_feature, auto_merge_rollup,
	             started_at, finished_at, total, implemented, failed, abort_reason
	      FROM runs`
	q := runsBase + where + ` ORDER BY started_at ASC` // #nosec G202 -- where contains only hardcoded clauses with ? placeholders

	rows, err := db.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()

	var results []RunRecord
	for rows.Next() {
		var r RunRecord
		var startedAt, finishedAt string
		if scanErr := rows.Scan(
			&r.ID, &r.Repo, &r.Milestone, &r.BaseBranch,
			&r.AutoMergeFeature, &r.AutoMergeRollup,
			&startedAt, &finishedAt,
			&r.Total, &r.Implemented, &r.Failed, &r.AbortReason,
		); scanErr != nil {
			return nil, fmt.Errorf("scan run: %w", scanErr)
		}
		t, parseErr := time.Parse(timeLayout, startedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse started_at %q: %w", startedAt, parseErr)
		}
		r.StartedAt = t
		t, parseErr = time.Parse(timeLayout, finishedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse finished_at %q: %w", finishedAt, parseErr)
		}
		r.FinishedAt = t
		results = append(results, r)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	if results == nil {
		return []RunRecord{}, nil
	}
	return results, nil
}

// QueryIssueOutcomes returns all issue outcomes for runs matching the filter,
// sorted by run started_at ascending. An empty filter returns all outcomes.
func QueryIssueOutcomes(ctx context.Context, db *DB, filter RunFilter) ([]IssueOutcomeRecord, error) {
	where, args := buildRunJoinWhere(filter)
	const issueOutcomesBase = `SELECT io.run_id, io.issue_number, io.title, io.status, io.pr_number, io.error, io.trace_id
	      FROM issue_outcomes io
	      INNER JOIN runs r ON r.id = io.run_id`
	q := issueOutcomesBase + where + ` ORDER BY r.started_at ASC` // #nosec G202 -- where contains only hardcoded clauses with ? placeholders

	rows, err := db.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query issue outcomes: %w", err)
	}
	defer rows.Close()

	var results []IssueOutcomeRecord
	for rows.Next() {
		var o IssueOutcomeRecord
		if scanErr := rows.Scan(
			&o.RunID, &o.IssueNumber, &o.Title, &o.Status, &o.PRNumber, &o.Error, &o.TraceID,
		); scanErr != nil {
			return nil, fmt.Errorf("scan issue outcome: %w", scanErr)
		}
		results = append(results, o)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue outcomes: %w", err)
	}
	if results == nil {
		return []IssueOutcomeRecord{}, nil
	}
	return results, nil
}

// QueryStepResults returns all step results for runs matching the filter,
// sorted by run started_at ascending. An empty filter returns all step results.
func QueryStepResults(ctx context.Context, db *DB, filter RunFilter) ([]StepResultRecord, error) {
	where, args := buildRunJoinWhere(filter)
	const stepResultsBase = `SELECT sr.run_id, sr.issue_number, sr.step_name, sr.cost_usd, sr.duration_seconds,
	             sr.flags, sr.started_at, sr.finished_at, sr.peak_memory_bytes, sr.cpu_nanoseconds, sr.trace_id
	      FROM step_results sr
	      INNER JOIN runs r ON r.id = sr.run_id`
	q := stepResultsBase + where + ` ORDER BY r.started_at ASC` // #nosec G202 -- where contains only hardcoded clauses with ? placeholders

	rows, err := db.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query step results: %w", err)
	}
	defer rows.Close()

	var results []StepResultRecord
	for rows.Next() {
		var s StepResultRecord
		var flagsJSON, startedAt, finishedAt string
		if scanErr := rows.Scan(
			&s.RunID, &s.IssueNumber, &s.StepName, &s.CostUSD, &s.DurationSeconds,
			&flagsJSON, &startedAt, &finishedAt, &s.PeakMemoryBytes, &s.CPUNanoseconds, &s.TraceID,
		); scanErr != nil {
			return nil, fmt.Errorf("scan step result: %w", scanErr)
		}
		if err := json.Unmarshal([]byte(flagsJSON), &s.Flags); err != nil {
			return nil, fmt.Errorf("unmarshal flags %q: %w", flagsJSON, err)
		}
		t, parseErr := time.Parse(timeLayout, startedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse step started_at %q: %w", startedAt, parseErr)
		}
		s.StartedAt = t
		t, parseErr = time.Parse(timeLayout, finishedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse step finished_at %q: %w", finishedAt, parseErr)
		}
		s.FinishedAt = t
		results = append(results, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate step results: %w", err)
	}
	if results == nil {
		return []StepResultRecord{}, nil
	}
	return results, nil
}

// QueryStepsByTraceID returns all step results for a given trace ID,
// sorted by started_at ascending. Returns an empty slice if none found.
func QueryStepsByTraceID(ctx context.Context, db *DB, traceID string) ([]StepResultRecord, error) {
	const q = `SELECT run_id, issue_number, step_name, cost_usd, duration_seconds,
	             flags, started_at, finished_at, peak_memory_bytes, cpu_nanoseconds, trace_id
	      FROM step_results WHERE trace_id = ? ORDER BY started_at ASC`

	rows, err := db.db.QueryContext(ctx, q, traceID)
	if err != nil {
		return nil, fmt.Errorf("query steps by trace_id: %w", err)
	}
	defer rows.Close()

	var results []StepResultRecord
	for rows.Next() {
		var s StepResultRecord
		var flagsJSON, startedAt, finishedAt string
		if scanErr := rows.Scan(
			&s.RunID, &s.IssueNumber, &s.StepName, &s.CostUSD, &s.DurationSeconds,
			&flagsJSON, &startedAt, &finishedAt, &s.PeakMemoryBytes, &s.CPUNanoseconds, &s.TraceID,
		); scanErr != nil {
			return nil, fmt.Errorf("scan step result: %w", scanErr)
		}
		if err := json.Unmarshal([]byte(flagsJSON), &s.Flags); err != nil {
			return nil, fmt.Errorf("unmarshal flags %q: %w", flagsJSON, err)
		}
		t, parseErr := time.Parse(timeLayout, startedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse step started_at %q: %w", startedAt, parseErr)
		}
		s.StartedAt = t
		t, parseErr = time.Parse(timeLayout, finishedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse step finished_at %q: %w", finishedAt, parseErr)
		}
		s.FinishedAt = t
		results = append(results, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate steps by trace_id: %w", err)
	}
	if results == nil {
		return []StepResultRecord{}, nil
	}
	return results, nil
}

// QueryOutcomeByTraceID returns the issue outcome for a given trace ID,
// or nil if no outcome exists with that trace ID.
func QueryOutcomeByTraceID(ctx context.Context, db *DB, traceID string) (*IssueOutcomeRecord, error) {
	const q = `SELECT run_id, issue_number, title, status, pr_number, error, trace_id
	      FROM issue_outcomes WHERE trace_id = ? LIMIT 1`

	var o IssueOutcomeRecord
	err := db.db.QueryRowContext(ctx, q, traceID).Scan(
		&o.RunID, &o.IssueNumber, &o.Title, &o.Status, &o.PRNumber, &o.Error, &o.TraceID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query outcome by trace_id: %w", err)
	}
	return &o, nil
}

// QueryLatestTraceForIssue returns the trace_id from the most recent issue outcome
// for the given issue number. If repo is non-empty, results are filtered to that repo.
// If runID is non-empty, results are filtered to that specific run.
// Returns an empty string and nil error if no matching outcome is found.
func QueryLatestTraceForIssue(ctx context.Context, db *DB, issueNumber int, repo string, runID string) (string, error) {
	var conds []string
	var args []any

	conds = append(conds, "io.issue_number = ?")
	args = append(args, issueNumber)

	if repo != "" {
		conds = append(conds, "r.repo = ?")
		args = append(args, repo)
	}
	if runID != "" {
		conds = append(conds, "io.run_id = ?")
		args = append(args, runID)
	}

	conds = append(conds, "io.trace_id != ''")

	q := `SELECT io.trace_id FROM issue_outcomes io
	      INNER JOIN runs r ON r.id = io.run_id
	      WHERE ` + strings.Join(conds, " AND ") + // #nosec G202 -- hardcoded clauses with ? placeholders
		` ORDER BY r.started_at DESC LIMIT 1`

	var traceID string
	err := db.db.QueryRowContext(ctx, q, args...).Scan(&traceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("query latest trace for issue: %w", err)
	}
	return traceID, nil
}

// buildRunWhere constructs a WHERE clause for queries directly against the runs table.
// Only non-zero filter fields generate conditions. All parameters are positional
// placeholders — no string interpolation of user-supplied values.
func buildRunWhere(filter RunFilter) (string, []any) {
	var conds []string
	var args []any

	if filter.Repo != "" {
		conds = append(conds, "repo = ?")
		args = append(args, filter.Repo)
	}
	if filter.Milestone != "" {
		conds = append(conds, "milestone = ?")
		args = append(args, filter.Milestone)
	}
	if !filter.Since.IsZero() {
		conds = append(conds, "started_at >= ?")
		args = append(args, filter.Since.UTC().Format(timeLayout))
	}
	if !filter.Until.IsZero() {
		conds = append(conds, "started_at <= ?")
		args = append(args, filter.Until.UTC().Format(timeLayout))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// buildRunJoinWhere constructs a WHERE clause for queries that join issue_outcomes
// or step_results against the runs table (aliased as "r").
func buildRunJoinWhere(filter RunFilter) (string, []any) {
	var conds []string
	var args []any

	if filter.Repo != "" {
		conds = append(conds, "r.repo = ?")
		args = append(args, filter.Repo)
	}
	if filter.Milestone != "" {
		conds = append(conds, "r.milestone = ?")
		args = append(args, filter.Milestone)
	}
	if !filter.Since.IsZero() {
		conds = append(conds, "r.started_at >= ?")
		args = append(args, filter.Since.UTC().Format(timeLayout))
	}
	if !filter.Until.IsZero() {
		conds = append(conds, "r.started_at <= ?")
		args = append(args, filter.Until.UTC().Format(timeLayout))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

