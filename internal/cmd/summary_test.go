package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/stats"
)

// TestCollectMilestones verifies unique milestones are extracted in order.
func TestCollectMilestones(t *testing.T) {
	runs := []stats.RunRecord{
		{Milestone: "Phase 22"},
		{Milestone: "Phase 21"},
		{Milestone: "Phase 22"}, // duplicate
		{Milestone: ""},         // empty — should be skipped
	}

	got := collectMilestones(runs)
	want := []string{"Phase 22", "Phase 21"}

	if len(got) != len(want) {
		t.Fatalf("collectMilestones() = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("collectMilestones()[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// TestCollectMilestonesEmpty verifies empty input returns nil.
func TestCollectMilestonesEmpty(t *testing.T) {
	got := collectMilestones(nil)
	if len(got) != 0 {
		t.Errorf("collectMilestones(nil) = %v, want empty", got)
	}
}

// TestReadPhaseOverviewNoFile verifies an empty string is returned when no
// phase overview file exists.
func TestReadPhaseOverviewNoFile(t *testing.T) {
	got := readPhaseOverview("Phase 99")
	if got != "" {
		t.Errorf("readPhaseOverview for non-existent phase = %q, want empty", got)
	}
}

// TestReadPhaseOverviewBadMilestone verifies a non-matching milestone returns empty.
func TestReadPhaseOverviewBadMilestone(t *testing.T) {
	for _, m := range []string{"", "Sprint 5", "v1.2.3"} {
		if got := readPhaseOverview(m); got != "" {
			t.Errorf("readPhaseOverview(%q) = %q, want empty", m, got)
		}
	}
}

// TestReadPhaseOverviewWithFile verifies the file content is returned when an
// overview file exists at the expected path.
func TestReadPhaseOverviewWithFile(t *testing.T) {
	// Create a temporary directory tree that looks like docs/phase-overviews/.
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs", "phase-overviews")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	overviewPath := filepath.Join(docsDir, "phase-42-test-overview.md")
	const content = "# Phase 42\n\nThis phase built the test harness."
	if err := os.WriteFile(overviewPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Change working directory so the relative glob resolves to the temp tree.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("warning: could not restore working directory: %v", err)
		}
	}()

	got := readPhaseOverview("Phase 42")
	if got != content {
		t.Errorf("readPhaseOverview(\"Phase 42\") = %q, want %q", got, content)
	}
}

// TestBuildSummaryPromptContainsIssueTitles verifies that issue titles appear
// in the generated prompt.
func TestBuildSummaryPromptContainsIssueTitles(t *testing.T) {
	runs := []stats.RunRecord{{ID: "r1", Milestone: "Phase 22"}}
	outcomes := []stats.IssueOutcomeRecord{
		{IssueNumber: 100, Title: "Add dashboard widget", Status: "implemented"},
		{IssueNumber: 101, Title: "Fix login timeout", Status: "implemented"},
	}

	prompt := buildSummaryPrompt(runs, outcomes)

	for _, title := range []string{"Add dashboard widget", "Fix login timeout"} {
		if !strings.Contains(prompt, title) {
			t.Errorf("prompt missing issue title %q", title)
		}
	}
	if !strings.Contains(prompt, "executive summary") {
		t.Error("prompt missing 'executive summary' instruction")
	}
}

// TestBuildSummaryPromptNoIssues verifies the prompt handles no outcomes gracefully.
func TestBuildSummaryPromptNoIssues(t *testing.T) {
	runs := []stats.RunRecord{{ID: "r1", Milestone: "Phase 22"}}
	prompt := buildSummaryPrompt(runs, nil)

	if !strings.Contains(prompt, "No issues processed") {
		t.Errorf("prompt for empty outcomes should mention 'No issues processed', got: %s", prompt)
	}
}
