package rundata

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindSessionID_NoRunsDir(t *testing.T) {
	base := t.TempDir()
	sid, err := FindSessionID(filepath.Join(base, "nonexistent"), "owner/repo", 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "" {
		t.Errorf("expected empty session ID, got %q", sid)
	}
}

func TestFindSessionID_InvalidRepo(t *testing.T) {
	base := t.TempDir()
	_, err := FindSessionID(base, "noslash", 42, nil)
	if err == nil {
		t.Error("expected error for invalid repo format, got nil")
	}
}

func TestFindSessionID_NoPRMatch(t *testing.T) {
	base := t.TempDir()
	now := time.Now().UTC()
	runDir := makeRunDir(t, base, "owner", "repo", "20260101-100000",
		RunMeta{Repo: "owner/repo", StartedAt: now})

	issueDir := filepath.Join(runDir, "issues", "1")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "outcome.json"), Outcome{
		IssueNumber: 1,
		PRNumber:    99, // different PR
	}); err != nil {
		t.Fatalf("writing outcome: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "implement.json"), StepResult{
		SessionID: "sess-xxx",
	}); err != nil {
		t.Fatalf("writing implement: %v", err)
	}

	sid, err := FindSessionID(base, "owner/repo", 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "" {
		t.Errorf("expected empty session ID, got %q", sid)
	}
}

func TestFindSessionID_FromImplementStep(t *testing.T) {
	base := t.TempDir()
	now := time.Now().UTC()
	runDir := makeRunDir(t, base, "owner", "repo", "20260101-100000",
		RunMeta{Repo: "owner/repo", StartedAt: now})

	issueDir := filepath.Join(runDir, "issues", "1")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "outcome.json"), Outcome{
		IssueNumber: 1,
		PRNumber:    42,
	}); err != nil {
		t.Fatalf("writing outcome: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "implement.json"), StepResult{
		SessionID: "sess-impl-123",
	}); err != nil {
		t.Fatalf("writing implement: %v", err)
	}

	sid, err := FindSessionID(base, "owner/repo", 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "sess-impl-123" {
		t.Errorf("got session ID %q, want %q", sid, "sess-impl-123")
	}
}

func TestFindSessionID_FromLatestRetry(t *testing.T) {
	base := t.TempDir()
	now := time.Now().UTC()
	runDir := makeRunDir(t, base, "owner", "repo", "20260101-100000",
		RunMeta{Repo: "owner/repo", StartedAt: now})

	issueDir := filepath.Join(runDir, "issues", "1")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "outcome.json"), Outcome{
		IssueNumber: 1,
		PRNumber:    42,
	}); err != nil {
		t.Fatalf("writing outcome: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "implement.json"), StepResult{
		SessionID: "sess-impl-001",
	}); err != nil {
		t.Fatalf("writing implement: %v", err)
	}

	// Retry 1 with session ID "sess-retry-001".
	retry1Dir := filepath.Join(issueDir, "retries", "1")
	if err := os.MkdirAll(retry1Dir, 0o755); err != nil {
		t.Fatalf("creating retry dir: %v", err)
	}
	if err := writeJSON(filepath.Join(retry1Dir, "retry.json"), StepResult{
		SessionID: "sess-retry-001",
	}); err != nil {
		t.Fatalf("writing retry 1: %v", err)
	}

	// Retry 2 (highest) with session ID "sess-retry-002" — should win.
	retry2Dir := filepath.Join(issueDir, "retries", "2")
	if err := os.MkdirAll(retry2Dir, 0o755); err != nil {
		t.Fatalf("creating retry dir: %v", err)
	}
	if err := writeJSON(filepath.Join(retry2Dir, "retry.json"), StepResult{
		SessionID: "sess-retry-002",
	}); err != nil {
		t.Fatalf("writing retry 2: %v", err)
	}

	sid, err := FindSessionID(base, "owner/repo", 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "sess-retry-002" {
		t.Errorf("got session ID %q, want %q", sid, "sess-retry-002")
	}
}

func TestFindSessionID_MostRecentRunFirst(t *testing.T) {
	base := t.TempDir()
	now := time.Now().UTC()

	// Older run with session ID "sess-old".
	oldRunDir := makeRunDir(t, base, "owner", "repo", "20260101-100000",
		RunMeta{Repo: "owner/repo", StartedAt: now.Add(-24 * time.Hour)})
	oldIssueDir := filepath.Join(oldRunDir, "issues", "1")
	if err := os.MkdirAll(oldIssueDir, 0o755); err != nil {
		t.Fatalf("creating old issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(oldIssueDir, "outcome.json"), Outcome{IssueNumber: 1, PRNumber: 42}); err != nil {
		t.Fatalf("writing old outcome: %v", err)
	}
	if err := writeJSON(filepath.Join(oldIssueDir, "implement.json"), StepResult{SessionID: "sess-old"}); err != nil {
		t.Fatalf("writing old implement: %v", err)
	}

	// Newer run with session ID "sess-new" — should win.
	newRunDir := makeRunDir(t, base, "owner", "repo", "20260102-100000",
		RunMeta{Repo: "owner/repo", StartedAt: now})
	newIssueDir := filepath.Join(newRunDir, "issues", "1")
	if err := os.MkdirAll(newIssueDir, 0o755); err != nil {
		t.Fatalf("creating new issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(newIssueDir, "outcome.json"), Outcome{IssueNumber: 1, PRNumber: 42}); err != nil {
		t.Fatalf("writing new outcome: %v", err)
	}
	if err := writeJSON(filepath.Join(newIssueDir, "implement.json"), StepResult{SessionID: "sess-new"}); err != nil {
		t.Fatalf("writing new implement: %v", err)
	}

	sid, err := FindSessionID(base, "owner/repo", 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "sess-new" {
		t.Errorf("got session ID %q, want %q", sid, "sess-new")
	}
}

func TestFindSessionID_MissingSessionIDInImplement(t *testing.T) {
	base := t.TempDir()
	now := time.Now().UTC()
	runDir := makeRunDir(t, base, "owner", "repo", "20260101-100000",
		RunMeta{Repo: "owner/repo", StartedAt: now})

	issueDir := filepath.Join(runDir, "issues", "1")
	if err := os.MkdirAll(issueDir, 0o755); err != nil {
		t.Fatalf("creating issue dir: %v", err)
	}
	if err := writeJSON(filepath.Join(issueDir, "outcome.json"), Outcome{
		IssueNumber: 1,
		PRNumber:    42,
	}); err != nil {
		t.Fatalf("writing outcome: %v", err)
	}
	// implement.json with empty session ID — no session to resume.
	if err := writeJSON(filepath.Join(issueDir, "implement.json"), StepResult{
		Output: "done",
	}); err != nil {
		t.Fatalf("writing implement: %v", err)
	}

	sid, err := FindSessionID(base, "owner/repo", 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "" {
		t.Errorf("expected empty session ID, got %q", sid)
	}
}
