package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/rundata"
)

// TestServer_Index_ExcludesTestRuns verifies that runs matching the test-run
// criteria (repo = "owner/repo" or milestone = "test-milestone") are excluded
// from the dashboard index page.
func TestServer_Index_ExcludesTestRuns(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	// Create a test run matching by repo.
	ts1 := now.Add(-2 * time.Hour).Format("20060102-150405")
	writeRunMeta(t, tmpDir, "owner", "repo", ts1, rundata.RunMeta{
		Repo:         "owner/repo",
		Milestone:    "Phase 21",
		IssueNumbers: []int{1},
		StartedAt:    now.Add(-2 * time.Hour),
	})

	// Create a test run matching by milestone.
	ts2 := now.Add(-1 * time.Hour).Format("20060102-150405")
	writeRunMeta(t, tmpDir, "org", "other", ts2, rundata.RunMeta{
		Repo:         "org/other",
		Milestone:    "test-milestone",
		IssueNumbers: []int{2},
		StartedAt:    now.Add(-1 * time.Hour),
	})

	// Create a real run that should be visible.
	ts3 := now.Format("20060102-150405")
	writeRunMeta(t, tmpDir, "org", "real", ts3, rundata.RunMeta{
		Repo:         "org/real",
		Milestone:    "Phase 21",
		IssueNumbers: []int{3},
		StartedAt:    now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// The real run should appear.
	if !strings.Contains(body, "org/real") {
		t.Error("expected org/real to appear in dashboard")
	}

	// Test runs should be excluded.
	if strings.Contains(body, ">owner/repo<") {
		t.Error("expected owner/repo (test repo) to be excluded from dashboard")
	}
	// The test-milestone run's repo "org/other" should not appear in the repo filter dropdown.
	if strings.Contains(body, "org/other") {
		t.Error("expected org/other (test-milestone run) to be excluded from dashboard")
	}
}

// TestServer_Index_ExcludesTestRunsFromRepoDropdown verifies that repos only
// present via test runs do not appear in the repo filter dropdown.
func TestServer_Index_ExcludesTestRunsFromRepoDropdown(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	// A test run and a real run.
	ts1 := now.Add(-1 * time.Hour).Format("20060102-150405")
	writeRunMeta(t, tmpDir, "owner", "repo", ts1, rundata.RunMeta{
		Repo:         "owner/repo",
		Milestone:    "Phase 21",
		IssueNumbers: []int{1},
		StartedAt:    now.Add(-1 * time.Hour),
	})

	ts2 := now.Format("20060102-150405")
	writeRunMeta(t, tmpDir, "org", "real", ts2, rundata.RunMeta{
		Repo:         "org/real",
		Milestone:    "Phase 21",
		IssueNumbers: []int{2},
		StartedAt:    now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// "owner/repo" should not appear as a repo option in the dropdown.
	// The dropdown uses <option value="owner/repo"> format.
	if strings.Contains(body, `value="owner/repo"`) {
		t.Error("expected owner/repo to be excluded from repo dropdown")
	}
	// The real run should appear.
	if !strings.Contains(body, "org/real") {
		t.Error("expected org/real to appear in dashboard")
	}
}
