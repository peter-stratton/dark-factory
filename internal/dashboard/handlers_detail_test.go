package dashboard_test

import (
	"encoding/json"
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

// writeIssueFiles creates the per-issue data files under the run directory.
func writeIssueFiles(t *testing.T, runDir string, issueNum int, outcome rundata.Outcome, implement, qualityReview, funcReview rundata.StepResult) {
	t.Helper()
	issueDir := filepath.Join(runDir, "issues", strconv.Itoa(issueNum))
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"), outcome)
	if hasAnyData(implement) {
		writeJSON(t, filepath.Join(issueDir, "implement.json"), implement)
	}
	if hasAnyData(qualityReview) {
		writeJSON(t, filepath.Join(issueDir, "quality-review.json"), qualityReview)
	}
	if hasAnyData(funcReview) {
		writeJSON(t, filepath.Join(issueDir, "functional-review.json"), funcReview)
	}
}

func hasAnyData(s rundata.StepResult) bool {
	return s.Output != "" || s.Error != "" || s.DurationSeconds > 0
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling JSON for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// buildRunDir creates a run directory structure and returns the run directory path.
func buildRunDir(t *testing.T, baseDir, owner, repo, timestamp string, meta rundata.RunMeta) string {
	t.Helper()
	dir := filepath.Join(baseDir, ".godark", "runs", owner, repo, timestamp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("creating run dir: %v", err)
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshaling run meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), data, 0o644); err != nil {
		t.Fatalf("writing run.json: %v", err)
	}
	return dir
}

// --- Run detail tests ---

func TestServer_RunDetail_Populated(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "myrepo", ts, rundata.RunMeta{
		Repo:         "acme/myrepo",
		Milestone:    "v1.0",
		IssueNumbers: []int{1, 2},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 1,
		rundata.Outcome{IssueNumber: 1, Status: "implemented", PRNumber: 10},
		rundata.StepResult{Output: "impl output", DurationSeconds: 30},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	writeIssueFiles(t, runDir, 2,
		rundata.Outcome{IssueNumber: 2, Status: "failed"},
		rundata.StepResult{Output: "impl output 2", DurationSeconds: 15},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/myrepo/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "#1") {
		t.Errorf("body missing issue #1; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "#2") {
		t.Errorf("body missing issue #2")
	}
}

func TestServer_RunDetail_StatusColorCoding(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{10, 20},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 10,
		rundata.Outcome{IssueNumber: 10, Status: "implemented"},
		rundata.StepResult{Output: "ok", DurationSeconds: 5},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	writeIssueFiles(t, runDir, 20,
		rundata.Outcome{IssueNumber: 20, Status: "failed"},
		rundata.StepResult{Error: "compilation error", DurationSeconds: 3},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "badge--success") {
		t.Errorf("body missing badge--success for implemented issue; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "badge--danger") {
		t.Errorf("body missing badge--danger for failed issue")
	}
}

func TestServer_RunDetail_PRLink(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "owner", "repo", ts, rundata.RunMeta{
		Repo:         "owner/repo",
		Milestone:    "v2.0",
		IssueNumbers: []int{7},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 7,
		rundata.Outcome{IssueNumber: 7, Status: "implemented", PRNumber: 57},
		rundata.StepResult{Output: "done", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/owner/repo/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "github.com/owner/repo/pull/57") {
		t.Errorf("body missing correct GitHub PR link; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "#57") {
		t.Errorf("body missing PR number #57")
	}
}

func TestServer_RunDetail_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/runs/acme/nope/20240101-000000", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_RunDetail_Breadcrumbs(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	buildRunDir(t, tmpDir, "myorg", "myproj", ts, rundata.RunMeta{
		Repo:      "myorg/myproj",
		Milestone: "v3.0",
		StartedAt: now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/myorg/myproj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `href="/"`) {
		t.Errorf("body missing breadcrumb link to Runs index")
	}
	if !strings.Contains(body, ts) {
		t.Errorf("body missing timestamp in breadcrumbs")
	}
}

// --- Issue detail tests ---

func TestServer_IssueDetail_Timeline(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{5},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "5")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 5, Status: "implemented", PRNumber: 99})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "impl trace", DurationSeconds: 45})
	writeJSON(t, filepath.Join(issueDir, "quality-review.json"),
		rundata.StepResult{Output: "quality trace", DurationSeconds: 12})
	writeJSON(t, filepath.Join(issueDir, "functional-review.json"),
		rundata.StepResult{Output: "functional trace", DurationSeconds: 8})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/5", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	// Check all steps appear in order
	idxImpl := strings.Index(body, "Implement")
	idxQA := strings.Index(body, "Quality Review")
	idxFR := strings.Index(body, "Functional Review")

	if idxImpl < 0 {
		t.Error("body missing Implement step")
	}
	if idxQA < 0 {
		t.Error("body missing Quality Review step")
	}
	if idxFR < 0 {
		t.Error("body missing Functional Review step")
	}
	if idxImpl >= 0 && idxQA >= 0 && idxImpl > idxQA {
		t.Error("Implement should appear before Quality Review")
	}
	if idxQA >= 0 && idxFR >= 0 && idxQA > idxFR {
		t.Error("Quality Review should appear before Functional Review")
	}
}

func TestServer_IssueDetail_ToolTraceToggle(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{3},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "3")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 3, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "tool trace output here", DurationSeconds: 20})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/3", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Alpine.js toggle attributes must be present
	if !strings.Contains(body, "x-data") {
		t.Errorf("body missing Alpine.js x-data attribute for tool trace toggle")
	}
	if !strings.Contains(body, "x-show") {
		t.Errorf("body missing Alpine.js x-show attribute for tool trace toggle")
	}
	if !strings.Contains(body, "@click") {
		t.Errorf("body missing Alpine.js @click attribute for tool trace toggle")
	}
	if !strings.Contains(body, "Tool Trace") {
		t.Errorf("body missing 'Tool Trace' trigger label")
	}
	if !strings.Contains(body, "tool trace output here") {
		t.Errorf("body missing tool trace content")
	}
}

func TestServer_IssueDetail_PRAndIssueLinks(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "org", "repo", ts, rundata.RunMeta{
		Repo:         "org/repo",
		Milestone:    "v1.0",
		IssueNumbers: []int{42},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "42")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 42, Status: "implemented", PRNumber: 57})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "done", DurationSeconds: 10})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/org/repo/"+ts+"/issues/42", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "github.com/org/repo/pull/57") {
		t.Errorf("body missing PR link; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "github.com/org/repo/issues/42") {
		t.Errorf("body missing GitHub issue link")
	}
}

func TestServer_IssueDetail_NotFound_BadRun(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/runs/acme/nope/20240101-000000/issues/1", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_IssueDetail_NotFound_BadIssue(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/999", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_IssueDetail_Breadcrumbs(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "co", "app", ts, rundata.RunMeta{
		Repo:         "co/app",
		Milestone:    "v1.0",
		IssueNumbers: []int{8},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "8")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 8, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "done", DurationSeconds: 5})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/co/app/"+ts+"/issues/8", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	// Breadcrumbs: Runs / run / #8
	if !strings.Contains(body, `href="/"`) {
		t.Errorf("body missing Runs breadcrumb link")
	}
	runDetailURL := "/runs/co/app/" + ts
	if !strings.Contains(body, runDetailURL) {
		t.Errorf("body missing run detail breadcrumb link %q", runDetailURL)
	}
	if !strings.Contains(body, "#8") {
		t.Errorf("body missing issue number in breadcrumbs")
	}
}

func TestServer_IssueDetail_WithRetries(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{11},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "11")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 11, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "first attempt", DurationSeconds: 30})
	writeJSON(t, filepath.Join(issueDir, "quality-review.json"),
		rundata.StepResult{
			Output:          "flagged",
			DurationSeconds: 10,
			Flags:           []rundata.Flag{{Code: "NO_TESTS", Message: "missing tests"}},
		})

	retryDir := filepath.Join(issueDir, "retries", "1")
	if err := os.MkdirAll(retryDir, 0o755); err != nil {
		t.Fatalf("creating retry dir: %v", err)
	}
	writeJSON(t, filepath.Join(retryDir, "retry.json"),
		rundata.StepResult{Output: "retry attempt", DurationSeconds: 25})
	writeJSON(t, filepath.Join(retryDir, "quality-review.json"),
		rundata.StepResult{Output: "looks good", DurationSeconds: 8})
	writeJSON(t, filepath.Join(issueDir, "functional-review.json"),
		rundata.StepResult{Output: "passed", DurationSeconds: 5})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/11", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	// All steps should be present
	for _, want := range []string{"Implement", "Quality Review", "Retry 1", "Functional Review"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing timeline step %q", want)
		}
	}
	// Quality flag indicator
	if !strings.Contains(body, "NO_TESTS") {
		t.Errorf("body missing quality flag code NO_TESTS")
	}
}

func TestServer_IndexRows_HaveRunDetailURL(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	writeRunMeta(t, tmpDir, "myorg", "myapp", ts, rundata.RunMeta{
		Repo:      "myorg/myapp",
		Milestone: "v1.0",
		StartedAt: now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	expectedURL := "/runs/myorg/myapp/" + ts
	// The URL appears in a data-href attribute (not in JS context, so no escaping).
	if !strings.Contains(body, `data-href="`+expectedURL+`"`) {
		t.Errorf("index body missing run detail URL %q in data-href; got: %q", expectedURL, truncate(body, 800))
	}
}
