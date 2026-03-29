package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
)

// OpenStatsDB opens or creates the stats SQLite database at ~/.godark/stats.db.
// Returns nil and logs a warning if the database cannot be opened (e.g. unwritable
// path) — callers must tolerate a nil return value.
func OpenStatsDB(logger *slog.Logger) *stats.DB {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("stats: failed to get home dir", "error", err)
		return nil
	}
	dir := filepath.Join(home, ".godark")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		logger.Warn("stats: failed to create .godark dir", "error", err)
		return nil
	}
	dbPath := filepath.Join(dir, "stats.db")
	db, err := stats.Open(dbPath)
	if err != nil {
		logger.Warn("stats: failed to open database", "path", dbPath, "error", err)
		return nil
	}
	return db
}

// writeRunStatsNewReaderFn creates the rundata.Reader used by WriteRunStats.
// Replaceable in tests to inject a reader over a custom base directory.
var writeRunStatsNewReaderFn = func(logger *slog.Logger) (*rundata.Reader, error) {
	return rundata.NewReader(logger)
}

// WriteRunStats loads the completed run directory and writes the run record,
// issue outcomes, and step results to the stats database. All failures are
// logged as warnings and never abort the run. A nil db or nil writer is a no-op.
func WriteRunStats(ctx context.Context, db *stats.DB, cfg *config.Config, writer *rundata.Writer, summary rundata.RunSummary, logger *slog.Logger) {
	if db == nil || writer == nil {
		return
	}

	if logger == nil {
		logger = slog.Default()
	}

	parts := strings.SplitN(cfg.Repo, "/", 2)
	if len(parts) != 2 {
		logger.Warn("stats: invalid repo format, skipping", "repo", cfg.Repo)
		return
	}
	owner, repoName := parts[0], parts[1]
	timestamp := filepath.Base(writer.Dir())

	reader, err := writeRunStatsNewReaderFn(logger)
	if err != nil {
		logger.Warn("stats: failed to create run data reader", "error", err)
		return
	}

	detail, err := reader.LoadRun(owner, repoName, timestamp)
	if err != nil {
		logger.Warn("stats: failed to load run detail", "error", err)
		return
	}

	runRec := buildRunRecord(timestamp, cfg.Repo, detail, summary)

	tx, err := db.BeginTx(ctx)
	if err != nil {
		logger.Warn("stats: failed to begin transaction", "error", err)
		return
	}

	if err := stats.WriteRunTx(ctx, tx, runRec); err != nil {
		logger.Warn("stats: failed to write run record", "error", err)
		_ = tx.Rollback()
		return
	}

	for _, issue := range detail.Issues {
		if issue.Outcome.Status == "" {
			continue
		}
		outcomeRec := stats.IssueOutcomeRecord{
			RunID:       timestamp,
			IssueNumber: issue.IssueNumber,
			Title:       issue.Outcome.Title,
			Status:      issue.Outcome.Status,
			PRNumber:    issue.Outcome.PRNumber,
			Error:       issue.Outcome.Error,
			TraceID:     issue.Outcome.TraceID,
			CloneSHA:    issue.Outcome.CloneSHA,
		}
		if err := stats.WriteIssueOutcomeTx(ctx, tx, outcomeRec); err != nil {
			logger.Warn("stats: failed to write issue outcome",
				"issue_number", issue.IssueNumber, "error", err)
			_ = tx.Rollback()
			return
		}

		for _, stepRec := range buildStepRecords(timestamp, issue) {
			if err := stats.WriteStepResultTx(ctx, tx, stepRec); err != nil {
				logger.Warn("stats: failed to write step result",
					"issue_number", issue.IssueNumber, "step", stepRec.StepName, "error", err)
				_ = tx.Rollback()
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Warn("stats: failed to commit transaction", "error", err)
	}
}

// buildRunRecord constructs a stats.RunRecord from run detail and summary data.
func buildRunRecord(runID, repo string, detail *rundata.RunDetail, summary rundata.RunSummary) stats.RunRecord {
	finishedAt := time.Now()
	if detail.FinishedAt != nil {
		finishedAt = *detail.FinishedAt
	}
	autoMergeFeature := ""
	autoMergeRollup := ""
	if detail.AutoMerge != nil {
		autoMergeFeature = detail.AutoMerge.Feature
		autoMergeRollup = detail.AutoMerge.Rollup
	}
	return stats.RunRecord{
		ID:               runID,
		Repo:             repo,
		Milestone:        detail.Milestone,
		BaseBranch:       detail.BaseBranch,
		AutoMergeFeature: autoMergeFeature,
		AutoMergeRollup:  autoMergeRollup,
		StartedAt:        detail.StartedAt,
		FinishedAt:       finishedAt,
		Total:            summary.Total,
		Implemented:      summary.Implemented,
		Failed:           summary.Failed,
		AbortReason:      summary.AbortReason,
	}
}

// buildStepRecords converts a rundata.IssueDetail to a slice of stats.StepResultRecord,
// including the main pipeline steps and any retry steps.
func buildStepRecords(runID string, issue rundata.IssueDetail) []stats.StepResultRecord {
	var records []stats.StepResultRecord

	type namedStep struct {
		name string
		step rundata.StepResult
	}
	mainSteps := []namedStep{
		{"recon", issue.Recon},
		{"spec-generator", issue.SpecGenerator},
		{"implement", issue.Implement},
		{"quality-review", issue.QualityReview},
		{"functional-review", issue.FunctionalReview},
	}
	for _, s := range mainSteps {
		if s.step.StartedAt == nil && s.step.DurationSeconds == 0 {
			continue // step was not executed
		}
		records = append(records, stepToRecord(runID, issue.IssueNumber, s.name, s.step))
	}

	for _, retry := range issue.Retries {
		if retry.Retry.StartedAt != nil || retry.Retry.DurationSeconds > 0 {
			name := fmt.Sprintf("retry-%d", retry.Attempt)
			records = append(records, stepToRecord(runID, issue.IssueNumber, name, retry.Retry))
		}
		if retry.QualityReview.StartedAt != nil || retry.QualityReview.DurationSeconds > 0 {
			name := fmt.Sprintf("retry-%d-quality-review", retry.Attempt)
			records = append(records, stepToRecord(runID, issue.IssueNumber, name, retry.QualityReview))
		}
		if retry.FunctionalReview.StartedAt != nil || retry.FunctionalReview.DurationSeconds > 0 {
			name := fmt.Sprintf("retry-%d-functional-review", retry.Attempt)
			records = append(records, stepToRecord(runID, issue.IssueNumber, name, retry.FunctionalReview))
		}
	}

	return records
}

// stepToRecord converts a single rundata.StepResult to a stats.StepResultRecord.
func stepToRecord(runID string, issueNumber int, stepName string, step rundata.StepResult) stats.StepResultRecord {
	startedAt := time.Time{}
	if step.StartedAt != nil {
		startedAt = *step.StartedAt
	}
	finishedAt := time.Time{}
	if step.FinishedAt != nil {
		finishedAt = *step.FinishedAt
	}

	flags := make([]string, len(step.Flags))
	for i, f := range step.Flags {
		flags[i] = f.Code
	}

	return stats.StepResultRecord{
		RunID:           runID,
		IssueNumber:     issueNumber,
		StepName:        stepName,
		CostUSD:         step.CostUSD,
		DurationSeconds: step.DurationSeconds,
		Flags:           flags,
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		PeakMemoryBytes: step.PeakMemoryBytes,
		CPUNanoseconds:  step.CPUNanoseconds,
		TraceID:         step.TraceID,
	}
}
