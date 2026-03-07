package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// buildAnalysisRun creates a full run with issue data for analysis tests.
// Returns the run directory path.
func buildAnalysisRun(t *testing.T, baseDir, owner, repo, timestamp string, meta rundata.RunMeta, issues []rundata.IssueDetail) string {
	t.Helper()
	runDir := buildRunDir(t, baseDir, owner, repo, timestamp, meta)
	for _, issue := range issues {
		issueDir := filepath.Join(runDir, "issues", itoa(issue.IssueNumber))
		if err := os.MkdirAll(issueDir, 0o755); err != nil {
			t.Fatalf("creating issue dir: %v", err)
		}
		writeJSON(t, filepath.Join(issueDir, "outcome.json"), issue.Outcome)
		if hasAnyData(issue.Implement) {
			writeJSON(t, filepath.Join(issueDir, "implement.json"), issue.Implement)
		}
		if hasAnyData(issue.QualityReview) {
			writeJSON(t, filepath.Join(issueDir, "quality-review.json"), issue.QualityReview)
		}
	}
	return runDir
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func TestServer_Analysis_EmptyState(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

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

func TestServer_Analysis_ReturnsHTML(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestServer_Analysis_StatsDisplayed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	issues := []rundata.IssueDetail{
		{
			IssueNumber: 1,
			Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
		},
		{
			IssueNumber: 2,
			Outcome:     rundata.Outcome{IssueNumber: 2, Status: "failed"},
			Implement:   rundata.StepResult{Output: "err", DurationSeconds: 5},
		},
	}

	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1, 2},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 2, Implemented: 1, Failed: 1},
	}, issues)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	// Outcome counts should appear.
	if !strings.Contains(body, "implemented") {
		t.Errorf("body missing outcome 'implemented'; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "failed") {
		t.Errorf("body missing outcome 'failed'")
	}
	// Run/issue summary counts.
	if !strings.Contains(body, "2 issues") {
		t.Errorf("body missing issue count '2 issues'; got: %q", truncate(body, 500))
	}
}

func TestServer_Analysis_FlagTable(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	issues := []rundata.IssueDetail{
		{
			IssueNumber: 1,
			Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
			QualityReview: rundata.StepResult{
				Output:          "flagged",
				DurationSeconds: 5,
				Flags: []rundata.Flag{
					{Code: "MISSING_TESTS", Message: "no tests found"},
				},
			},
		},
	}

	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}, issues)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	if !strings.Contains(body, "MISSING_TESTS") {
		t.Errorf("body missing flag code 'MISSING_TESTS'; got: %q", truncate(body, 500))
	}
}

func TestServer_Analysis_GapsSection(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	finishedAt := now

	// Create enough issues to trigger gap detection (need >= 3 on each side).
	// Issues WITH quality review that fail, and issues WITHOUT that pass.
	var issues []rundata.IssueDetail
	for i := 1; i <= 4; i++ {
		// With quality review (failing).
		issues = append(issues, rundata.IssueDetail{
			IssueNumber: i,
			Outcome:     rundata.Outcome{IssueNumber: i, Status: "failed"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
			QualityReview: rundata.StepResult{
				Output:          "flagged",
				DurationSeconds: 5,
			},
		})
	}
	for i := 5; i <= 8; i++ {
		// Without quality review (passing).
		issues = append(issues, rundata.IssueDetail{
			IssueNumber: i,
			Outcome:     rundata.Outcome{IssueNumber: i, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
		})
	}

	issueNums := make([]int, len(issues))
	for i, iss := range issues {
		issueNums[i] = iss.IssueNumber
	}

	ts := now.Format("20060102-150405")
	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: issueNums,
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: len(issues), Implemented: 4, Failed: 4},
	}, issues)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 400))
	}
	body := rr.Body.String()

	// The gap finding text should appear.
	if !strings.Contains(body, "quality reviewer") {
		t.Errorf("body missing prompt gap finding 'quality reviewer'; got: %q", truncate(body, 600))
	}
	// The Prompt Gaps card heading should be present.
	if !strings.Contains(body, "Prompt Gaps") {
		t.Errorf("body missing 'Prompt Gaps' section heading")
	}
}

func TestServer_Analysis_RepoFilter(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	finishedAt := now

	tsA := now.Format("20060102-150405")
	tsB := now.Add(-time.Second).Format("20060102-150405")

	issuesA := []rundata.IssueDetail{
		{
			IssueNumber: 1,
			Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 5},
		},
	}
	issuesB := []rundata.IssueDetail{
		{
			IssueNumber: 2,
			Outcome:     rundata.Outcome{IssueNumber: 2, Status: "failed"},
			Implement:   rundata.StepResult{Output: "err", DurationSeconds: 5},
		},
	}

	buildAnalysisRun(t, tmpDir, "acme", "repo-a", tsA, rundata.RunMeta{
		Repo:         "acme/repo-a",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}, issuesA)

	buildAnalysisRun(t, tmpDir, "acme", "repo-b", tsB, rundata.RunMeta{
		Repo:         "acme/repo-b",
		Milestone:    "v1.0",
		IssueNumbers: []int{2},
		StartedAt:    now.Add(-time.Second),
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Failed: 1},
	}, issuesB)

	srv := newServer(t, tmpDir)

	// Filter to repo-a only — should see "implemented" but not "failed".
	req := httptest.NewRequest(http.MethodGet, "/analysis?repo=acme%2Frepo-a", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	// Only repo-a's outcome "implemented" should be present.
	if !strings.Contains(body, "implemented") {
		t.Errorf("body missing outcome 'implemented' for repo-a; got: %q", truncate(body, 500))
	}
	// "failed" outcome from repo-b should not appear in the outcome table.
	// The word "failed" may appear in sidebar or status labels so check carefully.
	if !strings.Contains(body, "1 issues across 1 runs") {
		t.Errorf("body should show 1 issue across 1 run for filtered repo; got: %q", truncate(body, 500))
	}
}

func TestServer_Analysis_PartialEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	issues := []rundata.IssueDetail{
		{
			IssueNumber: 1,
			Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
		},
	}
	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}, issues)

	srv := newServer(t, tmpDir)
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

func TestServer_ChartJSServed(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/static/chart.min.js", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /static/chart.min.js: status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, want to contain javascript", ct)
	}
	if rr.Body.Len() == 0 {
		t.Error("chart.min.js response body is empty")
	}
}

func TestServer_Analysis_ChartsRendered(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	// Build 5 finished runs so HasTrends=true.
	for i := 0; i < 5; i++ {
		ts := now.Add(time.Duration(-i) * time.Hour).Format("20060102-150405")
		issues := []rundata.IssueDetail{
			{
				IssueNumber: 1,
				Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
				Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
			},
		}
		fa := now.Add(time.Duration(-i) * time.Hour)
		buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
			Repo:         "acme/proj",
			Milestone:    "v1.0",
			IssueNumbers: []int{1},
			StartedAt:    now.Add(time.Duration(-i) * time.Hour),
			FinishedAt:   &fa,
			Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
		}, issues)
	}

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<canvas") {
		t.Errorf("analysis page with 5 runs missing <canvas> elements; got: %q", truncate(body, 600))
	}
}

func TestServer_Analysis_TrendDataEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	// Build 2 finished runs so trend JSON is embedded.
	for i := 0; i < 2; i++ {
		ts := now.Add(time.Duration(-i) * time.Hour).Format("20060102-150405")
		fa := now.Add(time.Duration(-i) * time.Hour)
		issues := []rundata.IssueDetail{
			{
				IssueNumber: 1,
				Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
				Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
			},
		}
		buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
			Repo:         "acme/proj",
			Milestone:    "v1.0",
			IssueNumbers: []int{1},
			StartedAt:    now.Add(time.Duration(-i) * time.Hour),
			FinishedAt:   &fa,
			Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
		}, issues)
	}

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	// The page should contain the JSON trend data script tag.
	if !strings.Contains(body, `id="trend-data"`) {
		t.Errorf("analysis page missing trend-data script tag; got: %q", truncate(body, 600))
	}
	// The JSON should contain the success_rate field.
	if !strings.Contains(body, "success_rate") {
		t.Errorf("trend data JSON missing success_rate field; got: %q", truncate(body, 600))
	}
}

func TestServer_Analysis_NoDataFallback(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Not enough data") {
		t.Errorf("analysis page with 0 runs missing 'Not enough data' message; got: %q", truncate(body, 400))
	}
}

func TestServer_Analysis_SingleRunFallback(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	issues := []rundata.IssueDetail{
		{
			IssueNumber: 1,
			Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
		},
	}
	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}, issues)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/analysis", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	// With only 1 run, charts should not be shown.
	if strings.Contains(body, "<canvas") {
		t.Errorf("analysis page with 1 run should not contain <canvas> elements")
	}
	// Fallback message should be shown.
	if !strings.Contains(body, "Not enough data") {
		t.Errorf("analysis page with 1 run missing 'Not enough data' fallback; got: %q", truncate(body, 400))
	}
}

func TestServer_Analysis_NavLinkPresent(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `href="/analysis"`) {
		t.Errorf("index page missing navigation link to /analysis; got: %q", truncate(body, 400))
	}
}
