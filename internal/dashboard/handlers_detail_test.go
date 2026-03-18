package dashboard_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/rundata"
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
		rundata.StepResult{Output: "agent output here", DurationSeconds: 20})

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
		t.Errorf("body missing Alpine.js x-data attribute for agent output toggle")
	}
	if !strings.Contains(body, "x-show") {
		t.Errorf("body missing Alpine.js x-show attribute for agent output toggle")
	}
	if !strings.Contains(body, "@click") {
		t.Errorf("body missing Alpine.js @click attribute for agent output toggle")
	}
	if !strings.Contains(body, "Agent Output") {
		t.Errorf("body missing 'Agent Output' trigger label")
	}
	if !strings.Contains(body, "agent output here") {
		t.Errorf("body missing agent output content")
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

	retryDir := filepath.Join(issueDir, "retries", "0")
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

// --- Dialogue tests ---

// writeDialogueFile writes a dialogue.json file under issueDir.
func writeDialogueFile(t *testing.T, issueDir string, entries []rundata.DialogueEntry) {
	t.Helper()
	writeJSON(t, filepath.Join(issueDir, "dialogue.json"), entries)
}

// buildIssueDir creates the issue directory and writes outcome.json, returning the issue dir path.
func buildIssueDir(t *testing.T, runDir string, issueNum int) string {
	t.Helper()
	issueDir := filepath.Join(runDir, "issues", strconv.Itoa(issueNum))
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: issueNum, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "done", DurationSeconds: 5})
	return issueDir
}

func TestServer_IssueDetail_DialogueDisplayed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{20},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 20)
	writeDialogueFile(t, issueDir, []rundata.DialogueEntry{
		{Role: "implementer", Round: 1, Body: "implementation note here"},
		{Role: "reviewer", Round: 1, Body: "review comment here"},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/20", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Dialogue") {
		t.Errorf("body missing Dialogue section; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "implementation note here") {
		t.Errorf("body missing implementer body text")
	}
	if !strings.Contains(body, "review comment here") {
		t.Errorf("body missing reviewer body text")
	}
}

func TestServer_IssueDetail_NoDialogue(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{21},
		StartedAt:    now,
	})
	buildIssueDir(t, runDir, 21)
	// No dialogue.json written.

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/21", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	// The Dialogue card should not be rendered when there are no entries.
	// We check that the heading text doesn't appear inside a card context.
	// The word "Dialogue" in the page title is fine, so we check for the
	// card__title span which is only present when the section is rendered.
	if strings.Contains(body, `card__title">Dialogue`) {
		t.Errorf("body should not contain Dialogue card when no entries exist")
	}
}

func TestServer_IssueDetail_RolesStyled(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{22},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 22)
	writeDialogueFile(t, issueDir, []rundata.DialogueEntry{
		{Role: "implementer", Round: 1, Body: "implementer text"},
		{Role: "reviewer", Round: 1, Body: "reviewer text"},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/22", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	// Each role appears as a capitalized label in the summary.
	if !strings.Contains(body, "Implementer") {
		t.Errorf("body missing role label 'Implementer'")
	}
	if !strings.Contains(body, "Functional Reviewer") {
		t.Errorf("body missing role label 'Functional Reviewer'")
	}
	// Distinct visual styling: implementer uses color-accent, reviewer uses color-warning.
	if !strings.Contains(body, "color-border") {
		t.Errorf("body missing implementer border style (color-border)")
	}
	if !strings.Contains(body, "color-success") {
		t.Errorf("body missing reviewer border style (color-success)")
	}
}

func TestServer_IssueDetail_DialogueExpandable(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{23},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 23)
	writeDialogueFile(t, issueDir, []rundata.DialogueEntry{
		{Role: "implementer", Round: 1, Body: "expandable body content"},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/23", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<details") {
		t.Errorf("body missing <details> element for expandable dialogue body")
	}
	if !strings.Contains(body, "<summary") {
		t.Errorf("body missing <summary> element inside <details>")
	}
	if !strings.Contains(body, "expandable body content") {
		t.Errorf("body missing dialogue body content inside <details>")
	}
}

func TestServer_IssueDetail_MultipleRounds(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{24},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 24)
	writeDialogueFile(t, issueDir, []rundata.DialogueEntry{
		{Role: "implementer", Round: 1, Body: "impl round 1"},
		{Role: "reviewer", Round: 1, Body: "review round 1"},
		{Role: "implementer", Round: 2, Body: "impl round 2"},
		{Role: "reviewer", Round: 2, Body: "review round 2"},
		{Role: "implementer", Round: 3, Body: "impl round 3"},
		{Role: "reviewer", Round: 3, Body: "review round 3"},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/24", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// All six entries should appear.
	for _, want := range []string{
		"impl round 1", "review round 1",
		"impl round 2", "review round 2",
		"impl round 3", "review round 3",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing dialogue entry %q", want)
		}
	}

	// Verify ordering: round 1 appears before round 2, round 2 before round 3.
	idx1 := strings.Index(body, "impl round 1")
	idx2 := strings.Index(body, "impl round 2")
	idx3 := strings.Index(body, "impl round 3")
	if idx1 < 0 || idx2 < 0 || idx3 < 0 {
		t.Fatal("could not find all round markers for ordering check")
	}
	if idx1 > idx2 {
		t.Errorf("round 1 should appear before round 2")
	}
	if idx2 > idx3 {
		t.Errorf("round 2 should appear before round 3")
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

func TestServer_IssueDetail_ToolTraceSection(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{6},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "6")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 6, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "functional-review.json"),
		rundata.StepResult{
			Output:          "AGENT_RESULT=APPROVED",
			ToolTrace:       []string{"Read src/main.go", "Write tests/review/test_main.go", "go test ./..."},
			DurationSeconds: 15,
		})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/6", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	// Tool Trace section should be present with the call count.
	if !strings.Contains(body, "Tool Trace (3 calls)") {
		t.Errorf("body missing 'Tool Trace (3 calls)' trigger label; got: %q", truncate(body, 500))
	}
	// Each tool call entry should appear.
	if !strings.Contains(body, "Read src/main.go") {
		t.Errorf("body missing first tool trace entry 'Read src/main.go'")
	}
	if !strings.Contains(body, "Write tests/review/test_main.go") {
		t.Errorf("body missing second tool trace entry 'Write tests/review/test_main.go'")
	}
	if !strings.Contains(body, "go test ./...") {
		t.Errorf("body missing third tool trace entry 'go test ./...'")
	}
	// Alpine.js toggle should be present.
	if !strings.Contains(body, "traceOpen") {
		t.Errorf("body missing Alpine.js traceOpen variable for tool trace toggle")
	}
}

func TestServer_IssueDetail_RetryFunctionalReviewInTimeline(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{15},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "15")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 15, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "impl output", DurationSeconds: 30})

	// Retry 0: pre-retry functional review (CHANGES_REQUESTED) + retry implementer
	retryDir := filepath.Join(issueDir, "retries", "0")
	if err := os.MkdirAll(retryDir, 0o755); err != nil {
		t.Fatalf("creating retry dir: %v", err)
	}
	writeJSON(t, filepath.Join(retryDir, "functional-review.json"),
		rundata.StepResult{Output: "AGENT_RESULT=CHANGES_REQUESTED", DurationSeconds: 8})
	writeJSON(t, filepath.Join(retryDir, "retry.json"),
		rundata.StepResult{Output: "retry output", DurationSeconds: 25})

	// Final functional review (APPROVED)
	writeJSON(t, filepath.Join(issueDir, "functional-review.json"),
		rundata.StepResult{Output: "AGENT_RESULT=APPROVED", DurationSeconds: 7})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/15", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	// All steps should be present.
	for _, want := range []string{"Implement", "Functional Review", "Retry 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing timeline step %q", want)
		}
	}

	// The pre-retry functional review (Changes Requested) should appear before Retry 1.
	idxFR := strings.Index(body, "AGENT_RESULT=CHANGES_REQUESTED")
	idxRetry := strings.Index(body, "retry output")
	idxFinal := strings.Index(body, "AGENT_RESULT=APPROVED")

	if idxFR < 0 {
		t.Error("body missing pre-retry functional review output")
	}
	if idxRetry < 0 {
		t.Error("body missing retry implementer output")
	}
	if idxFinal < 0 {
		t.Error("body missing final functional review output")
	}
	if idxFR >= 0 && idxRetry >= 0 && idxFR > idxRetry {
		t.Error("pre-retry Functional Review should appear before Retry 1 in timeline")
	}
	if idxRetry >= 0 && idxFinal >= 0 && idxRetry > idxFinal {
		t.Error("Retry 1 should appear before final Functional Review in timeline")
	}
}

func TestServer_IssueDetail_NoToolTraceWhenAbsent(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{7},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "7")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 7, Status: "implemented"})
	// StepResult with no ToolTrace field.
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "done", DurationSeconds: 5})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/7", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	// The "Tool Trace (N calls)" label should not appear when ToolTrace is empty.
	if strings.Contains(body, "Tool Trace (") {
		t.Errorf("body should not contain 'Tool Trace (N calls)' when ToolTrace is absent")
	}
}

func TestServer_IssueDetail_SpecGeneratorInTimeline(t *testing.T) {
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
	writeJSON(t, filepath.Join(issueDir, "spec-generator.json"),
		rundata.StepResult{Output: "spec gen trace", DurationSeconds: 20})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "impl trace", DurationSeconds: 45})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/5", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 500))
	}
	body := rr.Body.String()

	idxSG := strings.Index(body, "Spec Generator")
	idxImpl := strings.Index(body, "Implement")

	if idxSG < 0 {
		t.Error("body missing Spec Generator step")
	}
	if idxImpl < 0 {
		t.Error("body missing Implement step")
	}
	if idxSG >= 0 && idxImpl >= 0 && idxSG > idxImpl {
		t.Error("Spec Generator should appear before Implement in timeline")
	}
}

func TestServer_IssueDetail_DescriptionDisplayed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{30},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "30")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{
			IssueNumber: 30,
			Status:      "implemented",
			Description: "This is the issue description body text.",
		})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "done", DurationSeconds: 5})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/30", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Issue Description") {
		t.Errorf("body missing 'Issue Description' section header")
	}
	if !strings.Contains(body, "This is the issue description body text.") {
		t.Errorf("body missing description content")
	}
	// Should be rendered inside a <details> element (collapsed by default).
	if !strings.Contains(body, "<details") {
		t.Errorf("body missing <details> element for collapsible description")
	}
}

func TestServer_IssueDetail_NoDescriptionHidesSection(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{31},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "31")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	// Outcome with no Description field.
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 31, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "done", DurationSeconds: 5})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/31", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	// The Issue Description card should not appear when description is empty.
	if strings.Contains(body, `card__title">Issue Description`) {
		t.Errorf("body should not contain 'Issue Description' card when description is absent")
	}
}

func TestServer_RunDetail_GranularStatus_Pending(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	// Issue 5 has no outcome and no status file — should show "Pending".
	buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{5},
		StartedAt:    now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Pending") {
		t.Errorf("body missing 'Pending' status for issue with no outcome/status; got: %q", truncate(body, 500))
	}
	if strings.Contains(body, "Running") {
		t.Errorf("body should not contain old 'Running' status label; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_GranularStatus_Blocked(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	// Issue 2 depends on issue 1, which has no outcome — should show "Blocked".
	buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{1, 2},
		IssueDeps:    []rundata.IssueDep{{IssueNumber: 2, DependsOn: []int{1}}},
		StartedAt:    now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Blocked") {
		t.Errorf("body missing 'Blocked' status for issue with unresolved dep; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_GranularStatus_Implementing(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{3},
		StartedAt:    now,
	})

	// Write a status.json with "implementing".
	issueDir := filepath.Join(runDir, "issues", "3")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "status.json"), rundata.IssueStatus{Status: "implementing"})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Implementing") {
		t.Errorf("body missing 'Implementing' status; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_GranularStatus_InReview(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{4},
		StartedAt:    now,
	})

	// Write a status.json with "in_review".
	issueDir := filepath.Join(runDir, "issues", "4")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "status.json"), rundata.IssueStatus{Status: "in_review"})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "In Review") {
		t.Errorf("body missing 'In Review' status; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_GranularStatus_ReadyToMerge(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{5},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 5,
		rundata.Outcome{IssueNumber: 5, Status: "ready-to-merge", PRNumber: 42},
		rundata.StepResult{},
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
	if !strings.Contains(body, "Ready to Merge") {
		t.Errorf("body missing 'Ready to Merge' status; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "badge--success") {
		t.Errorf("body missing badge--success for ready-to-merge issue; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_GranularStatus_NeedsHumanReview(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{6},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 6,
		rundata.Outcome{IssueNumber: 6, Status: "needs-human-review", PRNumber: 77},
		rundata.StepResult{},
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
	if !strings.Contains(body, "Needs Human Review") {
		t.Errorf("body missing 'Needs Human Review' status; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "badge--warning") {
		t.Errorf("body missing badge--warning for needs-human-review issue; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_FinalOutcomeTakesPrecedenceOverStatus(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{7},
		StartedAt:    now,
	})

	// Write both an outcome and a live status — outcome should win.
	writeIssueFiles(t, runDir, 7,
		rundata.Outcome{IssueNumber: 7, Status: "implemented", PRNumber: 99},
		rundata.StepResult{},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	issueDir := filepath.Join(runDir, "issues", "7")
	writeJSON(t, filepath.Join(issueDir, "status.json"), rundata.IssueStatus{Status: "implementing"})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Implemented") {
		t.Errorf("body missing 'Implemented' status (outcome should win over status file); got: %q", truncate(body, 500))
	}
	if strings.Contains(body, "Implementing") {
		t.Errorf("body should not show 'Implementing' when outcome is set; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_GranularStatus_BlockedToPendingAfterDepCompletes(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	// Issue 2 depends on issue 1, which HAS an outcome — so issue 2 should be "Pending", not "Blocked".
	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{1, 2},
		IssueDeps:    []rundata.IssueDep{{IssueNumber: 2, DependsOn: []int{1}}},
		StartedAt:    now,
	})

	// Issue 1 is complete.
	writeIssueFiles(t, runDir, 1,
		rundata.Outcome{IssueNumber: 1, Status: "implemented"},
		rundata.StepResult{},
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
	if strings.Contains(body, "Blocked") {
		t.Errorf("body should not show 'Blocked' when dep is completed; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "Pending") {
		t.Errorf("body should show 'Pending' for issue whose dep has completed; got: %q", truncate(body, 500))
	}
}

func TestServer_IssueDetail_SpecGeneratorAbsentOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{6},
		StartedAt:    now,
	})

	issueDir := filepath.Join(runDir, "issues", "6")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	writeJSON(t, filepath.Join(issueDir, "outcome.json"),
		rundata.Outcome{IssueNumber: 6, Status: "implemented"})
	writeJSON(t, filepath.Join(issueDir, "implement.json"),
		rundata.StepResult{Output: "impl trace", DurationSeconds: 45})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/6", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "Spec Generator") {
		t.Error("body should not contain Spec Generator step when no spec-generator.json exists")
	}
}

func TestServer_RunDetail_BaseBranch_Shown(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		BaseBranch:   "feature/foo",
		IssueNumbers: []int{1},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 1,
		rundata.Outcome{IssueNumber: 1, Status: "implemented"},
		rundata.StepResult{Output: "ok", DurationSeconds: 5},
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
	if !strings.Contains(body, "feature/foo") {
		t.Errorf("body missing base branch; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_BaseBranch_Hidden_When_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 1,
		rundata.Outcome{IssueNumber: 1, Status: "implemented"},
		rundata.StepResult{Output: "ok", DurationSeconds: 5},
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
	if strings.Contains(body, "Base branch") {
		t.Errorf("body should not contain 'Base branch' when BaseBranch is empty; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_AutoMerge_Shown(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	am := &rundata.AutoMerge{Feature: "low_risk", Rollup: "auto"}
	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		AutoMerge:    am,
	})

	writeIssueFiles(t, runDir, 1,
		rundata.Outcome{IssueNumber: 1, Status: "implemented"},
		rundata.StepResult{Output: "ok", DurationSeconds: 5},
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
	if !strings.Contains(body, "low_risk") {
		t.Errorf("body missing auto_merge.feature value; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "rollup=auto") {
		t.Errorf("body missing auto_merge.rollup value; got: %q", truncate(body, 500))
	}
}

func TestServer_RunDetail_AutoMerge_Hidden_When_Absent(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{1},
		StartedAt:    now,
		// AutoMerge intentionally absent — simulates older run data.
	})

	writeIssueFiles(t, runDir, 1,
		rundata.Outcome{IssueNumber: 1, Status: "implemented"},
		rundata.StepResult{Output: "ok", DurationSeconds: 5},
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
	if strings.Contains(body, "Auto-merge") {
		t.Errorf("body should not contain 'Auto-merge' when AutoMerge is absent; got: %q", truncate(body, 500))
	}
}

// --- Verify check summary tests ---

func writeVerifyResult(t *testing.T, issueDir string, vr rundata.VerifyStepResult) {
	t.Helper()
	path := filepath.Join(issueDir, fmt.Sprintf("verify-%d.json", vr.Attempt))
	writeJSON(t, path, vr)
}

func TestServer_IssueDetail_VerifyCheckSummaries_AllPassed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{50},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 50)
	writeVerifyResult(t, issueDir, rundata.VerifyStepResult{
		Attempt:   0,
		AllPassed: true,
		Checks: []rundata.CheckResult{
			{Name: "build", Passed: true, ExitCode: 0},
			{Name: "lint", Passed: true, ExitCode: 0},
			{Name: "test", Passed: true, ExitCode: 0},
		},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/50", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Check names should appear in the summary.
	for _, name := range []string{"build", "lint", "test"} {
		if !strings.Contains(body, name) {
			t.Errorf("body missing check name %q; got: %q", name, truncate(body, 500))
		}
	}

	// All checks passed — verify-check--pass class should appear, not verify-check--fail.
	if !strings.Contains(body, "verify-check--pass") {
		t.Errorf("body missing verify-check--pass class for passed checks")
	}
	if strings.Contains(body, "verify-check--fail") {
		t.Errorf("body should not contain verify-check--fail when all checks pass")
	}
}

func TestServer_IssueDetail_VerifyCheckSummaries_SomeFailed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{51},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 51)
	writeVerifyResult(t, issueDir, rundata.VerifyStepResult{
		Attempt:   0,
		AllPassed: false,
		Checks: []rundata.CheckResult{
			{Name: "format", Passed: true, ExitCode: 0},
			{Name: "build", Passed: true, ExitCode: 0},
			{Name: "lint", Passed: false, ExitCode: 1, Output: "lint error output"},
			{Name: "test", Passed: false, ExitCode: 2, Output: "test failure output"},
		},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/51", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// All check names should appear in the summary.
	for _, name := range []string{"format", "build", "lint", "test"} {
		if !strings.Contains(body, name) {
			t.Errorf("body missing check name %q", name)
		}
	}

	// Both pass and fail classes should appear.
	if !strings.Contains(body, "verify-check--pass") {
		t.Errorf("body missing verify-check--pass class for passed checks")
	}
	if !strings.Contains(body, "verify-check--fail") {
		t.Errorf("body missing verify-check--fail class for failed checks")
	}

	// The overall verdict should be Failed.
	if !strings.Contains(body, "Failed") {
		t.Errorf("body missing 'Failed' verdict for verify step with failing checks")
	}

	// Expanded detail should include output for failed checks.
	if !strings.Contains(body, "lint error output") {
		t.Errorf("body missing lint error output in expanded detail")
	}
	if !strings.Contains(body, "test failure output") {
		t.Errorf("body missing test failure output in expanded detail")
	}
}

func TestServer_IssueDetail_VerifyCheckSummaries_FixAttempt(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{52},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 52)

	// Initial verify: failed.
	writeVerifyResult(t, issueDir, rundata.VerifyStepResult{
		Attempt:   0,
		AllPassed: false,
		Checks: []rundata.CheckResult{
			{Name: "build", Passed: false, ExitCode: 1, Output: "compile error"},
		},
	})
	// Fix attempt: passed.
	writeVerifyResult(t, issueDir, rundata.VerifyStepResult{
		Attempt:      1,
		AllPassed:    true,
		FixAttempted: true,
		Checks: []rundata.CheckResult{
			{Name: "build", Passed: true, ExitCode: 0},
		},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/52", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Both verify steps should be visible.
	if !strings.Contains(body, "Verify (fix attempt 1)") {
		t.Errorf("body missing fix-attempt label; got: %q", truncate(body, 500))
	}

	// The initial failed step should show the build check as failed.
	if !strings.Contains(body, "verify-check--fail") {
		t.Errorf("body missing verify-check--fail for initial failing verify step")
	}
}

func TestServer_IssueDetail_VerifyCheckSummaries_NoChecks(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{53},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 53)
	writeVerifyResult(t, issueDir, rundata.VerifyStepResult{
		Attempt:   0,
		AllPassed: true,
		Checks:    nil,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/53", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// No check summary pills when no checks are recorded.
	if strings.Contains(body, "verify-check") {
		t.Errorf("body should not contain verify-check class when no checks are recorded")
	}
	// Verdict should still be Passed.
	if !strings.Contains(body, "Passed") {
		t.Errorf("body missing 'Passed' verdict")
	}
}

// --- Resource stats tests ---

func TestServer_IssueDetail_ResourceStats_WithData(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{60},
		StartedAt:    now,
	})

	issueDir := buildIssueDir(t, runDir, 60)
	// 104857600 bytes = 100.0 MB; 2500000000 ns = 2.50s
	writeJSON(t, filepath.Join(issueDir, "implement.json"), rundata.StepResult{
		Output:          "done",
		DurationSeconds: 30,
		CostUSD:         0.001,
		PeakMemoryBytes: 104857600,
		CPUNanoseconds:  2500000000,
	})
	writeJSON(t, filepath.Join(issueDir, "outcome.json"), rundata.Outcome{
		IssueNumber: 60,
		Status:      "implemented",
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/60", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	if !strings.Contains(body, "100.0 MB") {
		t.Errorf("body missing formatted peak memory '100.0 MB'; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "2.50s") {
		t.Errorf("body missing formatted CPU time '2.50s'; got: %q", truncate(body, 500))
	}
}

func TestServer_IssueDetail_ResourceStats_NoData(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{61},
		StartedAt:    now,
	})

	issueDir := buildIssueDir(t, runDir, 61)
	// Older run: resource fields absent (zero values).
	writeJSON(t, filepath.Join(issueDir, "implement.json"), rundata.StepResult{
		Output:          "done",
		DurationSeconds: 15,
		CostUSD:         0.0005,
		// PeakMemoryBytes and CPUNanoseconds intentionally zero (pre-feature run).
	})
	writeJSON(t, filepath.Join(issueDir, "outcome.json"), rundata.Outcome{
		IssueNumber: 61,
		Status:      "implemented",
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/61", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	// Both resource columns should show the placeholder "—" for zero values.
	// We check that "—" appears (it will appear at least for the two resource columns).
	count := strings.Count(body, "—")
	if count < 2 {
		t.Errorf("expected at least 2 '—' placeholders for missing resource data, got %d; body: %q", count, truncate(body, 500))
	}
	// Ensure no MB or CPU seconds values are rendered for zero data.
	if strings.Contains(body, " MB") {
		t.Errorf("body should not contain ' MB' when PeakMemoryBytes is zero; got: %q", truncate(body, 500))
	}
}

func TestServer_IssueDetail_ResourceStats_MixedSteps(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		IssueNumbers: []int{62},
		StartedAt:    now,
	})

	issueDir := buildIssueDir(t, runDir, 62)
	// Implement step has resource data.
	writeJSON(t, filepath.Join(issueDir, "implement.json"), rundata.StepResult{
		Output:          "done",
		DurationSeconds: 20,
		CostUSD:         0.002,
		PeakMemoryBytes: 52428800, // 50.0 MB
		CPUNanoseconds:  1000000000, // 1.00s
	})
	// Quality review step has no resource data.
	writeJSON(t, filepath.Join(issueDir, "quality-review.json"), rundata.StepResult{
		Output:          "AGENT_RESULT=APPROVED",
		DurationSeconds: 10,
		CostUSD:         0.001,
		// No resource fields.
	})
	writeJSON(t, filepath.Join(issueDir, "outcome.json"), rundata.Outcome{
		IssueNumber: 62,
		Status:      "implemented",
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/62", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	// Implement step should show resource values.
	if !strings.Contains(body, "50.0 MB") {
		t.Errorf("body missing '50.0 MB' for implement step; got: %q", truncate(body, 500))
	}
	if !strings.Contains(body, "1.00s") {
		t.Errorf("body missing '1.00s' for implement step; got: %q", truncate(body, 500))
	}
	// Quality review step (no resource data) should contribute "—" placeholders.
	if !strings.Contains(body, "—") {
		t.Errorf("body missing '—' placeholder for quality review step with no resource data")
	}
}
