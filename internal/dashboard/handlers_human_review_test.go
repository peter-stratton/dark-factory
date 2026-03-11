package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// TestServer_RunDetail_AwaitingReviewSection verifies that a run with
// ready-to-merge outcomes renders the "PRs Awaiting Review" section.
func TestServer_RunDetail_AwaitingReviewSection(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v2.0",
		IssueNumbers: []int{10, 11},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 10,
		rundata.Outcome{IssueNumber: 10, Status: "ready-to-merge", PRNumber: 42},
		rundata.StepResult{Output: "done", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	writeIssueFiles(t, runDir, 11,
		rundata.Outcome{IssueNumber: 11, Status: "implemented"},
		rundata.StepResult{Output: "done", DurationSeconds: 5},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "PRs Awaiting Review") {
		t.Errorf("body missing 'PRs Awaiting Review' section; got: %q", truncate(body, 600))
	}
	if !strings.Contains(body, "awaiting") {
		t.Errorf("body missing awaiting badge; got: %q", truncate(body, 600))
	}
}

// TestServer_RunDetail_NoAwaitingSection verifies that the "PRs Awaiting Review"
// section is hidden when no issues have ready-to-merge status.
func TestServer_RunDetail_NoAwaitingSection(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "noreview", ts, rundata.RunMeta{
		Repo:         "acme/noreview",
		Milestone:    "v1.0",
		IssueNumbers: []int{5, 6},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 5,
		rundata.Outcome{IssueNumber: 5, Status: "implemented"},
		rundata.StepResult{Output: "done", DurationSeconds: 8},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	writeIssueFiles(t, runDir, 6,
		rundata.Outcome{IssueNumber: 6, Status: "implemented"},
		rundata.StepResult{Output: "done", DurationSeconds: 12},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/noreview/"+ts, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "PRs Awaiting Review") {
		t.Errorf("body should not contain 'PRs Awaiting Review' when all issues are implemented")
	}
}

// TestServer_RunDetail_AwaitingFilter verifies that the issues table HTMX
// partial filters by awaiting_human state.
func TestServer_RunDetail_AwaitingFilter(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "filter", ts, rundata.RunMeta{
		Repo:         "acme/filter",
		Milestone:    "v3.0",
		IssueNumbers: []int{1, 2},
		StartedAt:    now,
	})

	writeIssueFiles(t, runDir, 1,
		rundata.Outcome{IssueNumber: 1, Status: "ready-to-merge", PRNumber: 99, Title: "Awaiting issue"},
		rundata.StepResult{Output: "done", DurationSeconds: 5},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	writeIssueFiles(t, runDir, 2,
		rundata.Outcome{IssueNumber: 2, Status: "implemented", Title: "Implemented issue"},
		rundata.StepResult{Output: "done", DurationSeconds: 5},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)

	// Without filter: both issues visible in partial.
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/filter/"+ts+"/issues-table", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "#1") {
		t.Errorf("unfiltered: body missing issue #1")
	}
	if !strings.Contains(body, "#2") {
		t.Errorf("unfiltered: body missing issue #2")
	}

	// With filter=awaiting_human: only ready-to-merge issue visible.
	req2 := httptest.NewRequest(http.MethodGet, "/runs/acme/filter/"+ts+"/issues-table?filter=awaiting_human", nil)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("filtered status = %d, want 200", rr2.Code)
	}
	body2 := rr2.Body.String()
	if !strings.Contains(body2, "#1") {
		t.Errorf("filtered: body missing awaiting issue #1; got: %q", truncate(body2, 400))
	}
	if strings.Contains(body2, "#2") {
		t.Errorf("filtered: body should not contain implemented issue #2; got: %q", truncate(body2, 400))
	}
}

// TestServer_IssueDetail_HumanDialogueDistinctStyle verifies that dialogue
// entries with role "human" display with distinct visual styling.
func TestServer_IssueDetail_HumanDialogueDistinctStyle(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "owner", "repo", ts, rundata.RunMeta{
		Repo:         "owner/repo",
		Milestone:    "v1.0",
		IssueNumbers: []int{30},
		StartedAt:    now,
	})
	issueDir := buildIssueDir(t, runDir, 30)
	writeDialogueFile(t, issueDir, []rundata.DialogueEntry{
		{Role: "implementer", Round: 1, Body: "agent implementation notes"},
		{Role: "human", Round: 1, Body: "human reviewer feedback here"},
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/owner/repo/"+ts+"/issues/30", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	// The human entry should render with the purple/indigo color (#a78bfa) and person icon.
	if !strings.Contains(body, "#a78bfa") {
		t.Errorf("body missing purple color for human dialogue entry; got: %q", truncate(body, 800))
	}
	// Person icon (👤 = &#128100;) should appear for human role.
	if !strings.Contains(body, "&#128100;") {
		t.Errorf("body missing person icon for human dialogue entry")
	}
	// "Human Reviewer" label should appear.
	if !strings.Contains(body, "Human Reviewer") {
		t.Errorf("body missing 'Human Reviewer' label")
	}
	// Implementer entry should still render with different (non-human) styling.
	if !strings.Contains(body, "Implementer") {
		t.Errorf("body missing Implementer dialogue entry")
	}
}

// TestServer_Index_AwaitingCountBadge verifies that the run list shows
// an awaiting count badge when runs have ready-to-merge outcomes.
func TestServer_Index_AwaitingCountBadge(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	finishedAt := now
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "corp", "service", ts, rundata.RunMeta{
		Repo:         "corp/service",
		Milestone:    "sprint-1",
		IssueNumbers: []int{100, 101},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 2, Implemented: 0, Failed: 0},
	})

	writeIssueFiles(t, runDir, 100,
		rundata.Outcome{IssueNumber: 100, Status: "ready-to-merge", PRNumber: 5},
		rundata.StepResult{Output: "done", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)
	writeIssueFiles(t, runDir, 101,
		rundata.Outcome{IssueNumber: 101, Status: "ready-to-merge", PRNumber: 6},
		rundata.StepResult{Output: "done", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "2 awaiting") {
		t.Errorf("body missing '2 awaiting' badge for run with 2 ready-to-merge issues; got: %q", truncate(body, 800))
	}
}

// TestServer_Index_NoAwaitingBadge verifies that no awaiting badge is shown
// when all issues in a run are fully implemented.
func TestServer_Index_NoAwaitingBadge(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	finishedAt := now
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "corp", "finished", ts, rundata.RunMeta{
		Repo:         "corp/finished",
		Milestone:    "done",
		IssueNumbers: []int{200},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 1, Implemented: 1, Failed: 0},
	})

	writeIssueFiles(t, runDir, 200,
		rundata.Outcome{IssueNumber: 200, Status: "implemented"},
		rundata.StepResult{Output: "done", DurationSeconds: 5},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "1 awaiting") || strings.Contains(body, "2 awaiting") {
		t.Errorf("body should not show awaiting badge when no issues are ready-to-merge; got: %q", truncate(body, 600))
	}
}
