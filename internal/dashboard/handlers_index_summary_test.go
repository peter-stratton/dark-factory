package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

func TestServer_Index_SummaryCards_Displayed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1, 2},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 2, Implemented: 2, Failed: 0},
	}, []rundata.IssueDetail{
		{IssueNumber: 1, Outcome: rundata.Outcome{IssueNumber: 1, Status: "implemented"}},
		{IssueNumber: 2, Outcome: rundata.Outcome{IssueNumber: 2, Status: "implemented"}},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "Total Runs") {
		t.Errorf("body missing 'Total Runs' card label; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "Total Issues") {
		t.Errorf("body missing 'Total Issues' card label")
	}
	if !strings.Contains(body, "Success Rate") {
		t.Errorf("body missing 'Success Rate' card label")
	}
	if !strings.Contains(body, "Avg Cost") {
		t.Errorf("body missing 'Avg Cost' card label")
	}
}

func TestServer_Index_SuccessRateCorrect(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	// 8 implemented, 2 failed => 80% success rate
	issues := make([]rundata.IssueDetail, 10)
	issueNums := make([]int, 10)
	for i := 0; i < 10; i++ {
		status := "implemented"
		if i >= 8 {
			status = "failed"
		}
		issues[i] = rundata.IssueDetail{
			IssueNumber: i + 1,
			Outcome:     rundata.Outcome{IssueNumber: i + 1, Status: status},
		}
		issueNums[i] = i + 1
	}

	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: issueNums,
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 10, Implemented: 8, Failed: 2},
	}, issues)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "80%") {
		t.Errorf("body missing '80%%' success rate; got: %q", truncate(body, 600))
	}
}

func TestServer_Index_CostDisplayed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}, []rundata.IssueDetail{
		{
			IssueNumber: 1,
			Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 5, CostUSD: 0.0042},
		},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "$0.0042") {
		t.Errorf("body missing cost '$0.0042'; got: %q", truncate(body, 600))
	}
}

func TestServer_Index_CardsLinkToAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}, []rundata.IssueDetail{
		{IssueNumber: 1, Outcome: rundata.Outcome{IssueNumber: 1, Status: "implemented"}},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Count occurrences of /analysis link in the summary section
	if !strings.Contains(body, `href="/analysis"`) {
		t.Errorf("body missing link to /analysis; got: %q", truncate(body, 400))
	}
	// The stats-row section should contain multiple links to /analysis
	statsRowIdx := strings.Index(body, "stats-row")
	if statsRowIdx < 0 {
		t.Errorf("body missing stats-row section; got: %q", truncate(body, 500))
	}
	statsSection := body[statsRowIdx:]
	count := strings.Count(statsSection[:strings.Index(statsSection, "</div>")+6], `href="/analysis"`)
	if count == 0 {
		t.Errorf("stats-row section missing /analysis links; got: %q", truncate(statsSection, 400))
	}
}

func TestServer_Index_EmptyState_HidesSummary(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if strings.Contains(body, "Total Runs") {
		t.Errorf("empty state should not show 'Total Runs' summary card")
	}
	if strings.Contains(body, "stats-row") {
		t.Errorf("empty state should not show stats-row section")
	}
	if !strings.Contains(body, "Welcome to godark") {
		t.Errorf("empty state should show welcome message")
	}
}

func TestServer_Index_MiniChart_ShownWithTrends(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	// Build 2 finished runs so HasTrends=true
	for i := 0; i < 2; i++ {
		ts := now.Add(time.Duration(-i) * time.Hour).Format("20060102-150405")
		fa := now.Add(time.Duration(-i) * time.Hour)
		buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
			Repo:         "acme/proj",
			Milestone:    "v1.0",
			IssueNumbers: []int{1},
			StartedAt:    now.Add(time.Duration(-i) * time.Hour),
			FinishedAt:   &fa,
			Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
		}, []rundata.IssueDetail{
			{
				IssueNumber: 1,
				Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
				Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
			},
		})
	}

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "<canvas") {
		t.Errorf("homepage with 2 finished runs missing <canvas> element for sparkline; got: %q", truncate(body, 600))
	}
	if !strings.Contains(body, "index-chart-success-rate") {
		t.Errorf("homepage missing sparkline canvas id 'index-chart-success-rate'")
	}
}

func TestServer_Index_MiniChart_HiddenWithSingleRun(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	finishedAt := now

	buildAnalysisRun(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1},
	}, []rundata.IssueDetail{
		{
			IssueNumber: 1,
			Outcome:     rundata.Outcome{IssueNumber: 1, Status: "implemented"},
			Implement:   rundata.StepResult{Output: "done", DurationSeconds: 10},
		},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// With only 1 run, sparkline should not be shown
	if strings.Contains(body, "index-chart-success-rate") {
		t.Errorf("homepage with 1 run should not show sparkline canvas")
	}
	// But summary cards should still be present
	if !strings.Contains(body, "Total Runs") {
		t.Errorf("homepage with 1 run should still show summary cards; got: %q", truncate(body, 500))
	}
}
