package cmd

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
)

func TestPurgeCmd_DryRun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db := openPurgeDB(t, dbPath)
	seedPurgeData(t, db)
	db.Close()

	origDB := newPurgeDB
	newPurgeDB = func() (*stats.DB, error) { return stats.Open(dbPath) }
	defer func() { newPurgeDB = origDB }()

	origReader := newPurgeReader
	newPurgeReader = func(logger *slog.Logger) (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(t.TempDir(), logger), nil
	}
	defer func() { newPurgeReader = origReader }()

	cmd := purgeCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set --dry-run: %v", err)
	}
	defer func() { _ = cmd.Flags().Set("dry-run", "false") }()

	_ = cmd.Flags().Set("repo", "owner/repo")
	_ = cmd.Flags().Set("milestone", "test-milestone")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("purge --dry-run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected dry run output, got: %s", out)
	}
	if !strings.Contains(out, "2 run(s)") {
		t.Errorf("expected 2 runs in dry run output, got: %s", out)
	}

	// Verify nothing was actually deleted.
	db2 := openPurgeDB(t, dbPath)
	defer db2.Close()
	runs, err := stats.QueryRuns(context.Background(), db2, stats.RunFilter{})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("expected 3 runs still present, got %d", len(runs))
	}
}

func TestPurgeCmd_DeletesMatchingRuns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db := openPurgeDB(t, dbPath)
	seedPurgeData(t, db)
	db.Close()

	origDB := newPurgeDB
	newPurgeDB = func() (*stats.DB, error) { return stats.Open(dbPath) }
	defer func() { newPurgeDB = origDB }()

	tmpDir := t.TempDir()
	runsBase := filepath.Join(tmpDir, ".godark", "runs")

	origReader := newPurgeReader
	newPurgeReader = func(logger *slog.Logger) (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(runsBase, logger), nil
	}
	defer func() { newPurgeReader = origReader }()

	createRunDir(t, runsBase, "owner", "repo", "20260101-000000")
	createRunDir(t, runsBase, "org", "real", "20260201-000000")
	createRunDir(t, runsBase, "org", "other", "20260301-000000")

	cmd := purgeCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	_ = cmd.Flags().Set("dry-run", "false")
	_ = cmd.Flags().Set("repo", "owner/repo")
	_ = cmd.Flags().Set("milestone", "test-milestone")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("purge: %v", err)
	}

	// Verify DB deletions.
	db2 := openPurgeDB(t, dbPath)
	defer db2.Close()

	runs, err := stats.QueryRuns(context.Background(), db2, stats.RunFilter{})
	if err != nil {
		t.Fatalf("QueryRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run remaining, got %d", len(runs))
	}
	if len(runs) > 0 && runs[0].ID != "run-real" {
		t.Errorf("remaining run should be run-real, got %q", runs[0].ID)
	}

	outcomes, err := stats.QueryIssueOutcomes(context.Background(), db2, stats.RunFilter{})
	if err != nil {
		t.Fatalf("QueryIssueOutcomes: %v", err)
	}
	if len(outcomes) != 1 {
		t.Errorf("expected 1 outcome remaining, got %d", len(outcomes))
	}

	steps, err := stats.QueryStepResults(context.Background(), db2, stats.RunFilter{})
	if err != nil {
		t.Fatalf("QueryStepResults: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step result remaining, got %d", len(steps))
	}

	// Verify filesystem cleanup.
	if _, err := os.Stat(filepath.Join(runsBase, "owner", "repo", "20260101-000000")); !os.IsNotExist(err) {
		t.Error("expected owner/repo run directory to be removed")
	}
	if _, err := os.Stat(filepath.Join(runsBase, "owner", "repo")); !os.IsNotExist(err) {
		t.Error("expected owner/repo parent directory to be removed")
	}
	if _, err := os.Stat(filepath.Join(runsBase, "owner")); !os.IsNotExist(err) {
		t.Error("expected owner parent directory to be removed")
	}

	out := buf.String()
	if !strings.Contains(out, "Purged") {
		t.Errorf("expected purge summary, got: %s", out)
	}
}

func TestPurgeCmd_NoMatches(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db := openPurgeDB(t, dbPath)
	writeTestRun(t, db, stats.RunRecord{
		ID: "run-real", Repo: "org/real", Milestone: "Phase 21",
		StartedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	db.Close()

	origDB := newPurgeDB
	newPurgeDB = func() (*stats.DB, error) { return stats.Open(dbPath) }
	defer func() { newPurgeDB = origDB }()

	origReader := newPurgeReader
	newPurgeReader = func(logger *slog.Logger) (*rundata.Reader, error) {
		return rundata.NewReaderWithBase(t.TempDir(), logger), nil
	}
	defer func() { newPurgeReader = origReader }()

	cmd := purgeCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	_ = cmd.Flags().Set("dry-run", "false")
	_ = cmd.Flags().Set("repo", "owner/repo")
	_ = cmd.Flags().Set("milestone", "test-milestone")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("purge: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No matching runs") {
		t.Errorf("expected no-match message, got: %s", out)
	}
}

func TestParseTimestampForDir(t *testing.T) {
	got, err := parseTimestampForDir("2026-03-14T10:30:00Z")
	if err != nil {
		t.Fatalf("parseTimestampForDir: %v", err)
	}
	want := "20260314-103000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- helpers ---

func openPurgeDB(t *testing.T, path string) *stats.DB {
	t.Helper()
	db, err := stats.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}

func writeTestRun(t *testing.T, db *stats.DB, r stats.RunRecord) {
	t.Helper()
	if err := stats.WriteRun(context.Background(), db, r); err != nil {
		t.Fatalf("WriteRun %q: %v", r.ID, err)
	}
}

func writeTestOutcome(t *testing.T, db *stats.DB, o stats.IssueOutcomeRecord) {
	t.Helper()
	if err := stats.WriteIssueOutcome(context.Background(), db, o); err != nil {
		t.Fatalf("WriteIssueOutcome run=%q issue=%d: %v", o.RunID, o.IssueNumber, err)
	}
}

func writeTestStep(t *testing.T, db *stats.DB, s stats.StepResultRecord) {
	t.Helper()
	if err := stats.WriteStepResult(context.Background(), db, s); err != nil {
		t.Fatalf("WriteStepResult run=%q issue=%d step=%q: %v", s.RunID, s.IssueNumber, s.StepName, err)
	}
}

func seedPurgeData(t *testing.T, db *stats.DB) {
	t.Helper()
	writeTestRun(t, db, stats.RunRecord{
		ID: "run-test-repo", Repo: "owner/repo", Milestone: "Phase 21",
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	writeTestOutcome(t, db, stats.IssueOutcomeRecord{RunID: "run-test-repo", IssueNumber: 1, Status: "implemented"})
	writeTestStep(t, db, stats.StepResultRecord{RunID: "run-test-repo", IssueNumber: 1, StepName: "implement"})

	writeTestRun(t, db, stats.RunRecord{
		ID: "run-test-ms", Repo: "org/other", Milestone: "test-milestone",
		StartedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	})
	writeTestOutcome(t, db, stats.IssueOutcomeRecord{RunID: "run-test-ms", IssueNumber: 2, Status: "failed"})
	writeTestStep(t, db, stats.StepResultRecord{RunID: "run-test-ms", IssueNumber: 2, StepName: "implement"})

	writeTestRun(t, db, stats.RunRecord{
		ID: "run-real", Repo: "org/real", Milestone: "Phase 21",
		StartedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	writeTestOutcome(t, db, stats.IssueOutcomeRecord{RunID: "run-real", IssueNumber: 3, Status: "implemented"})
	writeTestStep(t, db, stats.StepResultRecord{RunID: "run-real", IssueNumber: 3, StepName: "implement"})
}

func createRunDir(t *testing.T, base, owner, repo, timestamp string) {
	t.Helper()
	dir := filepath.Join(base, owner, repo, timestamp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating run dir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("writing placeholder: %v", err)
	}
}
