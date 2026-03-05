package dashboard_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/dashboard"
	"github.com/phs/dark-factory/internal/rundata"
)

// writeRunMeta creates a run directory and run.json under baseDir.
func writeRunMeta(t *testing.T, baseDir, owner, repo, timestamp string, meta rundata.RunMeta) {
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
}

// newServer creates a dashboard Server pointed at tmpDir as its HOME directory.
func newServer(t *testing.T, tmpDir string) *dashboard.Server {
	t.Helper()
	t.Setenv("HOME", tmpDir)
	reader, err := rundata.NewReader(slog.Default())
	if err != nil {
		t.Fatalf("creating reader: %v", err)
	}
	srv, err := dashboard.New(dashboard.Config{Port: 8374, Logger: slog.Default()}, reader)
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}
	return srv
}

func TestServer_Index_EmptyState(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Welcome to godark") {
		t.Errorf("body missing welcome message; got: %q", truncate(body, 300))
	}
	if !strings.Contains(body, "godark run") {
		t.Errorf("body missing example command")
	}
}

func TestServer_Index_WithRuns(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now().UTC()
	finishedAt := now
	ts := now.Format("20060102-150405")

	writeRunMeta(t, tmpDir, "phs", "dark-factory", ts, rundata.RunMeta{
		Repo:         "phs/dark-factory",
		Milestone:    "v1.0",
		IssueNumbers: []int{1, 2, 3},
		StartedAt:    now,
		FinishedAt:   &finishedAt,
		Summary:      &rundata.RunSummary{Total: 3, Implemented: 3, Failed: 0},
	})

	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "phs/dark-factory") {
		t.Errorf("body missing repo name; got: %q", truncate(body, 300))
	}
	if !strings.Contains(body, "v1.0") {
		t.Errorf("body missing milestone")
	}
	if !strings.Contains(body, "Passed") {
		t.Errorf("body missing status badge")
	}
}

func TestServer_Index_ReturnsHTML(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want to contain text/html", ct)
	}
}

func TestServer_RunsSortedMostRecentFirst(t *testing.T) {
	tmpDir := t.TempDir()

	older := time.Now().UTC().Add(-2 * time.Hour)
	newer := time.Now().UTC()
	// Use different seconds to avoid timestamp collision
	olderTS := older.Format("20060102-150405")
	newerTS := newer.Format("20060102-150405")

	writeRunMeta(t, tmpDir, "phs", "repo", olderTS, rundata.RunMeta{
		Repo:      "phs/repo",
		Milestone: "v1.0-older",
		StartedAt: older,
	})
	writeRunMeta(t, tmpDir, "phs", "repo", newerTS, rundata.RunMeta{
		Repo:      "phs/repo",
		Milestone: "v2.0-newer",
		StartedAt: newer,
	})

	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	body := rr.Body.String()
	idxNewer := strings.Index(body, "v2.0-newer")
	idxOlder := strings.Index(body, "v1.0-older")

	if idxNewer < 0 || idxOlder < 0 {
		t.Fatalf("milestones not found in output; got: %q", truncate(body, 500))
	}
	if idxNewer > idxOlder {
		t.Errorf("newer run (v2.0) should appear before older run (v1.0) in output")
	}
}

func TestServer_StaticAssets(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	tests := []struct {
		path            string
		wantContentType string
	}{
		{"/static/style.css", "text/css"},
		{"/static/htmx.min.js", "javascript"},
		{"/static/alpine.min.js", "javascript"},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", tc.path, rr.Code)
			continue
		}
		ct := rr.Header().Get("Content-Type")
		if !strings.Contains(ct, tc.wantContentType) {
			t.Errorf("GET %s: Content-Type = %q, want to contain %q", tc.path, ct, tc.wantContentType)
		}
		if rr.Body.Len() == 0 {
			t.Errorf("GET %s: empty response body", tc.path)
		}
	}
}

func TestServer_FilterByRepo_Partial(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now().UTC()
	tsA := now.Format("20060102-150405")
	tsB := now.Add(-time.Second).Format("20060102-150405")

	writeRunMeta(t, tmpDir, "phs", "repo-a", tsA, rundata.RunMeta{
		Repo:      "phs/repo-a",
		Milestone: "milestone-alpha",
		StartedAt: now,
	})
	writeRunMeta(t, tmpDir, "phs", "repo-b", tsB, rundata.RunMeta{
		Repo:      "phs/repo-b",
		Milestone: "milestone-beta",
		StartedAt: now.Add(-time.Second),
	})

	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/partials/runs-table?repo=phs%2Frepo-a", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "milestone-alpha") {
		t.Errorf("expected milestone-alpha in filtered result; got: %q", truncate(body, 300))
	}
	if strings.Contains(body, "milestone-beta") {
		t.Errorf("unexpected milestone-beta in result filtered to repo-a")
	}
}

func TestServer_FilterByRepo_Index(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now().UTC()
	ts := now.Format("20060102-150405")
	writeRunMeta(t, tmpDir, "phs", "myrepo", ts, rundata.RunMeta{
		Repo:      "phs/myrepo",
		Milestone: "v3.0",
		StartedAt: now,
	})

	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/?repo=phs%2Fmyrepo", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	// The selected repo filter should be reflected in the HTML
	if !strings.Contains(body, "phs/myrepo") {
		t.Errorf("body missing repo name; got: %q", truncate(body, 300))
	}
}

func TestServer_MultipleRuns_AllShown(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		ts := now.Add(time.Duration(-i) * time.Minute).Format("20060102-150405")
		// Avoid duplicate timestamps by appending a counter if needed
		if i > 0 {
			ts = now.Add(time.Duration(-i) * time.Minute).Format("20060102-150405")
		}
		writeRunMeta(t, tmpDir, "acme", "project", ts+"-"+string(rune('0'+i)), rundata.RunMeta{
			Repo:      "acme/project",
			Milestone: "sprint-" + string(rune('A'+i)),
			StartedAt: now.Add(time.Duration(-i) * time.Minute),
		})
	}

	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	body := rr.Body.String()
	for _, ms := range []string{"sprint-A", "sprint-B", "sprint-C"} {
		if !strings.Contains(body, ms) {
			t.Errorf("body missing %q", ms)
		}
	}
}

// truncate returns at most n bytes of s for use in error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
