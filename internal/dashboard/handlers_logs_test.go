package dashboard_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

// writeDebugLog writes a debug.log file with the given JSON lines into the run directory.
func writeDebugLog(t *testing.T, runDir string, lines []string) {
	t.Helper()
	content := strings.Join(lines, "\n") + "\n"
	path := filepath.Join(runDir, "debug.log")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing debug.log: %v", err)
	}
}

// makeLogLine returns a slog-style JSON log line.
func makeLogLine(level, msg string, extraFields ...string) string {
	ts := "2026-03-05T10:00:00.000000000Z"
	extra := ""
	for _, f := range extraFields {
		extra += "," + f
	}
	return fmt.Sprintf(`{"time":%q,"level":%q,"msg":%q%s}`, ts, level, msg, extra)
}

// --- Log viewer page tests ---

func TestServer_RunLogs_FullPage(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	writeDebugLog(t, runDir, []string{
		makeLogLine("INFO", "starting run"),
		makeLogLine("DEBUG", "processing issue", `"issue_number":5`),
		makeLogLine("ERROR", "something failed"),
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	// All entries rendered
	if !strings.Contains(body, "starting run") {
		t.Error("body missing INFO entry")
	}
	if !strings.Contains(body, "processing issue") {
		t.Error("body missing DEBUG entry")
	}
	if !strings.Contains(body, "something failed") {
		t.Error("body missing ERROR entry")
	}

	// Columns present
	for _, col := range []string{"Time", "Level", "Message", "Fields"} {
		if !strings.Contains(body, col) {
			t.Errorf("body missing column header %q", col)
		}
	}
}

func TestServer_RunLogs_Breadcrumbs(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	buildRunDir(t, tmpDir, "myorg", "myproj", ts, rundata.RunMeta{
		Repo:      "myorg/myproj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/myorg/myproj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Breadcrumb: Runs / run / Logs
	if !strings.Contains(body, `href="/"`) {
		t.Error("body missing Runs breadcrumb link")
	}
	runDetailURL := "/runs/myorg/myproj/" + ts
	if !strings.Contains(body, runDetailURL) {
		t.Errorf("body missing run detail breadcrumb link %q", runDetailURL)
	}
	if !strings.Contains(body, "breadcrumbs__current") {
		t.Error("body missing breadcrumbs__current for Logs")
	}
	if !strings.Contains(body, ">Logs<") {
		t.Error("body missing Logs as current breadcrumb")
	}
}

func TestServer_RunLogs_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/runs/acme/nope/20240101-000000/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestServer_RunLogs_EmptyLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	// Run exists but no debug.log
	buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "No log entries") {
		t.Error("body should show empty state when no log file")
	}
}

// --- Log parsing tests ---

func TestServer_RunLogs_LogParsing_100Lines(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	// Write exactly 100 JSON log lines.
	lines := make([]string, 100)
	for i := 0; i < 100; i++ {
		lines[i] = makeLogLine("INFO", fmt.Sprintf("message %d", i))
	}
	writeDebugLog(t, runDir, lines)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Newest entries should appear first (reverse chronological).
	if !strings.Contains(body, "message 99") {
		t.Error("body missing newest log entry")
	}

	// With 100 entries and page size 50, a "load more" button is present.
	if !strings.Contains(body, "Load more") {
		t.Error("should show load more when 100 entries exceed first page")
	}
}

func TestServer_RunLogs_LogParsing_Columns(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	writeDebugLog(t, runDir, []string{
		makeLogLine("WARN", "some warning", `"issue_number":42,"repo":"acme/proj"`),
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Level badge
	if !strings.Contains(body, "WARN") {
		t.Error("body missing WARN level")
	}
	// Message
	if !strings.Contains(body, "some warning") {
		t.Error("body missing message")
	}
	// Structured fields
	if !strings.Contains(body, "issue_number") {
		t.Error("body missing structured field key issue_number")
	}
}

// --- Level filter tests ---

func TestServer_RunLogs_LevelFilter(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	writeDebugLog(t, runDir, []string{
		makeLogLine("INFO", "info message"),
		makeLogLine("ERROR", "error message"),
		makeLogLine("DEBUG", "debug message"),
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs?level=ERROR", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "error message") {
		t.Error("body missing ERROR entry after filtering")
	}
	if strings.Contains(body, "info message") {
		t.Error("body should not contain INFO entry when filtering by ERROR")
	}
	if strings.Contains(body, "debug message") {
		t.Error("body should not contain DEBUG entry when filtering by ERROR")
	}
}

func TestServer_RunLogs_LevelFilter_Partial(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	writeDebugLog(t, runDir, []string{
		makeLogLine("INFO", "info only"),
		makeLogLine("ERROR", "error only"),
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs/entries?level=ERROR", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "error only") {
		t.Error("partial missing ERROR entry")
	}
	if strings.Contains(body, "info only") {
		t.Error("partial should not contain INFO entry when filtering by ERROR")
	}
}

// --- Search tests ---

func TestServer_RunLogs_Search(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	writeDebugLog(t, runDir, []string{
		makeLogLine("INFO", "processing issue_number 5"),
		makeLogLine("INFO", "unrelated message"),
		makeLogLine("DEBUG", "fetching data", `"issue_number":7`),
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs?search=issue_number", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Matches both the message and structured field
	if !strings.Contains(body, "processing issue_number 5") {
		t.Error("body missing entry matching issue_number in message")
	}
	if !strings.Contains(body, "fetching data") {
		t.Error("body missing entry matching issue_number in fields")
	}
	if strings.Contains(body, "unrelated message") {
		t.Error("body should not contain unrelated entry")
	}
}

func TestServer_RunLogs_Search_Partial(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	writeDebugLog(t, runDir, []string{
		makeLogLine("INFO", "match this"),
		makeLogLine("INFO", "no match here"),
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs/entries?search=match+this", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "match this") {
		t.Error("partial missing matching entry")
	}
	if strings.Contains(body, "no match here") {
		t.Error("partial should not contain non-matching entry")
	}
}

// --- Pagination tests ---

func TestServer_RunLogs_Pagination_LoadMore(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	// Write 100 lines (two pages of 50).
	lines := make([]string, 100)
	for i := 0; i < 100; i++ {
		lines[i] = makeLogLine("INFO", fmt.Sprintf("entry %d", i))
	}
	writeDebugLog(t, runDir, lines)

	srv := newServer(t, tmpDir)

	// First page: newest entries (99-50) in reverse chronological order.
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("first page status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "entry 99") {
		t.Error("first page missing entry 99 (newest)")
	}
	if !strings.Contains(body, "entry 50") {
		t.Error("first page missing entry 50")
	}
	if strings.Contains(body, "entry 49") {
		t.Error("first page should not contain entry 49 (second page)")
	}
	if !strings.Contains(body, "Load more") {
		t.Error("first page missing load more button")
	}

	// Second page (htmx partial): older entries (49-0) and no load more.
	req2 := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs/entries?page=2", nil)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("second page status = %d, want 200", rr2.Code)
	}
	body2 := rr2.Body.String()

	if !strings.Contains(body2, "entry 49") {
		t.Error("second page missing entry 49")
	}
	if !strings.Contains(body2, "entry 0") {
		t.Error("second page missing entry 0 (oldest)")
	}
	if strings.Contains(body2, "Load more") {
		t.Error("second page should not show load more (no more entries)")
	}
}

func TestServer_RunLogs_Pagination_HtmxTarget(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	// 101 lines so there's exactly one extra entry on page 2.
	lines := make([]string, 101)
	for i := 0; i < 101; i++ {
		lines[i] = makeLogLine("INFO", fmt.Sprintf("entry %d", i))
	}
	writeDebugLog(t, runDir, lines)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Load more button must target log-load-more with outerHTML.
	if !strings.Contains(body, "log-load-more") {
		t.Error("body missing log-load-more element ID")
	}
	if !strings.Contains(body, "hx-swap") {
		t.Error("body missing hx-swap attribute on load more button")
	}
	if !strings.Contains(body, "page=2") {
		t.Error("body missing page=2 in load more URL")
	}
}

// --- HTMX filter integration tests ---

func TestServer_RunLogs_FilterControls_HTMX(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:      "acme/proj",
		Milestone: "v1.0",
		StartedAt: now,
	})

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/logs", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// Level select must trigger htmx.
	if !strings.Contains(body, "hx-get") {
		t.Error("body missing hx-get on filter control")
	}
	if !strings.Contains(body, "log-entries-tbody") {
		t.Error("body missing log-entries-tbody target")
	}
	// Search input present.
	if !strings.Contains(body, `name="search"`) {
		t.Error("body missing search input")
	}
	if !strings.Contains(body, `name="level"`) {
		t.Error("body missing level select")
	}
}
