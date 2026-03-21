package agent

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// rebaseTestConfig returns a config suitable for runPreMergeRebasePhase tests.
// No verify commands are configured so the verify phase is a no-op.
func rebaseTestConfig() *config.Config {
	return &config.Config{
		Repo:              "owner/repo",
		NoSandbox:         true,
		AgentTimeout:      "10m",
		ProtectedPaths:    []string{"CLAUDE.md"},
		ScenarioDir:       "tests/scenarios/",
		ReviewDir:         "tests/review/",
		MaxRebaseAttempts: 1,
	}
}

// standardRebaseGuard returns a GuardRunner stub that satisfies the minimum
// responses needed for runPreMergeRebasePhase: drift diff and any other calls.
func standardRebaseGuard() func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	}
}

func TestRunPreMergeRebasePhase_Disabled(t *testing.T) {
	cfg := rebaseTestConfig()
	cfg.MaxRebaseAttempts = 0

	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		// Should not be called when MaxRebaseAttempts=0.
		return []byte(""), nil
	})

	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	called := false
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte(""), nil
	}

	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsHR {
		t.Error("expected needsHumanReview=false when MaxRebaseAttempts=0")
	}
	if called {
		t.Error("expected github.CommandRunner not to be called when MaxRebaseAttempts=0")
	}
}

func TestRunPreMergeRebasePhase_Mergeable(t *testing.T) {
	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json mergeable") {
			return []byte(`{"mergeable":"MERGEABLE"}`), nil
		}
		return []byte(""), nil
	}

	stubGuardRunner(t, standardRebaseGuard())

	cfg := rebaseTestConfig()
	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsHR {
		t.Error("expected needsHumanReview=false for MERGEABLE PR")
	}
}

func TestRunPreMergeRebasePhase_Unknown(t *testing.T) {
	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json mergeable") {
			return []byte(`{"mergeable":"UNKNOWN"}`), nil
		}
		return []byte(""), nil
	}

	stubGuardRunner(t, standardRebaseGuard())

	cfg := rebaseTestConfig()
	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsHR {
		t.Error("expected needsHumanReview=false for UNKNOWN mergeable status")
	}
}

func TestRunPreMergeRebasePhase_CheckMergeableError(t *testing.T) {
	// CheckMergeable failing is best-effort: should log warning and proceed (false, nil).
	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh: network error")
	}

	stubGuardRunner(t, standardRebaseGuard())

	cfg := rebaseTestConfig()
	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsHR {
		t.Error("expected needsHumanReview=false on CheckMergeable error (best-effort)")
	}
}

func TestRunPreMergeRebasePhase_ConflictingAutoRebaseSuccess(t *testing.T) {
	// PR is CONFLICTING. UpdateBranch succeeds. Final check returns MERGEABLE.
	checkCallCount := 0
	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json mergeable") {
			checkCallCount++
			if checkCallCount == 1 {
				return []byte(`{"mergeable":"CONFLICTING"}`), nil
			}
			// Final check after rebase: return MERGEABLE.
			return []byte(`{"mergeable":"MERGEABLE"}`), nil
		}
		if strings.Contains(joined, "update-branch") {
			return []byte(""), nil // update-branch succeeds
		}
		return []byte(""), nil
	}

	stubGuardRunner(t, standardRebaseGuard())

	cfg := rebaseTestConfig()
	cfg.MaxRebaseAttempts = 1
	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsHR {
		t.Errorf("expected needsHumanReview=false after successful auto-rebase")
	}
	if checkCallCount < 2 {
		t.Errorf("expected at least 2 CheckMergeable calls, got %d", checkCallCount)
	}
}

func TestRunPreMergeRebasePhase_ConflictingUpdateBranchFails_ImplementerFixes(t *testing.T) {
	// PR is CONFLICTING. UpdateBranch fails. Retry agent runs. Final check MERGEABLE.
	checkCallCount := 0
	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json mergeable") {
			checkCallCount++
			if checkCallCount == 1 {
				return []byte(`{"mergeable":"CONFLICTING"}`), nil
			}
			return []byte(`{"mergeable":"MERGEABLE"}`), nil
		}
		if strings.Contains(joined, "update-branch") {
			return nil, fmt.Errorf("exit status 1: conflicts") // update-branch fails
		}
		return []byte(""), nil
	}

	retryCallCount := 0
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, *syscall.Rusage, error) {
		retryCallCount++
		out := wrapRunnerJSON("conflict fix output")
		return []byte(out), []byte(""), 0, nil, nil
	})

	stubGuardRunner(t, standardRebaseGuard())

	cfg := rebaseTestConfig()
	cfg.MaxRebaseAttempts = 1
	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsHR {
		t.Errorf("expected needsHumanReview=false after implementer fixes conflicts")
	}
	if retryCallCount == 0 {
		t.Errorf("expected Retry agent to be called on update-branch failure, got 0 calls")
	}
}

func TestRunPreMergeRebasePhase_ExhaustsAttempts_NeedsHumanReview(t *testing.T) {
	// PR remains CONFLICTING after all rebase attempts.
	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json mergeable") {
			return []byte(`{"mergeable":"CONFLICTING"}`), nil
		}
		if strings.Contains(joined, "update-branch") {
			return nil, fmt.Errorf("exit status 1: conflicts")
		}
		return []byte(""), nil
	}

	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, *syscall.Rusage, error) {
		out := wrapRunnerJSON("conflict fix attempted but conflicts remain")
		return []byte(out), []byte(""), 0, nil, nil
	})

	stubGuardRunner(t, standardRebaseGuard())

	cfg := rebaseTestConfig()
	cfg.MaxRebaseAttempts = 1
	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsHR {
		t.Errorf("expected needsHumanReview=true when all rebase attempts exhausted")
	}
}

func TestRunPreMergeRebasePhase_MultipleAttempts(t *testing.T) {
	// With MaxRebaseAttempts=2, should try twice before marking needs-human-review.
	checkCallCount := 0
	retryCallCount := 0

	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json mergeable") {
			checkCallCount++
			return []byte(`{"mergeable":"CONFLICTING"}`), nil
		}
		if strings.Contains(joined, "update-branch") {
			return nil, fmt.Errorf("exit status 1: conflicts")
		}
		return []byte(""), nil
	}

	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, *syscall.Rusage, error) {
		retryCallCount++
		out := wrapRunnerJSON("conflict fix output")
		return []byte(out), []byte(""), 0, nil, nil
	})

	stubGuardRunner(t, standardRebaseGuard())

	cfg := rebaseTestConfig()
	cfg.MaxRebaseAttempts = 2
	sid := "sess"
	fc := 0
	needsHR, err := runPreMergeRebasePhase(
		context.Background(), testIssue(), 42, "42-test", "abc123",
		cfg, testPrompts(t), nil, testLogger(t), nil, &sid, &fc,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsHR {
		t.Error("expected needsHumanReview=true after exhausting 2 attempts")
	}
	// 2 loop checks + 1 final check = 3 total.
	if checkCallCount != 3 {
		t.Errorf("expected 3 CheckMergeable calls (2 loop + 1 final), got %d", checkCallCount)
	}
	if retryCallCount != 2 {
		t.Errorf("expected 2 Retry calls (one per attempt), got %d", retryCallCount)
	}
}

// TestProcessIssue_PreMergeRebase_LabelsNeedsHumanReview tests the full
// ProcessIssue flow when the pre-merge rebase phase exhausts attempts.
func TestProcessIssue_PreMergeRebase_LabelsNeedsHumanReview(t *testing.T) {
	// github.CommandRunner handles CheckMergeable (CONFLICTING) and UpdateBranch (fails),
	// plus AddIssueLabel for the needs-human-review label.
	var addedLabels []string
	origGH := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGH })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		for i, a := range args {
			if a == "--add-label" && i+1 < len(args) {
				addedLabels = append(addedLabels, args[i+1])
			}
		}
		if strings.Contains(joined, "--json mergeable") {
			return []byte(`{"mergeable":"CONFLICTING"}`), nil
		}
		if strings.Contains(joined, "update-branch") {
			return nil, fmt.Errorf("exit status 1: conflicts")
		}
		return []byte(""), nil
	}

	// Agent calls: implementer, quality_reviewer (APPROVED), reviewer (APPROVED), conflict fix
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
		"conflict fix output",
	}, standardLoopGuard())

	cfg := loopConfig()
	cfg.MaxRebaseAttempts = 1

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "needs-human-review" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "needs-human-review", outcome.Err)
	}
}
