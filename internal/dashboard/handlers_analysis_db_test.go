package dashboard_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/dashboard"
	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
)

// newServerWithStats creates a dashboard Server using an in-memory stats DB.
// The filesystem reader is pointed at tmpDir but will not be used for analysis.
func newServerWithStats(t *testing.T, tmpDir string, db *stats.DB) *dashboard.Server {
	t.Helper()
	t.Setenv("HOME", tmpDir)
	reader, err := rundata.NewReader(slog.Default())
	if err != nil {
		t.Fatalf("creating reader: %v", err)
	}
	srv, err := dashboard.New(dashboard.Config{Port: 8374, Logger: slog.Default()}, reader, db)
	if err != nil {
		t.Fatalf("creating server with stats db: %v", err)
	}
	return srv
}

// openTestStatsDB opens an in-memory stats.DB for use in tests.
func openTestStatsDB(t *testing.T) *stats.DB {
	t.Helper()
	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("opening in-memory stats db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// writeStatsRun inserts a complete run record with issue outcomes and step results.
func writeStatsRun(t *testing.T, db *stats.DB, run stats.RunRecord, outcomes []stats.IssueOutcomeRecord, steps []stats.StepResultRecord) {
	t.Helper()
	ctx := context.Background()
	if err := stats.WriteRun(ctx, db, run); err != nil {
		t.Fatalf("writing run: %v", err)
	}
	for _, o := range outcomes {
		if err := stats.WriteIssueOutcome(ctx, db, o); err != nil {
			t.Fatalf("writing issue outcome: %v", err)
		}
	}
	for _, s := range steps {
		if err := stats.WriteStepResult(ctx, db, s); err != nil {
			t.Fatalf("writing step result: %v", err)
		}
	}
}

func TestServer_Analysis_DB_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	db := openTestStatsDB(t)
	srv := newServerWithStats(t, tmpDir, db)

	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "No run data available") {
		t.Errorf("body missing empty state message; got: %q", truncate(body, 400))
	}
}

func TestServer_Analysis_DB_StatsDisplayed(t *testing.T) {
	tmpDir := t.TempDir()
	db := openTestStatsDB(t)

	now := time.Now().UTC()
	runID := now.Format("20060102-150405")
	finished := now.Add(5 * time.Minute)

	writeStatsRun(t, db,
		stats.RunRecord{
			ID:          runID,
			Repo:        "acme/proj",
			Milestone:   "v1.0",
			StartedAt:   now,
			FinishedAt:  finished,
			Total:       2,
			Implemented: 1,
			Failed:      1,
		},
		[]stats.IssueOutcomeRecord{
			{RunID: runID, IssueNumber: 1, Title: "Issue One", Status: "implemented"},
			{RunID: runID, IssueNumber: 2, Title: "Issue Two", Status: "failed"},
		},
		[]stats.StepResultRecord{
			{RunID: runID, IssueNumber: 1, StepName: "implement", DurationSeconds: 10, StartedAt: now, FinishedAt: now.Add(10 * time.Second)},
			{RunID: runID, IssueNumber: 2, StepName: "implement", DurationSeconds: 5, StartedAt: now, FinishedAt: now.Add(5 * time.Second)},
		},
	)

	srv := newServerWithStats(t, tmpDir, db)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	if !strings.Contains(body, "implemented") {
		t.Errorf("body missing outcome 'implemented'; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "failed") {
		t.Errorf("body missing outcome 'failed'")
	}
	if !strings.Contains(body, "2 issues") {
		t.Errorf("body missing issue count '2 issues'; got: %q", truncate(body, 500))
	}
}

func TestServer_Analysis_DB_RepoFilter(t *testing.T) {
	tmpDir := t.TempDir()
	db := openTestStatsDB(t)

	now := time.Now().UTC()
	runAID := now.Format("20060102-150405")
	runBID := now.Add(-time.Second).Format("20060102-150405")
	finished := now.Add(time.Minute)

	// Repo A: one implemented issue.
	writeStatsRun(t, db,
		stats.RunRecord{
			ID:          runAID,
			Repo:        "acme/repo-a",
			Milestone:   "v1.0",
			StartedAt:   now,
			FinishedAt:  finished,
			Total:       1,
			Implemented: 1,
		},
		[]stats.IssueOutcomeRecord{
			{RunID: runAID, IssueNumber: 1, Status: "implemented"},
		},
		nil,
	)

	// Repo B: one failed issue.
	writeStatsRun(t, db,
		stats.RunRecord{
			ID:         runBID,
			Repo:       "acme/repo-b",
			Milestone:  "v1.0",
			StartedAt:  now.Add(-time.Second),
			FinishedAt: finished,
			Total:      1,
			Failed:     1,
		},
		[]stats.IssueOutcomeRecord{
			{RunID: runBID, IssueNumber: 2, Status: "failed"},
		},
		nil,
	)

	srv := newServerWithStats(t, tmpDir, db)

	req := httptest.NewRequest(http.MethodGet, "/analysis?repo=acme%2Frepo-a", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	if !strings.Contains(body, "1 issues across 1 runs") {
		t.Errorf("body should show 1 issue across 1 run for repo-a; got: %q", truncate(body, 500))
	}
}

func TestServer_Analysis_DB_RepoBothPresent(t *testing.T) {
	tmpDir := t.TempDir()
	db := openTestStatsDB(t)

	now := time.Now().UTC()
	runAID := now.Format("20060102-150405")
	runBID := now.Add(-time.Second).Format("20060102-150405")
	finished := now.Add(time.Minute)

	writeStatsRun(t, db,
		stats.RunRecord{ID: runAID, Repo: "org/alpha", StartedAt: now, FinishedAt: finished},
		nil, nil,
	)
	writeStatsRun(t, db,
		stats.RunRecord{ID: runBID, Repo: "org/beta", StartedAt: now.Add(-time.Second), FinishedAt: finished},
		nil, nil,
	)

	srv := newServerWithStats(t, tmpDir, db)

	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Both repos should appear in the dropdown.
	if !strings.Contains(body, "org/alpha") {
		t.Errorf("body missing org/alpha in repo dropdown; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "org/beta") {
		t.Errorf("body missing org/beta in repo dropdown; got: %q", truncate(body, 500))
	}
}

func TestServer_Analysis_DB_PartialEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	db := openTestStatsDB(t)

	now := time.Now().UTC()
	runID := now.Format("20060102-150405")
	finished := now.Add(time.Minute)

	writeStatsRun(t, db,
		stats.RunRecord{
			ID:          runID,
			Repo:        "acme/proj",
			Milestone:   "v1.0",
			StartedAt:   now,
			FinishedAt:  finished,
			Total:       1,
			Implemented: 1,
		},
		[]stats.IssueOutcomeRecord{
			{RunID: runID, IssueNumber: 1, Status: "implemented"},
		},
		nil,
	)

	srv := newServerWithStats(t, tmpDir, db)
	req := httptest.NewRequest(http.MethodGet, "/partials/analysis-stats", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "implemented") {
		t.Errorf("partial body missing outcome data; got: %q", truncate(body, 400))
	}
}

func TestServer_Analysis_DB_TrendCharts(t *testing.T) {
	tmpDir := t.TempDir()
	db := openTestStatsDB(t)

	// Insert 5 finished runs so HasTrends=true.
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ts := now.Add(time.Duration(-i) * time.Hour)
		runID := ts.Format("20060102-150405") + "-" + string(rune('0'+i))
		finished := ts.Add(30 * time.Minute)
		writeStatsRun(t, db,
			stats.RunRecord{
				ID:          runID,
				Repo:        "acme/proj",
				Milestone:   "v1.0",
				StartedAt:   ts,
				FinishedAt:  finished,
				Total:       1,
				Implemented: 1,
			},
			[]stats.IssueOutcomeRecord{
				{RunID: runID, IssueNumber: 1, Status: "implemented"},
			},
			nil,
		)
	}

	srv := newServerWithStats(t, tmpDir, db)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<canvas") {
		t.Errorf("analysis page with 5 DB runs missing <canvas> elements; got: %q", truncate(body, 600))
	}
}
