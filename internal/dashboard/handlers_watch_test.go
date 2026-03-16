package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/dashboard"
	"github.com/phs/dark-factory/internal/rundata"
	"log/slog"
)

// stubWatchQuerier is a test double for dashboard.WatchQuerier.
type stubWatchQuerier struct {
	prs []dashboard.WatchedPRInfo
	err error
}

func (s *stubWatchQuerier) ListWatchedPRs(_ context.Context, repo, label string) ([]dashboard.WatchedPRInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.prs, nil
}

// newWatchServer creates a dashboard Server with the given WatchQuerier.
func newWatchServer(t *testing.T, tmpDir string, q dashboard.WatchQuerier) *dashboard.Server {
	t.Helper()
	t.Setenv("HOME", tmpDir)
	reader, err := rundata.NewReader(slog.Default())
	if err != nil {
		t.Fatalf("creating reader: %v", err)
	}
	srv, err := dashboard.New(dashboard.Config{
		Port:         8374,
		Logger:       slog.Default(),
		WatchQuerier: q,
	}, reader, nil)
	if err != nil {
		t.Fatalf("creating server: %v", err)
	}
	return srv
}

// TestWatch_NoQuerier verifies the /watch page shows a disabled state when
// no WatchQuerier is configured.
func TestWatch_NoQuerier(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir) // uses newServer which sets WatchQuerier to nil

	req := httptest.NewRequest(http.MethodGet, "/watch", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Watch not configured") {
		t.Errorf("body missing 'Watch not configured'; got: %q", truncate(body, 600))
	}
}

// TestWatch_NoRepo verifies the /watch page shows a repo selection prompt
// when a querier is configured but no ?repo= param is provided.
func TestWatch_NoRepo(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newWatchServer(t, tmpDir, &stubWatchQuerier{})

	req := httptest.NewRequest(http.MethodGet, "/watch", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Select a repository") {
		t.Errorf("body missing 'Select a repository'; got: %q", truncate(body, 600))
	}
}

// TestWatch_PRsDisplayed verifies two PRs with different labels are shown
// with correct badges.
func TestWatch_PRsDisplayed(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()

	q := &stubWatchQuerier{
		prs: []dashboard.WatchedPRInfo{
			{
				Number:    42,
				Title:     "Implement feature X",
				Labels:    []string{"godark:awaiting-human-review"},
				UpdatedAt: now.Add(-2 * time.Hour),
			},
			{
				Number:    99,
				Title:     "Fix review feedback",
				Labels:    []string{"godark:fixing-review-feedback"},
				UpdatedAt: now.Add(-30 * time.Minute),
			},
		},
	}

	srv := newWatchServer(t, tmpDir, q)
	req := httptest.NewRequest(http.MethodGet, "/watch?repo=owner/repo", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", rr.Code, truncate(rr.Body.String(), 300))
	}
	body := rr.Body.String()

	// Both PRs should appear.
	if !strings.Contains(body, "#42") {
		t.Errorf("body missing PR #42; got: %q", truncate(body, 800))
	}
	if !strings.Contains(body, "Implement feature X") {
		t.Errorf("body missing PR #42 title; got: %q", truncate(body, 800))
	}
	if !strings.Contains(body, "#99") {
		t.Errorf("body missing PR #99; got: %q", truncate(body, 800))
	}
	if !strings.Contains(body, "Fix review feedback") {
		t.Errorf("body missing PR #99 title; got: %q", truncate(body, 800))
	}

	// Correct badge labels.
	if !strings.Contains(body, "Awaiting Review") {
		t.Errorf("body missing 'Awaiting Review' badge; got: %q", truncate(body, 800))
	}
	if !strings.Contains(body, "Fixing Feedback") {
		t.Errorf("body missing 'Fixing Feedback' badge; got: %q", truncate(body, 800))
	}

	// GitHub links point to the correct URLs.
	if !strings.Contains(body, "https://github.com/owner/repo/pull/42") {
		t.Errorf("body missing GitHub link for PR #42; got: %q", truncate(body, 800))
	}
	if !strings.Contains(body, "https://github.com/owner/repo/pull/99") {
		t.Errorf("body missing GitHub link for PR #99; got: %q", truncate(body, 800))
	}
}

// TestWatch_NoPRs verifies the empty state message is shown when no labeled
// PRs exist.
func TestWatch_NoPRs(t *testing.T) {
	tmpDir := t.TempDir()
	q := &stubWatchQuerier{prs: nil}

	srv := newWatchServer(t, tmpDir, q)
	req := httptest.NewRequest(http.MethodGet, "/watch?repo=owner/empty", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "No watched PRs") {
		t.Errorf("body missing 'No watched PRs'; got: %q", truncate(body, 600))
	}
}

// TestWatch_NavLink verifies the Watch navigation link appears on the index page.
func TestWatch_NavLink(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newServer(t, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `href="/watch"`) {
		t.Errorf("index page missing Watch nav link; got: %q", truncate(body, 800))
	}
}
