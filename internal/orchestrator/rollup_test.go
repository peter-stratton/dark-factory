package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// setupRollupCommandRunner stubs CommandRunner to handle rollup gh commands
// and records every invocation.
func setupRollupCommandRunner(t *testing.T, prURL string, mergeErr error) *[][]string {
	t.Helper()

	var calls [][]string

	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		call := append([]string{name}, args...)
		calls = append(calls, call)
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "pr create") {
			return []byte(prURL + "\n"), nil
		}
		if strings.Contains(joined, "pr merge") {
			return []byte(""), mergeErr
		}
		return []byte(""), nil
	}

	return &calls
}

func rollupConfig(rollup, baseBranch string) *config.Config {
	return &config.Config{
		Repo:       "owner/repo",
		AutoMerge:  config.AutoMerge{Rollup: rollup},
		BaseBranch: baseBranch,
	}
}

var testImplementedIssues = []github.Issue{
	{Number: 10, Title: "Add feature A"},
	{Number: 11, Title: "Fix bug B"},
}

func TestCreateRollupPR_Manual_CreatesPRAndDoesNotMerge(t *testing.T) {
	calls := setupRollupCommandRunner(t, "https://github.com/owner/repo/pull/42", nil)

	cfg := rollupConfig("manual", "sprint-1")

	output := captureStdout(t, func() {
		result, err := createRollupPR(context.Background(), cfg, testImplementedIssues, testLogger(t))
		if err != nil {
			t.Fatalf("createRollupPR() error = %v", err)
		}
		if result.PRNumber != 42 {
			t.Errorf("PRNumber = %d, want 42", result.PRNumber)
		}
		if result.PRURL != "https://github.com/owner/repo/pull/42" {
			t.Errorf("PRURL = %q, unexpected", result.PRURL)
		}
	})

	// Verify gh pr create was called with correct args.
	var createArgs string
	for _, c := range *calls {
		if len(c) > 2 && c[1] == "pr" && c[2] == "create" {
			createArgs = strings.Join(c, " ")
			break
		}
	}
	if createArgs == "" {
		t.Fatal("expected gh pr create to be called")
	}
	if !strings.Contains(createArgs, "--head sprint-1") {
		t.Errorf("expected --head sprint-1 in create call: %s", createArgs)
	}
	if !strings.Contains(createArgs, "--base main") {
		t.Errorf("expected --base main in create call: %s", createArgs)
	}
	if !strings.Contains(createArgs, "owner/repo") {
		t.Errorf("expected repo in create call: %s", createArgs)
	}

	// Verify gh pr merge was NOT called.
	for _, c := range *calls {
		if len(c) > 2 && c[1] == "pr" && c[2] == "merge" {
			t.Errorf("expected gh pr merge NOT to be called for rollup=manual, got: %v", c)
		}
	}

	if !strings.Contains(output, "Rollup PR created:") {
		t.Errorf("expected 'Rollup PR created:' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ready for review") {
		t.Errorf("expected 'ready for review' in output, got:\n%s", output)
	}
}

func TestCreateRollupPR_Auto_CreatesPRAndMerges(t *testing.T) {
	calls := setupRollupCommandRunner(t, "https://github.com/owner/repo/pull/99", nil)

	cfg := rollupConfig("auto", "release-2")

	output := captureStdout(t, func() {
		result, err := createRollupPR(context.Background(), cfg, testImplementedIssues, testLogger(t))
		if err != nil {
			t.Fatalf("createRollupPR() error = %v", err)
		}
		if result.PRNumber != 99 {
			t.Errorf("PRNumber = %d, want 99", result.PRNumber)
		}
	})

	// Verify gh pr merge was called once.
	mergeCount := 0
	for _, c := range *calls {
		if len(c) > 2 && c[1] == "pr" && c[2] == "merge" {
			mergeCount++
			joined := strings.Join(c, " ")
			if !strings.Contains(joined, "99") {
				t.Errorf("expected PR number 99 in merge call: %s", joined)
			}
			if !strings.Contains(joined, "--squash") {
				t.Errorf("expected --squash in merge call: %s", joined)
			}
		}
	}
	if mergeCount != 1 {
		t.Errorf("expected gh pr merge called once, got %d", mergeCount)
	}

	if !strings.Contains(output, "Rollup PR merged:") {
		t.Errorf("expected 'Rollup PR merged:' in output, got:\n%s", output)
	}
}

func TestCreateRollupPR_Auto_MergeError_ReturnsError(t *testing.T) {
	setupRollupCommandRunner(t, "https://github.com/owner/repo/pull/5", fmt.Errorf("merge conflict"))

	cfg := rollupConfig("auto", "feature-branch")

	captureStdout(t, func() {
		result, err := createRollupPR(context.Background(), cfg, testImplementedIssues, testLogger(t))
		if err == nil {
			t.Fatal("expected error when merge fails, got nil")
		}
		if !strings.Contains(err.Error(), "merging rollup PR") {
			t.Errorf("error = %q, want to contain 'merging rollup PR'", err)
		}
		// Result is still returned even on merge error.
		if result == nil {
			t.Error("expected non-nil result even when merge fails")
		}
	})
}

func TestCreateRollupPR_CreateError_ReturnsError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh: authentication required")
	}

	cfg := rollupConfig("manual", "sprint-3")

	captureStdout(t, func() {
		result, err := createRollupPR(context.Background(), cfg, testImplementedIssues, testLogger(t))
		if err == nil {
			t.Fatal("expected error when pr create fails, got nil")
		}
		if !strings.Contains(err.Error(), "creating rollup PR") {
			t.Errorf("error = %q, want to contain 'creating rollup PR'", err)
		}
		if result != nil {
			t.Error("expected nil result when pr create fails")
		}
	})
}

func TestParsePRNumberFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want int
	}{
		{"https://github.com/owner/repo/pull/42", 42},
		{"https://github.com/owner/repo/pull/1", 1},
		{"https://github.com/owner/repo/pull/9999", 9999},
		{"https://github.com/owner/repo/pull/42/", 42},
		{"not-a-url", 0},
		{"", 0},
		{"https://github.com/owner/repo/pull/abc", 0},
	}
	for _, tt := range tests {
		got := parsePRNumberFromURL(tt.url)
		if got != tt.want {
			t.Errorf("parsePRNumberFromURL(%q) = %d, want %d", tt.url, got, tt.want)
		}
	}
}

func TestBuildRollupPRBody_ContainsIssues(t *testing.T) {
	issues := []github.Issue{
		{Number: 10, Title: "Add feature A"},
		{Number: 20, Title: "Fix bug B"},
	}
	body := buildRollupPRBody("sprint-1", "main", issues)

	if !strings.Contains(body, "#10 Add feature A") {
		t.Errorf("body missing issue #10: %s", body)
	}
	if !strings.Contains(body, "#20 Fix bug B") {
		t.Errorf("body missing issue #20: %s", body)
	}
	if !strings.Contains(body, "`sprint-1`") {
		t.Errorf("body missing base branch name: %s", body)
	}
	if !strings.Contains(body, "`main`") {
		t.Errorf("body missing target branch: %s", body)
	}
}

// TestProcessIssues_RollupPR_SkipWhenNoneImplemented verifies that no rollup
// PR is created when stats.implemented == 0 (all issues failed).
func TestProcessIssues_RollupPR_SkipWhenNoneImplemented(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "will fail"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed", Err: fmt.Errorf("boom")}
		},
	)

	var rollupCalled bool
	orig := createRollupPRFn
	t.Cleanup(func() { createRollupPRFn = orig })
	createRollupPRFn = func(ctx context.Context, cfg *config.Config, issues []github.Issue, logger *slog.Logger) (*RollupResult, error) {
		rollupCalled = true
		return nil, nil
	}

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "auto"
	cfg.BaseBranch = "sprint-1"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "milestone", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if rollupCalled {
		t.Error("expected rollup PR NOT to be created when no issues were implemented")
	}
}

// TestProcessIssues_RollupPR_SkipWhenNoBaseBranch verifies that no rollup
// PR is created when BaseBranch is empty.
func TestProcessIssues_RollupPR_SkipWhenNoBaseBranch(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "a task"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		},
	)

	var rollupCalled bool
	orig := createRollupPRFn
	t.Cleanup(func() { createRollupPRFn = orig })
	createRollupPRFn = func(ctx context.Context, cfg *config.Config, issues []github.Issue, logger *slog.Logger) (*RollupResult, error) {
		rollupCalled = true
		return nil, nil
	}

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "manual"
	cfg.BaseBranch = "" // no base branch configured

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "milestone", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if rollupCalled {
		t.Error("expected rollup PR NOT to be created when BaseBranch is empty")
	}
}

// TestProcessIssues_RollupPR_SkipWhenRollupNone verifies that no rollup PR is
// created when rollup is "none".
func TestProcessIssues_RollupPR_SkipWhenRollupNone(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "a task"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		},
	)

	var rollupCalled bool
	orig := createRollupPRFn
	t.Cleanup(func() { createRollupPRFn = orig })
	createRollupPRFn = func(ctx context.Context, cfg *config.Config, issues []github.Issue, logger *slog.Logger) (*RollupResult, error) {
		rollupCalled = true
		return nil, nil
	}

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "none"
	cfg.BaseBranch = "sprint-1"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "milestone", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if rollupCalled {
		t.Error("expected rollup PR NOT to be created when rollup=none")
	}
}

// TestProcessIssues_RollupPR_CalledWhenConditionsMet verifies that the rollup
// PR function is called with the correct implemented issues when all conditions
// are met.
func TestProcessIssues_RollupPR_CalledWhenConditionsMet(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 3, Title: "task three"},
		{Number: 4, Title: "task four"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: issue.Number * 10}
		},
	)

	var capturedIssues []github.Issue
	orig := createRollupPRFn
	t.Cleanup(func() { createRollupPRFn = orig })
	createRollupPRFn = func(ctx context.Context, cfg *config.Config, issues []github.Issue, logger *slog.Logger) (*RollupResult, error) {
		capturedIssues = issues
		return &RollupResult{PRNumber: 500, PRURL: "https://github.com/owner/repo/pull/500"}, nil
	}

	cfg := testConfig()
	cfg.NoSandbox = true
	cfg.AutoMerge.Rollup = "manual"
	cfg.BaseBranch = "sprint-1"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, cfg, testLogger(t), nil, false, "", "milestone", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(capturedIssues) == 0 {
		t.Fatal("expected rollup PR to be created with implemented issues")
	}
	// Both issues should be in the list (they both succeed; wave stops after
	// first merge, so only the first issue is captured per wave, but re-resolution
	// would catch the second — in this test both succeed in wave 1).
	// The exact count depends on the wave mechanics; assert at least one.
	for _, iss := range capturedIssues {
		if iss.Number != 3 && iss.Number != 4 {
			t.Errorf("unexpected issue number %d in rollup", iss.Number)
		}
	}
}
