package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// TestBuildRollupBody verifies the rollup PR body includes branches and issue list.
func TestBuildRollupBody(t *testing.T) {
	issues := []implementedIssue{
		{Number: 1, Title: "First feature"},
		{Number: 2, Title: "Second feature"},
	}
	body := buildRollupBody("phase-18", "main", issues)

	if !strings.Contains(body, "phase-18") {
		t.Errorf("body missing base branch 'phase-18':\n%s", body)
	}
	if !strings.Contains(body, "main") {
		t.Errorf("body missing default branch 'main':\n%s", body)
	}
	if !strings.Contains(body, "#1 First feature") {
		t.Errorf("body missing issue #1:\n%s", body)
	}
	if !strings.Contains(body, "#2 Second feature") {
		t.Errorf("body missing issue #2:\n%s", body)
	}
}

// TestBuildRollupBody_Empty verifies body with no issues still renders.
func TestBuildRollupBody_Empty(t *testing.T) {
	body := buildRollupBody("feature", "main", nil)
	if body == "" {
		t.Error("expected non-empty body even with no issues")
	}
}

// TestCreateRollupPR_Manual verifies that rollup=manual creates a PR but does
// not merge it.
func TestCreateRollupPR_Manual(t *testing.T) {
	var ghCalls [][]string
	origCmd := CommandRunner
	t.Cleanup(func() { CommandRunner = origCmd })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		ghCalls = append(ghCalls, append([]string{name}, args...))
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return json.Marshal(map[string]interface{}{
				"number": 42,
				"url":    "https://github.com/owner/repo/pull/42",
			})
		}
		return []byte("{}"), nil
	}

	cfg := &config.Config{
		Repo:       "owner/repo",
		BaseBranch: "phase-18",
		AutoMerge:  config.AutoMerge{Rollup: "manual"},
	}
	issues := []implementedIssue{{Number: 1, Title: "Feature A"}}

	result, err := createRollupPR(context.Background(), cfg, issues, nil)
	if err != nil {
		t.Fatalf("createRollupPR() error = %v", err)
	}
	if result.PRNumber != 42 {
		t.Errorf("PRNumber = %d, want 42", result.PRNumber)
	}
	if result.Merged {
		t.Error("Merged = true, want false for rollup=manual")
	}

	// Verify that pr merge was NOT called.
	for _, call := range ghCalls {
		if len(call) >= 3 && call[0] == "gh" && call[1] == "pr" && call[2] == "merge" {
			t.Errorf("unexpected 'gh pr merge' call for rollup=manual: %v", call)
		}
	}
}

// TestCreateRollupPR_Auto verifies that rollup=auto creates and merges the PR.
func TestCreateRollupPR_Auto(t *testing.T) {
	var ghCalls [][]string
	origCmd := CommandRunner
	t.Cleanup(func() { CommandRunner = origCmd })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		ghCalls = append(ghCalls, append([]string{name}, args...))
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return json.Marshal(map[string]interface{}{
				"number": 55,
				"url":    "https://github.com/owner/repo/pull/55",
			})
		}
		return []byte(""), nil
	}

	cfg := &config.Config{
		Repo:       "owner/repo",
		BaseBranch: "phase-18",
		AutoMerge:  config.AutoMerge{Rollup: "auto"},
	}
	issues := []implementedIssue{{Number: 3, Title: "Feature C"}}

	result, err := createRollupPR(context.Background(), cfg, issues, nil)
	if err != nil {
		t.Fatalf("createRollupPR() error = %v", err)
	}
	if result.PRNumber != 55 {
		t.Errorf("PRNumber = %d, want 55", result.PRNumber)
	}
	if !result.Merged {
		t.Error("Merged = false, want true for rollup=auto")
	}

	// Verify that pr merge WAS called.
	mergeFound := false
	for _, call := range ghCalls {
		if len(call) >= 3 && call[0] == "gh" && call[1] == "pr" && call[2] == "merge" {
			mergeFound = true
		}
	}
	if !mergeFound {
		t.Error("expected 'gh pr merge' call for rollup=auto, none found")
	}
}

// TestCreateRollupPR_PRCreateError verifies that an error from gh pr create
// is propagated correctly.
func TestCreateRollupPR_PRCreateError(t *testing.T) {
	origCmd := CommandRunner
	t.Cleanup(func() { CommandRunner = origCmd })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "create" {
			return []byte("error output"), fmt.Errorf("gh pr create failed")
		}
		return []byte(""), nil
	}

	cfg := &config.Config{
		Repo:       "owner/repo",
		BaseBranch: "phase-18",
		AutoMerge:  config.AutoMerge{Rollup: "manual"},
	}
	issues := []implementedIssue{{Number: 1, Title: "A"}}

	_, err := createRollupPR(context.Background(), cfg, issues, nil)
	if err == nil {
		t.Fatal("expected error from createRollupPR, got nil")
	}
	if !strings.Contains(err.Error(), "creating rollup PR") {
		t.Errorf("error = %q, want mention of 'creating rollup PR'", err.Error())
	}
}

// stubRollupFn is a helper to install a stub for createRollupPRFn.
func stubRollupFn(t *testing.T, fn func(ctx context.Context, cfg *config.Config, issues []implementedIssue, logger *slog.Logger) (RollupResult, error)) {
	t.Helper()
	orig := createRollupPRFn
	t.Cleanup(func() { createRollupPRFn = orig })
	createRollupPRFn = fn
}

// TestProcessIssues_RollupNone verifies that rollup=none does not create any
// rollup PR even when issues are implemented.
func TestProcessIssues_RollupNone(t *testing.T) {
	var rollupCalled bool
	stubRollupFn(t, func(_ context.Context, _ *config.Config, _ []implementedIssue, _ *slog.Logger) (RollupResult, error) {
		rollupCalled = true
		return RollupResult{}, nil
	})

	allIssues := []github.Issue{{Number: 1, Title: "feature"}}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook) agent.IssueOutcome {
			// PRNumber: 0 avoids triggering the punchlist goroutine.
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 0}
		},
	)

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "none"
	cfg.BaseBranch = "phase-18"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "test", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if rollupCalled {
		t.Error("createRollupPRFn called for rollup=none, expected no-op")
	}
}

// TestProcessIssues_RollupSkipNoBaseBranch verifies that when BaseBranch is
// empty, no rollup PR is created even if rollup=manual.
func TestProcessIssues_RollupSkipNoBaseBranch(t *testing.T) {
	var rollupCalled bool
	stubRollupFn(t, func(_ context.Context, _ *config.Config, _ []implementedIssue, _ *slog.Logger) (RollupResult, error) {
		rollupCalled = true
		return RollupResult{}, nil
	})

	allIssues := []github.Issue{{Number: 1, Title: "feature"}}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook) agent.IssueOutcome {
			// PRNumber: 0 avoids triggering the punchlist goroutine.
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 0}
		},
	)

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "manual"
	cfg.BaseBranch = "" // empty — should skip

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "test", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if rollupCalled {
		t.Error("createRollupPRFn called when BaseBranch empty, expected skip")
	}
}

// TestProcessIssues_RollupSkipNoImplemented verifies that when no issues were
// implemented, no rollup PR is created.
func TestProcessIssues_RollupSkipNoImplemented(t *testing.T) {
	var rollupCalled bool
	stubRollupFn(t, func(_ context.Context, _ *config.Config, _ []implementedIssue, _ *slog.Logger) (RollupResult, error) {
		rollupCalled = true
		return RollupResult{}, nil
	})

	allIssues := []github.Issue{{Number: 1, Title: "fails"}}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed", Err: fmt.Errorf("test failure")}
		},
	)

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "manual"
	cfg.BaseBranch = "phase-18"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "test", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if rollupCalled {
		t.Error("createRollupPRFn called when no issues implemented, expected skip")
	}
}

// TestProcessIssues_RollupManualCreates verifies that rollup=manual triggers
// rollup PR creation and outputs the PR URL.
func TestProcessIssues_RollupManualCreates(t *testing.T) {
	var capturedIssues []implementedIssue
	stubRollupFn(t, func(_ context.Context, _ *config.Config, issues []implementedIssue, _ *slog.Logger) (RollupResult, error) {
		capturedIssues = issues
		return RollupResult{
			PRNumber: 77,
			PRURL:    "https://github.com/owner/repo/pull/77",
			Merged:   false,
		}, nil
	})

	allIssues := []github.Issue{{Number: 5, Title: "great feature"}}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook) agent.IssueOutcome {
			// PRNumber: 0 avoids triggering the punchlist goroutine.
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 0}
		},
	)

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "manual"
	cfg.BaseBranch = "phase-18"

	output := captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "test", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(capturedIssues) != 1 || capturedIssues[0].Number != 5 {
		t.Errorf("capturedIssues = %v, want [{5, 'great feature'}]", capturedIssues)
	}
	if !strings.Contains(output, "Rollup PR: #77") {
		t.Errorf("expected rollup PR output with #77, got:\n%s", output)
	}
	if !strings.Contains(output, "awaiting human review") {
		t.Errorf("expected 'awaiting human review' for rollup=manual, got:\n%s", output)
	}
}

// TestProcessIssues_RollupAutoMerges verifies that rollup=auto triggers
// PR creation, merges it, and outputs the correct message.
func TestProcessIssues_RollupAutoMerges(t *testing.T) {
	stubRollupFn(t, func(_ context.Context, _ *config.Config, _ []implementedIssue, _ *slog.Logger) (RollupResult, error) {
		return RollupResult{
			PRNumber: 88,
			PRURL:    "https://github.com/owner/repo/pull/88",
			Merged:   true,
		}, nil
	})

	allIssues := []github.Issue{{Number: 7, Title: "another feature"}}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook) agent.IssueOutcome {
			// PRNumber: 0 avoids triggering the punchlist goroutine.
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 0}
		},
	)

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "auto"
	cfg.BaseBranch = "phase-18"

	output := captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "test", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if !strings.Contains(output, "Rollup PR: #88 merged") {
		t.Errorf("expected 'Rollup PR: #88 merged' in output, got:\n%s", output)
	}
}
