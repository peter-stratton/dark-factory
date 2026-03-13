package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// writeReconFile creates a recon.json under the issue directory.
func writeReconFile(t *testing.T, issueDir string, step rundata.StepResult) {
	t.Helper()
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "recon.json"), step)
}

func TestServer_ReviewChain_WithReconStep(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{8},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 8,
		rundata.Outcome{IssueNumber: 8, Status: "implemented", PRNumber: 42},
		rundata.StepResult{Output: "impl done", DurationSeconds: 60},
		rundata.StepResult{},
		rundata.StepResult{Output: "review done AGENT_RESULT=APPROVED", DurationSeconds: 15},
	)
	issueDir := filepath.Join(runDir, "issues", "8")
	writeReconFile(t, issueDir, rundata.StepResult{
		Output:          "Recon brief: the codebase uses X pattern",
		CostUSD:         0.0025,
		DurationSeconds: 18.5,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/8/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	if !strings.Contains(body, "Recon") {
		t.Error("body missing Recon step")
	}
	if !strings.Contains(body, "Implement") {
		t.Error("body missing Implement step")
	}
}

func TestServer_ReviewChain_WithoutReconStep(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{9},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 9,
		rundata.Outcome{IssueNumber: 9, Status: "implemented"},
		rundata.StepResult{Output: "impl done AGENT_RESULT=APPROVED", DurationSeconds: 30},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	// No recon.json written — simulates a pre-Phase-18 run.

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/9/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	if strings.Contains(body, ">Recon<") {
		t.Error("body should not contain Recon step when no recon data exists")
	}
	if !strings.Contains(body, "Implement") {
		t.Error("body missing Implement step")
	}
}

func TestServer_ReviewChain_WithSteps(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{5},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 5,
		rundata.Outcome{IssueNumber: 5, Status: "implemented", PRNumber: 99},
		rundata.StepResult{Output: "impl trace AGENT_RESULT=APPROVED", DurationSeconds: 45},
		rundata.StepResult{Output: "quality trace AGENT_RESULT=APPROVED", DurationSeconds: 12},
		rundata.StepResult{Output: "functional trace AGENT_RESULT=APPROVED", DurationSeconds: 8},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/5/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	if !strings.Contains(body, "Implement") {
		t.Error("body missing Implement step")
	}
	if !strings.Contains(body, "Quality Review") {
		t.Error("body missing Quality Review step")
	}
	if !strings.Contains(body, "Functional Review") {
		t.Error("body missing Functional Review step")
	}
	// Steps with APPROVED should show Passed verdict
	if !strings.Contains(body, "Passed") {
		t.Error("body missing Passed verdict")
	}
}

func TestServer_ReviewChain_EmptyState(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{7},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 7,
		rundata.Outcome{IssueNumber: 7, Status: "pending"},
		rundata.StepResult{},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/7/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "No review chain data recorded") {
		t.Errorf("body missing empty-state message; got: %q", truncate(body, 300))
	}
}

func TestServer_ReviewChain_NotFound_MissingRun(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/runs/acme/nope/20240101-000000/issues/1/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_ReviewChain_NotFound_MissingIssue(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/999/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_ReviewChain_NotFound_BadIssueNumber(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/notanumber/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_ReviewChain_ContentType(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{3},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 3,
		rundata.Outcome{IssueNumber: 3, Status: "implemented"},
		rundata.StepResult{Output: "done", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/3/review-chain", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want to contain text/html", ct)
	}
}

func TestServer_IssueDetail_HasPollingAttributes(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{5},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 5,
		rundata.Outcome{IssueNumber: 5, Status: "implemented"},
		rundata.StepResult{Output: "impl trace", DurationSeconds: 45},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/5", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	// Verify HTMX polling attributes are present on the review chain container.
	if !strings.Contains(body, "review-chain-body") {
		t.Error("body missing review-chain-body id")
	}
	if !strings.Contains(body, "hx-get") {
		t.Error("body missing hx-get polling attribute")
	}
	if !strings.Contains(body, "/review-chain") {
		t.Error("body missing review-chain URL in hx-get")
	}
	if !strings.Contains(body, "every 5s") {
		t.Error("body missing 5s polling trigger")
	}
	if !strings.Contains(body, "htmx.min.js") {
		t.Error("body missing htmx.min.js script tag")
	}
}
