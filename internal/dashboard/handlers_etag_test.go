package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/rundata"
)

func TestPartial_ETag_304_WhenUnchanged(t *testing.T) {
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
		rundata.StepResult{Output: "done", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)

	// First request — should get 200 with ETag.
	req1 := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/1/review-chain", nil)
	rr1 := httptest.NewRecorder()
	srv.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", rr1.Code)
	}
	etag := rr1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first request: missing ETag header")
	}
	if rr1.Body.Len() == 0 {
		t.Fatal("first request: empty body")
	}

	// Second request with If-None-Match — should get 304.
	req2 := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/1/review-chain", nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotModified {
		t.Errorf("second request: status = %d, want 304", rr2.Code)
	}
	if rr2.Body.Len() != 0 {
		t.Errorf("second request: expected empty body, got %d bytes", rr2.Body.Len())
	}
}

func TestPartial_ETag_200_WhenChanged(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	runDir := buildRunDir(t, tmpDir, "acme", "proj", ts, rundata.RunMeta{
		Repo:         "acme/proj",
		Milestone:    "v1.0",
		IssueNumbers: []int{2},
		StartedAt:    now,
	})
	writeIssueFiles(t, runDir, 2,
		rundata.Outcome{IssueNumber: 2, Status: "implemented"},
		rundata.StepResult{Output: "version-1", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)

	// Stale ETag should not match — should get 200.
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/2/review-chain", nil)
	req.Header.Set("If-None-Match", `"stale-etag"`)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for stale etag", rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Error("expected non-empty body for stale etag")
	}
}

func TestPartial_RunsTable_ETag(t *testing.T) {
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

	req1 := httptest.NewRequest(http.MethodGet, "/partials/runs-table", nil)
	rr1 := httptest.NewRecorder()
	srv.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr1.Code)
	}
	etag := rr1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag header on runs-table partial")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/partials/runs-table", nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304", rr2.Code)
	}
}

func TestIssueDetail_MorphSwapAttribute(t *testing.T) {
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
		rundata.StepResult{Output: "impl", DurationSeconds: 10},
		rundata.StepResult{},
		rundata.StepResult{},
	)

	srv := newServer(t, tmpDir)
	req := httptest.NewRequest(http.MethodGet, "/runs/acme/proj/"+ts+"/issues/5", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, `hx-swap="morph:innerHTML"`) {
		t.Error("body missing morph:innerHTML swap attribute on review chain")
	}
	if !strings.Contains(body, `hx-ext="morph"`) {
		t.Error("body missing hx-ext morph attribute")
	}
	if !strings.Contains(body, "idiomorph-ext.min.js") {
		t.Error("body missing idiomorph extension script tag")
	}
}
