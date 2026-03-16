package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/progress"
	"github.com/phs/dark-factory/internal/label"
	"github.com/phs/dark-factory/internal/quality"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/sandbox"
)

// setupSandboxDockerStubs stubs sandbox.CommandRunner, sandbox.CommandRunnerWithContext,
// and sandbox.SplitRunner to simulate Docker for agent runs in sandbox mode.
// The agentOutputs slice provides the stdout for successive agent container runs.
// It returns a cleanup function.
func setupSandboxDockerStubs(t *testing.T, agentOutputs []string) {
	t.Helper()
	agentCallIdx := 0

	origCmd := sandbox.CommandRunner
	origWait := sandbox.CommandRunnerWithContext
	origSplit := sandbox.SplitRunner
	t.Cleanup(func() {
		sandbox.CommandRunner = origCmd
		sandbox.CommandRunnerWithContext = origWait
		sandbox.SplitRunner = origSplit
	})

	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "create" {
			return []byte("fake-container-id\n"), nil
		}
		return []byte(""), nil
	}
	sandbox.CommandRunnerWithContext = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte("0\n"), nil
	}
	sandbox.SplitRunner = func(name string, args ...string) ([]byte, []byte, error) {
		// Return the next agent output as the container stdout.
		var out string
		if agentCallIdx < len(agentOutputs) {
			out = wrapRunnerJSON(agentOutputs[agentCallIdx])
			agentCallIdx++
		}
		return []byte(out), []byte(""), nil
	}
}

// loopTestSetup stubs both Runner (for agent invocations) and GuardRunner
// (for git/gh commands) and returns the config. The caller configures
// specific behavior via the provided function maps.
type loopStubs struct {
	// agentOutputs maps call index to stdout (simulating agent runs)
	agentCalls   int
	agentOutputs []string

	// guardCalls records all GuardRunner invocations
	guardCalls [][]string
}

func (s *loopStubs) nextAgentOutput() string {
	idx := s.agentCalls
	s.agentCalls++
	if idx < len(s.agentOutputs) {
		return s.agentOutputs[idx]
	}
	return ""
}

func setupLoopTest(t *testing.T, agentOutputs []string, guardFn func(name string, args ...string) ([]byte, error)) *loopStubs {
	t.Helper()
	stubs := &loopStubs{agentOutputs: agentOutputs}

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := stubs.nextAgentOutput()
		return []byte(wrapRunnerJSON(out)), []byte(""), 0, nil
	}

	GuardRunner = func(name string, args ...string) ([]byte, error) {
		stubs.guardCalls = append(stubs.guardCalls, append([]string{name}, args...))
		if guardFn != nil {
			return guardFn(name, args...)
		}
		return []byte(""), nil
	}

	return stubs
}

func loopConfig() *config.Config {
	return &config.Config{
		Repo:           "owner/repo",
		NoSandbox:      true,
		AutoMerge:      config.AutoMerge{Feature: "all", Rollup: "none"},
		MaxRetries:     2,
		AgentTimeout:   "10m",
		ProtectedPaths: []string{"CLAUDE.md"},
		ScenarioDir:    "/nonexistent-scenario-dir",
		ReviewDir:      "tests/review/",
	}
}

func loopIssue() github.Issue {
	return github.Issue{Number: 5, Title: "Test Issue", Body: "body"}
}

func TestProcessIssue_ImplementedOnFirstApproval(t *testing.T) {
	// Agent outputs: 1=implementer, 2=quality_reviewer(APPROVED), 3=reviewer(APPROVED)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if outcome.PRNumber != 10 {
		t.Errorf("PRNumber = %d, want 10", outcome.PRNumber)
	}
	if outcome.Retries != 0 {
		t.Errorf("Retries = %d, want 0", outcome.Retries)
	}
}

func TestProcessIssue_RetriesOnChangesRequested(t *testing.T) {
	// Agent outputs: implementer, quality(APPROVED), reviewer(CHANGES), retry, reviewer(APPROVED)
	callIdx := 0
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		callIdx++
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if outcome.Retries != 1 {
		t.Errorf("Retries = %d, want 1", outcome.Retries)
	}
}

func TestProcessIssue_NeedsHumanReviewAfterMaxRetries(t *testing.T) {
	cfg := loopConfig()
	cfg.MaxRetries = 1

	// Agent outputs: implementer, quality(APPROVED), reviewer(CHANGES), retry, reviewer(CHANGES)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
		"AGENT_RESULT=CHANGES_REQUESTED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "needs-human-review" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "needs-human-review", outcome.Err)
	}
}

func TestProcessIssue_FailedWhenNoPRFound(t *testing.T) {
	setupLoopTest(t, []string{"implementer output"}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") {
			return nil, fmt.Errorf("no PR found")
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want %q", outcome.Status, "failed")
	}
	if !strings.Contains(outcome.Err.Error(), "did not create a PR") {
		t.Errorf("Err = %v, want 'did not create a PR'", outcome.Err)
	}
}

func TestProcessIssue_FailedOnProtectedDrift(t *testing.T) {
	setupLoopTest(t, []string{"implementer output"}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("CLAUDE.md\nsrc/main.go\n"), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want %q", outcome.Status, "failed")
	}
	if !strings.Contains(outcome.Err.Error(), "protected path drift") {
		t.Errorf("Err = %v, want 'protected path drift'", outcome.Err)
	}
}

func TestProcessIssue_RechecksProtectedDriftAfterRetry(t *testing.T) {
	driftCheckCount := 0
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			driftCheckCount++
			if driftCheckCount == 2 {
				// Second drift check (after retry) shows protected file modified
				return []byte("CLAUDE.md\n"), nil
			}
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want %q", outcome.Status, "failed")
	}
	if driftCheckCount < 2 {
		t.Errorf("expected at least 2 drift checks, got %d", driftCheckCount)
	}
}

func TestProcessIssue_MergeOnApproval(t *testing.T) {
	var mergedCalled bool
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr merge") {
			mergedCalled = true
			if !strings.Contains(joined, "--squash") {
				t.Error("expected --squash flag")
			}
			if !strings.Contains(joined, "--delete-branch") {
				t.Error("expected --delete-branch flag")
			}
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if !mergedCalled {
		t.Error("expected gh pr merge to be called")
	}
}

func TestShouldHandoff(t *testing.T) {
	tests := []struct {
		name             string
		attempt          int
		maxResumeRetries int
		want             bool
	}{
		{"at threshold returns true", 2, 2, true},
		{"below threshold returns false", 1, 2, false},
		{"zero threshold returns true", 0, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldHandoff(tc.attempt, tc.maxResumeRetries)
			if got != tc.want {
				t.Errorf("shouldHandoff(%d, %d) = %v, want %v", tc.attempt, tc.maxResumeRetries, got, tc.want)
			}
		})
	}
}

func TestCheckDriftAndClose_NoDrift(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("src/main.go\n"), nil
	})

	cfg := loopConfig()
	err := checkDriftAndClose("abc123", cfg, 10, testLogger(t))
	if err != nil {
		t.Errorf("expected no error when no drift, got: %v", err)
	}
}

func TestCheckDriftAndClose_Drift(t *testing.T) {
	var closeCalled bool
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "diff --name-only") {
			return []byte("CLAUDE.md\nsrc/main.go\n"), nil
		}
		if strings.Contains(joined, "pr close") {
			closeCalled = true
		}
		return []byte(""), nil
	})

	cfg := loopConfig()
	err := checkDriftAndClose("abc123", cfg, 10, testLogger(t))
	if err == nil {
		t.Fatal("expected error when drift detected")
	}
	if !strings.Contains(err.Error(), "protected path drift") {
		t.Errorf("error = %v, want 'protected path drift'", err)
	}
	if !closeCalled {
		t.Error("expected PR to be closed")
	}
}

func TestCheckDriftAndClose_GitDiffError(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("git error")
	})

	cfg := loopConfig()
	err := checkDriftAndClose("abc123", cfg, 10, testLogger(t))
	if err != nil {
		t.Errorf("expected nil error on git diff failure (non-fatal), got: %v", err)
	}
}

func TestProcessIssue_SkipsSpecGenWhenNoPrompt(t *testing.T) {
	agentCallCount := 0
	setupLoopTest(t, []string{
		"implementer output",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	// Override Runner to count agent calls.
	origRunner := Runner
	t.Cleanup(func() { Runner = origRunner })
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		agentCallCount++
		switch agentCallCount {
		case 1:
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2:
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default:
			return []byte(wrapRunnerJSON("reviewer output\nAGENT_RESULT=APPROVED\n")), []byte(""), 0, nil
		}
	}

	// No SpecGenerator prompt → should only have 3 agent calls (implement + quality + review).
	prompts := testPrompts(t)
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if agentCallCount != 3 {
		t.Errorf("expected 3 agent calls (implement + quality + review), got %d", agentCallCount)
	}
}

// loopGuardFn returns a GuardRunner stub for standard loop tests (rev-parse, PR view, diff).
func loopGuardFn(name string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
		return []byte("abc123\n"), nil
	}
	if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
		return []byte(`{"number": 10}`), nil
	}
	if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
		return []byte(`{"body": "Closes #5"}`), nil
	}
	if name == "git" && strings.Contains(joined, "diff --name-only") {
		return []byte("src/main.go\n"), nil
	}
	return []byte(""), nil
}

func TestProcessIssue_PassesSessionIDToFirstRetry(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var retryEnv map[string]string

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer — returns a session ID
			out := `{"session_id":"sess-impl-001","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // quality reviewer — approve
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		case 3: // reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 4: // first retry — capture env
			retryEnv = make(map[string]string, len(env))
			for k, v := range env {
				retryEnv[k] = v
			}
			out := `{"session_id":"sess-retry-002","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		default: // reviewer after retry — approve
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if retryEnv == nil {
		t.Fatal("retry was never called")
	}
	if retryEnv["GODARK_SESSION_ID"] != "sess-impl-001" {
		t.Errorf("GODARK_SESSION_ID passed to first retry = %q, want %q", retryEnv["GODARK_SESSION_ID"], "sess-impl-001")
	}
}

func TestProcessIssue_UpdatesSessionIDFromRetryResult(t *testing.T) {
	cfg := loopConfig()
	cfg.MaxRetries = 2

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var secondRetryEnv map[string]string

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			out := `{"session_id":"sess-impl","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // quality reviewer — approve
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		case 3: // reviewer — changes requested
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 4: // first retry — returns its own session ID
			out := `{"session_id":"sess-retry-1","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 5: // reviewer — changes requested again
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 6: // second retry — capture env
			secondRetryEnv = make(map[string]string, len(env))
			for k, v := range env {
				secondRetryEnv[k] = v
			}
			out := `{"session_id":"sess-retry-2","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		default: // reviewer after second retry — approve
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if secondRetryEnv == nil {
		t.Fatal("second retry was never called")
	}
	if secondRetryEnv["GODARK_SESSION_ID"] != "sess-retry-1" {
		t.Errorf("GODARK_SESSION_ID passed to second retry = %q, want %q", secondRetryEnv["GODARK_SESSION_ID"], "sess-retry-1")
	}
}

func TestProcessIssue_ReviewerHasNoSessionID(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var reviewerEnvs []map[string]string

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer — returns a session ID
			out := `{"session_id":"sess-impl-001","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // quality reviewer — approve
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer calls — capture env
			captured := make(map[string]string, len(env))
			for k, v := range env {
				captured[k] = v
			}
			reviewerEnvs = append(reviewerEnvs, captured)
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if len(reviewerEnvs) == 0 {
		t.Fatal("reviewer was never called")
	}
	for i, env := range reviewerEnvs {
		if _, ok := env["GODARK_SESSION_ID"]; ok {
			t.Errorf("reviewer call %d should not receive GODARK_SESSION_ID, got %q", i+1, env["GODARK_SESSION_ID"])
		}
	}
}

func TestProcessIssue_QualityReviewChangesRequestedTriggersRetry(t *testing.T) {
	// Agent outputs: implementer, quality(CHANGES), retry, quality(APPROVED), reviewer(APPROVED)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
}

func TestProcessIssue_PassesCycleToQualityReview(t *testing.T) {
	cfg := loopConfig()
	cfg.QualityStrictnessDecay = true
	// MaxRetries=2 → qualityMaxAttempts=3

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var qualityPrompts []string

	// Prompts with StrictnessDirective variable so we can observe the rendered output.
	prompts := testPrompts(t)
	prompts.QualityReviewer = "Quality review PR #{{.PRNumber}}{{if .StrictnessDirective}}\n{{.StrictnessDirective}}{{end}}"

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality review cycle=0 (should have no directive)
			if env["GODARK_ROLE"] == "quality_reviewer" {
				qualityPrompts = append(qualityPrompts, env["GODARK_PROMPT"])
			}
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 3: // retry
			return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
		case 4: // quality review cycle=1 (should have directive)
			if env["GODARK_ROLE"] == "quality_reviewer" {
				qualityPrompts = append(qualityPrompts, env["GODARK_PROMPT"])
			}
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), cfg, prompts, nil, testLogger(t), nil, nil)

	if len(qualityPrompts) != 2 {
		t.Fatalf("expected 2 quality review calls, got %d", len(qualityPrompts))
	}
	// First call (cycle=0) should not have directive.
	if strings.Contains(qualityPrompts[0], "STRICTNESS OVERRIDE") {
		t.Errorf("first quality review (cycle=0) should not have strictness directive, got: %q", qualityPrompts[0])
	}
	// Second call (cycle=1, maxAttempts=3, not final) should have the narrowing directive.
	if !strings.Contains(qualityPrompts[1], "STRICTNESS OVERRIDE") {
		t.Errorf("second quality review (cycle=1) should have strictness directive, got: %q", qualityPrompts[1])
	}
	if !strings.Contains(qualityPrompts[1], "security vulnerabilities and correctness issues only") {
		t.Errorf("second quality review (cycle=1) should narrow to security+correctness, got: %q", qualityPrompts[1])
	}
}

func TestProcessIssue_AutoMergeNone_SkipsMergeOnApproval(t *testing.T) {
	var mergeCallCount int
	cfg := loopConfig()
	cfg.AutoMerge.Feature = "none"

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr merge") {
			mergeCallCount++
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "ready-to-merge", outcome.Err)
	}
	if mergeCallCount != 0 {
		t.Errorf("gh pr merge was called %d time(s), want 0 when AutoMerge=none", mergeCallCount)
	}
	if outcome.PRNumber != 10 {
		t.Errorf("PRNumber = %d, want 10", outcome.PRNumber)
	}
}

func TestProcessIssue_AutoMergeNone_OutcomeStatus(t *testing.T) {
	cfg := loopConfig()
	cfg.AutoMerge.Feature = "none"

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q", outcome.Status, "ready-to-merge")
	}
}

func TestProcessIssue_AutoMergeEmpty_SkipsMerge(t *testing.T) {
	// An empty AutoMerge value (e.g. from a YAML parsing edge case) must
	// never attempt to merge. The switch/default path should treat it like "none".
	var mergeCallCount int
	cfg := loopConfig()
	cfg.AutoMerge.Feature = ""

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr merge") {
			mergeCallCount++
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q when AutoMerge is empty", outcome.Status, "ready-to-merge")
	}
	if mergeCallCount > 0 {
		t.Errorf("gh pr merge was called %d time(s), want 0 when AutoMerge is empty", mergeCallCount)
	}
}

func TestProcessIssue_AutoMergeAll_Merges(t *testing.T) {
	// With AutoMerge=all, approved PR should be merged.
	var mergeCallCount int
	cfg := loopConfig() // AutoMerge defaults to "all" in loopConfig

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr merge") {
			mergeCallCount++
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if mergeCallCount != 1 {
		t.Errorf("gh pr merge was called %d time(s), want 1 when AutoMerge=all", mergeCallCount)
	}
}

func TestProcessIssue_AutoMergeNone_QualityReviewStillRuns(t *testing.T) {
	// With AutoMerge=none, quality review should still execute normally.
	cfg := loopConfig()
	cfg.AutoMerge.Feature = "none"

	agentCallCount := 0
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		agentCallCount++
		switch agentCallCount {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer
			if env["GODARK_ROLE"] != "quality_reviewer" {
				return []byte(wrapRunnerJSON("unexpected role: " + env["GODARK_ROLE"])), []byte(""), 0, nil
			}
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q", outcome.Status, "ready-to-merge")
	}
	if agentCallCount < 3 {
		t.Errorf("expected at least 3 agent calls (implement + quality + review), got %d", agentCallCount)
	}
}

func TestProcessIssue_SkipsQualityReviewWhenNoPrompt(t *testing.T) {
	agentCallCount := 0
	setupLoopTest(t, nil, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	origRunner := Runner
	t.Cleanup(func() { Runner = origRunner })
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		agentCallCount++
		switch agentCallCount {
		case 1:
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		default:
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	prompts := testPrompts(t)
	prompts.QualityReviewer = "" // disable quality reviewer
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if agentCallCount != 2 {
		t.Errorf("expected 2 agent calls (implement + review), got %d", agentCallCount)
	}
}

// testRunDataHook is a simple RunDataHook implementation for unit tests.
type testRunDataHook struct {
	reconCalls                 int
	specGeneratorCalls         int
	implementCalls             int
	reviewKinds                []string
	retryCalls                 int
	retryReviewCalls           int
	retryFunctionalReviewCalls int
	verifyResults              []rundata.VerifyStepResult
	outcomes                   []rundata.Outcome
	issueStatuses              []rundata.IssueStatus
	riskAssessments            []rundata.RiskAssessment
}

func (h *testRunDataHook) WriteReconResult(_ int, _ rundata.StepResult) error {
	h.reconCalls++
	return nil
}
func (h *testRunDataHook) WriteSpecGeneratorResult(_ int, _ rundata.StepResult) error {
	h.specGeneratorCalls++
	return nil
}
func (h *testRunDataHook) WriteImplementResult(_ int, _ rundata.StepResult) error {
	h.implementCalls++
	return nil
}
func (h *testRunDataHook) WriteReviewResult(_ int, kind string, _ rundata.StepResult) error {
	h.reviewKinds = append(h.reviewKinds, kind)
	return nil
}
func (h *testRunDataHook) WriteRetryResult(_ int, _ int, _ rundata.StepResult) error {
	h.retryCalls++
	return nil
}
func (h *testRunDataHook) WriteRetryReviewResult(_ int, _ int, _ rundata.StepResult) error {
	h.retryReviewCalls++
	return nil
}
func (h *testRunDataHook) WriteRetryFunctionalReviewResult(_ int, _ int, _ rundata.StepResult) error {
	h.retryFunctionalReviewCalls++
	return nil
}
func (h *testRunDataHook) WriteVerifyResult(_ int, step rundata.VerifyStepResult) error {
	h.verifyResults = append(h.verifyResults, step)
	return nil
}
func (h *testRunDataHook) WriteOutcome(o rundata.Outcome) error {
	h.outcomes = append(h.outcomes, o)
	return nil
}
func (h *testRunDataHook) WriteIssueStatus(_ int, s rundata.IssueStatus) error {
	h.issueStatuses = append(h.issueStatuses, s)
	return nil
}
func (h *testRunDataHook) WriteRiskAssessment(_ int, a rundata.RiskAssessment) error {
	h.riskAssessments = append(h.riskAssessments, a)
	return nil
}
func (h *testRunDataHook) WriteFailureAnalysis(_ int, _ rundata.FailureAnalysis) error { return nil }
func (h *testRunDataHook) WriteContainerLog(_ int, _ string) error                     { return nil }

func TestProcessIssue_HookCalledOnImplement(t *testing.T) {
	hook := &testRunDataHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook, nil)

	if hook.implementCalls != 1 {
		t.Errorf("WriteImplementResult called %d times, want 1", hook.implementCalls)
	}
}

func TestProcessIssue_HookCalledOnReview(t *testing.T) {
	hook := &testRunDataHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook, nil)

	if len(hook.reviewKinds) == 0 {
		t.Fatal("WriteReviewResult was never called")
	}
	// Quality review then functional review both expected.
	foundQuality := false
	foundFunctional := false
	for _, k := range hook.reviewKinds {
		if k == "quality" {
			foundQuality = true
		}
		if k == "functional" {
			foundFunctional = true
		}
	}
	if !foundQuality {
		t.Errorf("WriteReviewResult(\"quality\") not called; calls: %v", hook.reviewKinds)
	}
	if !foundFunctional {
		t.Errorf("WriteReviewResult(\"functional\") not called; calls: %v", hook.reviewKinds)
	}
}

func TestProcessIssue_HookCalledOnOutcome(t *testing.T) {
	hook := &testRunDataHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook, nil)

	if len(hook.outcomes) != 1 {
		t.Fatalf("WriteOutcome called %d times, want 1", len(hook.outcomes))
	}
	if hook.outcomes[0].Status != string(outcome.Status) {
		t.Errorf("WriteOutcome status = %q, want %q", hook.outcomes[0].Status, outcome.Status)
	}
	if hook.outcomes[0].IssueNumber != loopIssue().Number {
		t.Errorf("WriteOutcome issue_number = %d, want %d", hook.outcomes[0].IssueNumber, loopIssue().Number)
	}
}

func TestProcessIssue_HookOutcomeIncludesError(t *testing.T) {
	hook := &testRunDataHook{}
	// Force a failure by making the implementer agent fail.
	setupLoopTest(t, []string{}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		return []byte(""), nil
	})

	cfg := loopConfig()
	// Use a prompt that will cause the agent to fail by timing out.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately to force failure

	ProcessIssue(ctx, loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook, nil)

	if len(hook.outcomes) != 1 {
		t.Fatalf("WriteOutcome called %d times, want 1", len(hook.outcomes))
	}
	if hook.outcomes[0].Status != "failed" {
		t.Errorf("outcome status = %q, want %q", hook.outcomes[0].Status, "failed")
	}
	if hook.outcomes[0].Error == "" {
		t.Error("outcome error is empty, want failure reason persisted")
	}

	// Verify status.json is updated to reflect the final state.
	var foundFinalStatus bool
	for _, s := range hook.issueStatuses {
		if s.Status == "failed" {
			foundFinalStatus = true
			break
		}
	}
	if !foundFinalStatus {
		t.Error("status.json was not updated to 'failed'")
	}
}

func TestProcessIssue_NilHookSafe(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	// Should not panic with nil hook.
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)
	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
}

func TestProcessIssue_HookWritesImplementingAndInReviewStatuses(t *testing.T) {
	hook := &testRunDataHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook, nil)

	if len(hook.issueStatuses) < 2 {
		t.Fatalf("WriteIssueStatus called %d times, want at least 2", len(hook.issueStatuses))
	}

	// First status should be "implementing".
	if hook.issueStatuses[0].Status != "implementing" {
		t.Errorf("issueStatuses[0].Status = %q, want %q", hook.issueStatuses[0].Status, "implementing")
	}

	// One of the statuses should be "in_review".
	foundInReview := false
	for _, s := range hook.issueStatuses {
		if s.Status == "in_review" {
			foundInReview = true
			break
		}
	}
	if !foundInReview {
		t.Errorf("WriteIssueStatus(%q) not called; statuses: %v", "in_review", hook.issueStatuses)
	}
}

func testPromptsWithSpecGen(t *testing.T) *Prompts {
	t.Helper()
	p := testPrompts(t)
	p.SpecGenerator = "Generate spec for #{{.IssueNumber}}"
	return p
}

func TestProcessIssue_HookCalledOnSpecGeneratorSuccess(t *testing.T) {
	hook := &testRunDataHook{}
	// Agent outputs: spec generator (first call), implementer, quality reviewer, reviewer
	setupLoopTest(t, []string{
		"spec gen output",
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPromptsWithSpecGen(t), nil, testLogger(t), hook, nil)

	if hook.specGeneratorCalls != 1 {
		t.Errorf("WriteSpecGeneratorResult called %d times, want 1", hook.specGeneratorCalls)
	}
}

func TestProcessIssue_HookCalledOnSpecGeneratorError(t *testing.T) {
	hook := &testRunDataHook{}

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		if callIdx == 1 {
			// Spec generator call: return an error
			return nil, nil, 0, fmt.Errorf("spec gen failed")
		}
		// Subsequent calls: implementer, quality reviewer, reviewer
		outputs := []string{"implementer output", "AGENT_RESULT=APPROVED", "AGENT_RESULT=APPROVED"}
		idx := callIdx - 2
		if idx < len(outputs) {
			return []byte(wrapRunnerJSON(outputs[idx])), []byte(""), 0, nil
		}
		return []byte(wrapRunnerJSON("")), []byte(""), 0, nil
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPromptsWithSpecGen(t), nil, testLogger(t), hook, nil)

	if hook.specGeneratorCalls != 1 {
		t.Errorf("WriteSpecGeneratorResult called %d times, want 1", hook.specGeneratorCalls)
	}
}

func TestProcessIssue_HookCalledOnSpecGeneratorTimeout(t *testing.T) {
	hook := &testRunDataHook{}

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	callIdx := 0
	Runner = func(innerCtx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		if callIdx == 1 {
			// Cancel the context during spec gen to trigger TimedOut path
			cancel()
			return []byte(""), []byte(""), 0, nil
		}
		outputs := []string{"implementer output", "AGENT_RESULT=APPROVED", "AGENT_RESULT=APPROVED"}
		idx := callIdx - 2
		if idx < len(outputs) {
			return []byte(wrapRunnerJSON(outputs[idx])), []byte(""), 0, nil
		}
		return []byte(wrapRunnerJSON("")), []byte(""), 0, nil
	}

	ProcessIssue(ctx, loopIssue(), loopConfig(), testPromptsWithSpecGen(t), nil, testLogger(t), hook, nil)

	if hook.specGeneratorCalls != 1 {
		t.Errorf("WriteSpecGeneratorResult called %d times, want 1", hook.specGeneratorCalls)
	}
}

func TestProcessIssue_HookCalledOnVerifyPass(t *testing.T) {
	hook := &testRunDataHook{}
	cfg := verifyLoopConfig(true)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" {
			return []byte(""), nil // verify command passes
		}
		return []byte(""), nil
	})

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook, nil)

	if len(hook.verifyResults) != 1 {
		t.Fatalf("WriteVerifyResult called %d times, want 1", len(hook.verifyResults))
	}
	if hook.verifyResults[0].Attempt != 0 {
		t.Errorf("verifyResults[0].Attempt = %d, want 0", hook.verifyResults[0].Attempt)
	}
	if hook.verifyResults[0].FixAttempted {
		t.Errorf("verifyResults[0].FixAttempted = true, want false")
	}
}

func TestProcessIssue_HookCalledOnVerifyFixRetries(t *testing.T) {
	hook := &testRunDataHook{}
	cfg := verifyFixLoopConfig(true, 1)
	shCallIdx := 0

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
		"verify-fix output",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" {
			idx := shCallIdx
			shCallIdx++
			if idx == 0 {
				// First verify: fail.
				return []byte("error output"), fmt.Errorf("exit status 1")
			}
			// Second verify (after fix): pass.
			return []byte(""), nil
		}
		return []byte(""), nil
	})

	ProcessIssue(context.Background(), loopIssue(), cfg, verifyFixPrompts(t), nil, testLogger(t), hook, nil)

	// Expect two WriteVerifyResult calls: attempt 0 (fail) and attempt 1 (pass after fix).
	if len(hook.verifyResults) != 2 {
		t.Fatalf("WriteVerifyResult called %d times, want 2", len(hook.verifyResults))
	}
	if hook.verifyResults[0].Attempt != 0 {
		t.Errorf("verifyResults[0].Attempt = %d, want 0", hook.verifyResults[0].Attempt)
	}
	if hook.verifyResults[0].FixAttempted {
		t.Errorf("verifyResults[0].FixAttempted = true, want false")
	}
	if hook.verifyResults[1].Attempt != 1 {
		t.Errorf("verifyResults[1].Attempt = %d, want 1", hook.verifyResults[1].Attempt)
	}
	if !hook.verifyResults[1].FixAttempted {
		t.Errorf("verifyResults[1].FixAttempted = false, want true")
	}
}

// --- Quality flag tests ---

// captureRunDataHook captures review steps (including flags) for inspection.
type captureRunDataHook struct {
	testRunDataHook
	reviewSteps map[string]rundata.StepResult // keyed by "quality" or "functional"
}

func newCaptureHook() *captureRunDataHook {
	return &captureRunDataHook{reviewSteps: make(map[string]rundata.StepResult)}
}

func (h *captureRunDataHook) WriteReviewResult(issueNum int, kind string, step rundata.StepResult) error {
	h.reviewKinds = append(h.reviewKinds, kind)
	h.reviewSteps[kind] = step
	return nil
}

// bufferLogger returns a logger that captures all log records into buf.
func bufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestComputeReviewFlags_QualityReviewerExemptFromTestExecution(t *testing.T) {
	// A result with no tool trace entries — would normally trigger no_diff_read,
	// no_tests_run, no_review_tests_written, and no_review_tests_run for the functional reviewer.
	result := &Result{ToolTrace: nil}
	cfg := &config.Config{
		TestCommand: "go test ./...",
		ReviewDir:   "tests/review/",
	}

	// Quality reviewer (checkTestExecution=false): should NOT produce test-execution or test-run flags.
	qFlags := computeReviewFlags(result, cfg, false, false)
	for _, f := range qFlags {
		if f.Code == "no_review_tests_written" || f.Code == "no_review_tests_run" || f.Code == "no_tests_run" {
			t.Errorf("quality reviewer should be exempt from %q, got flag: %+v", f.Code, f)
		}
	}

	// Functional reviewer (checkTestExecution=true): SHOULD produce test-execution flags.
	fFlags := computeReviewFlags(result, cfg, true, true)
	var hasTestFlag bool
	for _, f := range fFlags {
		if f.Code == "no_review_tests_written" || f.Code == "no_review_tests_run" {
			hasTestFlag = true
			break
		}
	}
	if !hasTestFlag {
		t.Error("functional reviewer should produce test-execution flags when tests not found in trace")
	}
}

func TestComputeReviewFlags_CostFloorFlag(t *testing.T) {
	result := &Result{
		CostUSD:   0.001, // below threshold
		ToolTrace: []string{"Read file", "go test ./..."},
	}
	cfg := &config.Config{
		Quality: config.Quality{MinReviewCostUSD: 0.10},
	}

	flags := computeReviewFlags(result, cfg, false, false)

	var found bool
	for _, f := range flags {
		if f.Code == "low_cost" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected low_cost flag when cost $0.001 < threshold $0.10, got flags: %+v", flags)
	}
}

func TestComputeReviewFlags_DisabledWhenZeroThreshold(t *testing.T) {
	result := &Result{
		CostUSD:   0.0001,
		ToolTrace: nil,
	}
	cfg := &config.Config{
		// Zero thresholds = disabled
		Quality: config.Quality{MinReviewCostUSD: 0, MinReviewDurationSeconds: 0},
	}

	flags := computeReviewFlags(result, cfg, false, false)
	for _, f := range flags {
		if f.Code == "low_cost" || f.Code == "short_duration" {
			t.Errorf("cost/duration flags should be disabled when threshold is 0, got: %+v", f)
		}
	}
}

func TestProcessIssue_QualityFlagsLoggedAsWarnings(t *testing.T) {
	// Set a high cost threshold so the mock agent's $0 cost triggers a low_cost flag.
	cfg := loopConfig()
	cfg.Quality.MinReviewCostUSD = 0.10

	var logBuf bytes.Buffer
	logger := bufferLogger(&logBuf)

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, logger, nil, nil)
	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "quality flag detected") {
		t.Errorf("expected 'quality flag detected' warning in log output, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "low_cost") {
		t.Errorf("expected 'low_cost' code in log output, got:\n%s", logOutput)
	}
}

func TestProcessIssue_FlagsIncludedInHookStepResult(t *testing.T) {
	// Set a high cost threshold so flags are generated.
	cfg := loopConfig()
	cfg.Quality.MinReviewCostUSD = 0.10

	hook := newCaptureHook()

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook, nil)
	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}

	qStep, ok := hook.reviewSteps["quality"]
	if !ok {
		t.Fatal("quality review step not found in hook")
	}
	var hasLowCost bool
	for _, f := range qStep.Flags {
		if f.Code == "low_cost" {
			hasLowCost = true
			break
		}
	}
	if !hasLowCost {
		t.Errorf("expected low_cost flag in quality review step, got flags: %+v", qStep.Flags)
	}
}

func TestProcessIssue_QualityReviewerExemptInLoop(t *testing.T) {
	// Set a non-empty TestCommand and ReviewDir so CheckReviewTestExecution would
	// produce flags — but only for the functional reviewer, not the quality reviewer.
	// Create a scenario dir with a spec for issue #5 so hasScenarioSpec=true and
	// test-execution flags are expected from the functional reviewer.
	scenarioDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scenarioDir, "spec.md"), []byte("Relates to: Issue #5\n"), 0600); err != nil {
		t.Fatalf("failed to write scenario spec: %v", err)
	}

	cfg := loopConfig()
	cfg.TestCommand = "go test ./..."
	cfg.ReviewDir = "tests/review/"
	cfg.ScenarioDir = scenarioDir

	hook := newCaptureHook()

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook, nil)

	qStep := hook.reviewSteps["quality"]
	for _, f := range qStep.Flags {
		if f.Code == "no_review_tests_written" || f.Code == "no_review_tests_run" {
			t.Errorf("quality reviewer should be exempt from test-execution flags, got: %+v", f)
		}
	}

	fStep := hook.reviewSteps["functional"]
	var hasTestFlag bool
	for _, f := range fStep.Flags {
		if f.Code == "no_review_tests_written" || f.Code == "no_review_tests_run" {
			hasTestFlag = true
			break
		}
	}
	if !hasTestFlag {
		t.Errorf("functional reviewer should have test-execution flags, got flags: %+v", fStep.Flags)
	}
}

// --- Pre-merge guard tests ---

func TestHasQualityFlag_Found(t *testing.T) {
	flags := []quality.Flag{
		{Code: "low_cost", Message: "too cheap"},
		{Code: "no_review_tests_written", Message: "no tests"},
	}
	if !hasQualityFlag(flags, "no_review_tests_written") {
		t.Error("hasQualityFlag should return true when flag is present")
	}
}

func TestHasQualityFlag_NotFound(t *testing.T) {
	flags := []quality.Flag{
		{Code: "low_cost", Message: "too cheap"},
	}
	if hasQualityFlag(flags, "no_review_tests_written") {
		t.Error("hasQualityFlag should return false when flag is absent")
	}
}

func TestHasQualityFlag_Empty(t *testing.T) {
	if hasQualityFlag(nil, "no_review_tests_written") {
		t.Error("hasQualityFlag should return false for empty slice")
	}
}

// TestProcessIssue_PreMergeGuardRerunsReviewer verifies that when the
// functional reviewer approves a PR but the no_review_tests_written flag is detected,
// the reviewer is re-run (not the implementer) and no merge happens until tests are written.
func TestProcessIssue_PreMergeGuardRerunsReviewer(t *testing.T) {
	// Create a scenario dir with a spec for issue #5 so hasScenarioSpec=true.
	scenarioDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scenarioDir, "spec.md"), []byte("Relates to: Issue #5\n"), 0600); err != nil {
		t.Fatalf("failed to write scenario spec: %v", err)
	}

	cfg := loopConfig()
	cfg.MaxRetries = 1
	cfg.TestCommand = "go test ./..."
	cfg.ReviewDir = "tests/review/"
	cfg.ScenarioDir = scenarioDir

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var mergeCallCount int

	reviewerWithoutTests := func() []byte {
		final := runnerFinalResult{
			Result:    "AGENT_RESULT=APPROVED",
			ToolTrace: []string{"Read tests/existing_test.go", "go test ./..."},
		}
		b, _ := json.Marshal(final)
		return b
	}

	reviewerWithTests := func() []byte {
		final := runnerFinalResult{
			Result:    "AGENT_RESULT=APPROVED",
			ToolTrace: []string{"Write tests/review/my_test.go", "go test ./..."},
		}
		b, _ := json.Marshal(final)
		return b
	}

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer — approve
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		case 3: // functional reviewer — approves without writing tests (guard re-runs reviewer)
			return reviewerWithoutTests(), []byte(""), 0, nil
		default: // second functional reviewer — approves with tests written (no implementer retry in between)
			return reviewerWithTests(), []byte(""), 0, nil
		}
	}

	guardFn := func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "gh" && strings.Contains(joined, "pr merge") {
			mergeCallCount++
		}
		return loopGuardFn(name, args...)
	}
	GuardRunner = guardFn

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	// 4 agent calls: implementer, quality reviewer, reviewer (no tests), reviewer (with tests).
	// No implementer retry — the guard re-runs the reviewer directly.
	if callIdx != 4 {
		t.Errorf("expected exactly 4 agent calls (no implementer retry), got %d", callIdx)
	}
	if mergeCallCount != 1 {
		t.Errorf("gh pr merge should be called once at the end, got %d calls", mergeCallCount)
	}
}

// TestProcessIssue_SpecGeneratedNoFetchNeeded verifies that after
// GenerateSpec succeeds, no git fetch or checkout is performed — the
// specGenerated flag is used instead of pulling files to the host.
func TestProcessIssue_SpecGeneratedNoFetchNeeded(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	var guardCalls [][]string
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		call := append([]string{name}, args...)
		guardCalls = append(guardCalls, call)
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // spec generator
			return []byte(wrapRunnerJSON("spec generated")), []byte(""), 0, nil
		case 2: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer — include Write to review dir
			return []byte(wrapRunnerJSONWithTrace("AGENT_RESULT=APPROVED", []string{"Write tests/review/test_main.go", "Bash go test ./tests/review/ -v"})), []byte(""), 0, nil
		}
	}

	prompts := testPrompts(t)
	prompts.SpecGenerator = "Generate spec for #{{.IssueNumber}}"

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}

	// Verify no git fetch or checkout was called — specGenerated flag handles it.
	for _, call := range guardCalls {
		if len(call) >= 2 && call[0] == "git" && call[1] == "fetch" {
			t.Errorf("unexpected git fetch call: %v", call)
		}
		if len(call) >= 2 && call[0] == "git" && call[1] == "checkout" {
			t.Errorf("unexpected git checkout call: %v", call)
		}
	}
}

// TestProcessIssue_SpecGeneratedSetsHasSpec verifies that when the spec
// generator succeeds, hasSpec is true for quality checks even though
// the spec file only exists on the remote branch (not the host).
func TestProcessIssue_SpecGeneratedSetsHasSpec(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // spec generator
			return []byte(wrapRunnerJSON("spec generated")), []byte(""), 0, nil
		case 2: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer — must include Write to review dir
			return []byte(wrapRunnerJSONWithTrace("AGENT_RESULT=APPROVED", []string{"Write tests/review/test_main.go", "Bash go test ./tests/review/ -v"})), []byte(""), 0, nil
		}
	}

	prompts := testPrompts(t)
	prompts.SpecGenerator = "Generate spec for #{{.IssueNumber}}"

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	// Spec generator success sets hasSpec=true; functional reviewer wrote tests, so it passes.
	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
}

// TestProcessIssue_PreMergeGuardAllowsApprovalWithTests verifies that when the
// functional reviewer approves a PR and writes tests, the guard does not interfere.
func TestProcessIssue_PreMergeGuardAllowsApprovalWithTests(t *testing.T) {
	scenarioDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scenarioDir, "spec.md"), []byte("Relates to: Issue #5\n"), 0600); err != nil {
		t.Fatalf("failed to write scenario spec: %v", err)
	}

	cfg := loopConfig()
	cfg.TestCommand = "go test ./..."
	cfg.ReviewDir = "tests/review/"
	cfg.ScenarioDir = scenarioDir

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer — approve
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer — approves WITH tests written
			final := runnerFinalResult{
				Result:    "AGENT_RESULT=APPROVED",
				ToolTrace: []string{"Write tests/review/my_test.go", "go test ./..."},
			}
			b, _ := json.Marshal(final)
			return b, []byte(""), 0, nil
		}
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	// Should be exactly 3 agent calls: implement + quality + functional (no retries).
	if callIdx != 3 {
		t.Errorf("expected 3 agent calls (implement + quality + review), got %d", callIdx)
	}
}

// captureRetryFunctionalHook extends testRunDataHook to record which attempts
// triggered WriteRetryFunctionalReviewResult vs WriteReviewResult("functional").
type captureRetryFunctionalHook struct {
	testRunDataHook
	retryFunctionalAttempts []int    // attempt indices passed to WriteRetryFunctionalReviewResult
	topLevelFunctionalKinds []string // kinds passed to WriteReviewResult
}

func (h *captureRetryFunctionalHook) WriteReviewResult(_ int, kind string, _ rundata.StepResult) error {
	h.reviewKinds = append(h.reviewKinds, kind)
	h.topLevelFunctionalKinds = append(h.topLevelFunctionalKinds, kind)
	return nil
}

func (h *captureRetryFunctionalHook) WriteRetryFunctionalReviewResult(_ int, attempt int, _ rundata.StepResult) error {
	h.retryFunctionalReviewCalls++
	h.retryFunctionalAttempts = append(h.retryFunctionalAttempts, attempt)
	return nil
}

// TestProcessIssue_FunctionalReviewOnRetry_WritesToRetryDir verifies that when
// the functional reviewer returns CHANGES_REQUESTED and a retry is available,
// the pre-retry review is written to the retry dir (not top-level).
func TestProcessIssue_FunctionalReviewOnRetry_WritesToRetryDir(t *testing.T) {
	hook := &captureRetryFunctionalHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	// Pre-retry functional review should go to retry dir (attempt 0).
	if hook.retryFunctionalReviewCalls != 1 {
		t.Errorf("WriteRetryFunctionalReviewResult called %d times, want 1", hook.retryFunctionalReviewCalls)
	}
	if len(hook.retryFunctionalAttempts) == 1 && hook.retryFunctionalAttempts[0] != 0 {
		t.Errorf("WriteRetryFunctionalReviewResult attempt = %d, want 0", hook.retryFunctionalAttempts[0])
	}
	// Final (approved) functional review should go to top-level.
	functionalTopLevel := 0
	for _, k := range hook.topLevelFunctionalKinds {
		if k == "functional" {
			functionalTopLevel++
		}
	}
	if functionalTopLevel != 1 {
		t.Errorf("WriteReviewResult(\"functional\") top-level called %d times, want 1; kinds: %v", functionalTopLevel, hook.topLevelFunctionalKinds)
	}
}

// TestProcessIssue_FunctionalReviewExhausted_WritesToTopLevel verifies that when
// retries are exhausted, the final CHANGES_REQUESTED review goes to top-level.
func TestProcessIssue_FunctionalReviewExhausted_WritesToTopLevel(t *testing.T) {
	cfg := loopConfig()
	cfg.MaxRetries = 1

	hook := &captureRetryFunctionalHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
		"AGENT_RESULT=CHANGES_REQUESTED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook, nil)

	if outcome.Status != "needs-human-review" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "needs-human-review", outcome.Err)
	}
	// First CHANGES_REQUESTED goes to retry dir (attempt 0 — retry available).
	if hook.retryFunctionalReviewCalls != 1 {
		t.Errorf("WriteRetryFunctionalReviewResult called %d times, want 1", hook.retryFunctionalReviewCalls)
	}
	// Final CHANGES_REQUESTED (retries exhausted) goes to top-level.
	functionalTopLevel := 0
	for _, k := range hook.topLevelFunctionalKinds {
		if k == "functional" {
			functionalTopLevel++
		}
	}
	if functionalTopLevel != 1 {
		t.Errorf("WriteReviewResult(\"functional\") top-level called %d times, want 1 (for exhausted retry); kinds: %v", functionalTopLevel, hook.topLevelFunctionalKinds)
	}
}

// TestProcessIssue_FunctionalReviewApproved_WritesToTopLevel verifies that when
// the functional reviewer approves on the first attempt, the result goes to top-level
// and WriteRetryFunctionalReviewResult is never called.
func TestProcessIssue_FunctionalReviewApproved_WritesToTopLevel(t *testing.T) {
	hook := &captureRetryFunctionalHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if hook.retryFunctionalReviewCalls != 0 {
		t.Errorf("WriteRetryFunctionalReviewResult called %d times, want 0", hook.retryFunctionalReviewCalls)
	}
	functionalTopLevel := 0
	for _, k := range hook.topLevelFunctionalKinds {
		if k == "functional" {
			functionalTopLevel++
		}
	}
	if functionalTopLevel != 1 {
		t.Errorf("WriteReviewResult(\"functional\") called %d times, want 1; kinds: %v", functionalTopLevel, hook.topLevelFunctionalKinds)
	}
}

// verifyLoopConfig returns a loop config with a build command and the given blocking setting.
func verifyLoopConfig(blocking bool) *config.Config {
	cfg := loopConfig()
	cfg.BuildCommand = "go build ./..."
	cfg.Verify.Blocking = blocking
	return cfg
}

// TestBuildVerifyChecks_AllCommands verifies that all four non-empty commands produce checks
// in generate → build → lint → test order.
func TestBuildVerifyChecks_AllCommands(t *testing.T) {
	cfg := &config.Config{
		GenerateCommand: "go generate ./...",
		BuildCommand:    "go build ./...",
		LintCommand:     "golangci-lint run",
		TestCommand:     "go test ./...",
	}
	checks := buildVerifyChecks(cfg)
	if len(checks) != 4 {
		t.Fatalf("len = %d, want 4", len(checks))
	}
	if checks[0].Name != "generate" || checks[0].Command != "go generate ./..." {
		t.Errorf("checks[0] = %+v, want {generate go generate ./...}", checks[0])
	}
	if checks[1].Name != "build" || checks[1].Command != "go build ./..." {
		t.Errorf("checks[1] = %+v, want {build go build ./...}", checks[1])
	}
	if checks[2].Name != "lint" || checks[2].Command != "golangci-lint run" {
		t.Errorf("checks[2] = %+v, want {lint golangci-lint run}", checks[2])
	}
	if checks[3].Name != "test" || checks[3].Command != "go test ./..." {
		t.Errorf("checks[3] = %+v, want {test go test ./...}", checks[3])
	}
}

// TestBuildVerifyChecks_SkipsEmpty verifies that empty commands are omitted.
func TestBuildVerifyChecks_SkipsEmpty(t *testing.T) {
	cfg := &config.Config{
		BuildCommand: "go build ./...",
		LintCommand:  "",
		TestCommand:  "go test ./...",
	}
	checks := buildVerifyChecks(cfg)
	if len(checks) != 2 {
		t.Fatalf("len = %d, want 2 (lint skipped)", len(checks))
	}
	if checks[0].Name != "build" {
		t.Errorf("checks[0].Name = %q, want build", checks[0].Name)
	}
	if checks[1].Name != "test" {
		t.Errorf("checks[1].Name = %q, want test", checks[1].Name)
	}
}

// TestBuildVerifyChecks_NoneConfigured verifies that an empty config produces no checks.
func TestBuildVerifyChecks_NoneConfigured(t *testing.T) {
	checks := buildVerifyChecks(&config.Config{})
	if len(checks) != 0 {
		t.Fatalf("len = %d, want 0", len(checks))
	}
}

// TestBuildVerifyChecks_GenerateRunsFirst verifies generate is first when all commands are set.
func TestBuildVerifyChecks_GenerateRunsFirst(t *testing.T) {
	cfg := &config.Config{
		GenerateCommand: "go generate ./...",
		BuildCommand:    "go build ./...",
		LintCommand:     "golangci-lint run",
		TestCommand:     "go test ./...",
	}
	checks := buildVerifyChecks(cfg)
	if len(checks) != 4 {
		t.Fatalf("len = %d, want 4", len(checks))
	}
	if checks[0].Name != "generate" {
		t.Errorf("checks[0].Name = %q, want generate", checks[0].Name)
	}
	if checks[1].Name != "build" {
		t.Errorf("checks[1].Name = %q, want build", checks[1].Name)
	}
}

// TestBuildVerifyChecks_GenerateSkippedWhenEmpty verifies that empty generate_command
// produces no generate entry in the check list.
func TestBuildVerifyChecks_GenerateSkippedWhenEmpty(t *testing.T) {
	cfg := &config.Config{
		GenerateCommand: "",
		BuildCommand:    "go build ./...",
		TestCommand:     "go test ./...",
	}
	checks := buildVerifyChecks(cfg)
	if len(checks) != 2 {
		t.Fatalf("len = %d, want 2", len(checks))
	}
	if checks[0].Name != "build" {
		t.Errorf("checks[0].Name = %q, want build", checks[0].Name)
	}
	if checks[1].Name != "test" {
		t.Errorf("checks[1].Name = %q, want test", checks[1].Name)
	}
}

// TestBuildVerifyChecks_GenerateFailureStopsPipeline verifies that a failing generate
// step prevents build from running (stop-on-failure via RunVerify).
func TestBuildVerifyChecks_GenerateFailureStopsPipeline(t *testing.T) {
	checks := []Check{
		{Name: "generate", Command: "generate-cmd"},
		{Name: "build", Command: "build-cmd"},
	}

	callLog := []string{}
	runner := func(_ context.Context, command string) ([]byte, []byte, int, error) {
		callLog = append(callLog, command)
		if command == "generate-cmd" {
			return []byte("generate error"), nil, 1, nil
		}
		return []byte(""), nil, 0, nil
	}

	result := RunVerify(context.Background(), checks, runner, config.TruncationLimits{VerifyOutput: 4096})

	if result.AllPassed {
		t.Error("AllPassed = true, want false (generate failed)")
	}
	if len(callLog) != 1 || callLog[0] != "generate-cmd" {
		t.Errorf("callLog = %v, want [generate-cmd] (build must not run after generate fails)", callLog)
	}
	if len(result.Checks) != 1 {
		t.Fatalf("len(result.Checks) = %d, want 1", len(result.Checks))
	}
	if result.Checks[0].Name != "generate" || result.Checks[0].Passed {
		t.Errorf("result.Checks[0] = %+v, want {generate, passed=false}", result.Checks[0])
	}
}

// TestBuildVerifyChecks_GenerateSuccessProceedsToBuild verifies that a passing generate
// step allows build to run next.
func TestBuildVerifyChecks_GenerateSuccessProceedsToBuild(t *testing.T) {
	checks := []Check{
		{Name: "generate", Command: "generate-cmd"},
		{Name: "build", Command: "build-cmd"},
	}

	callLog := []string{}
	runner := func(_ context.Context, command string) ([]byte, []byte, int, error) {
		callLog = append(callLog, command)
		return []byte(""), nil, 0, nil
	}

	result := RunVerify(context.Background(), checks, runner, config.TruncationLimits{VerifyOutput: 4096})

	if !result.AllPassed {
		t.Error("AllPassed = false, want true (both generate and build pass)")
	}
	if len(callLog) != 2 || callLog[0] != "generate-cmd" || callLog[1] != "build-cmd" {
		t.Errorf("callLog = %v, want [generate-cmd build-cmd]", callLog)
	}
}

// TestNewHostRunner_SuccessOnZeroExit verifies that a nil-error GuardRunner response maps to exit 0.
func TestNewHostRunner_SuccessOnZeroExit(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("build output"), nil
	})

	runner := newHostRunner()
	stdout, _, exitCode, err := runner(context.Background(), "go build ./...")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
	if string(stdout) != "build output" {
		t.Errorf("stdout = %q, want %q", string(stdout), "build output")
	}
}

// TestNewHostRunner_FailsOnError verifies that a non-nil GuardRunner error maps to non-zero exit.
func TestNewHostRunner_FailsOnError(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("build error"), fmt.Errorf("exit status 1")
	})

	runner := newHostRunner()
	_, _, exitCode, _ := runner(context.Background(), "go build ./...")

	if exitCode == 0 {
		t.Error("exitCode = 0, want non-zero on error")
	}
}

// TestNewHostRunner_UsesShC verifies the runner invokes GuardRunner with sh -c.
func TestNewHostRunner_UsesShC(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		capturedName = name
		capturedArgs = args
		return []byte(""), nil
	})

	runner := newHostRunner()
	runner(context.Background(), "go build ./...") //nolint

	if capturedName != "sh" {
		t.Errorf("name = %q, want sh", capturedName)
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "-c" || capturedArgs[1] != "go build ./..." {
		t.Errorf("args = %v, want [-c go build ./...]", capturedArgs)
	}
}

// TestProcessIssue_VerifyPassedProceedsToReview verifies that when verify passes
// the issue proceeds to review and is ultimately implemented.
func TestProcessIssue_VerifyPassedProceedsToReview(t *testing.T) {
	stubs := setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil // sh -c and everything else succeeds
	})
	_ = stubs

	outcome := ProcessIssue(context.Background(), loopIssue(), verifyLoopConfig(true), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
}

// TestProcessIssue_VerifySkippedWhenNoCommands verifies that when no commands are
// configured the verify step is skipped entirely (no sh calls made).
func TestProcessIssue_VerifySkippedWhenNoCommands(t *testing.T) {
	stubs := setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
	for _, call := range stubs.guardCalls {
		if len(call) > 0 && call[0] == "sh" {
			t.Errorf("expected no sh calls when no verify commands configured, got: %v", call)
		}
	}
}

// TestProcessIssue_VerifyHostMode verifies that verify commands are run on the host
// via sh -c.
func TestProcessIssue_VerifyHostMode(t *testing.T) {
	var verifyCmds []string

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 1 && args[0] == "-c" {
			verifyCmds = append(verifyCmds, args[1])
			return []byte("ok"), nil
		}
		return []byte(""), nil
	})

	cfg := verifyLoopConfig(true)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
	if len(verifyCmds) == 0 {
		t.Error("expected verify commands to be run via sh -c")
	}
	if len(verifyCmds) > 0 && verifyCmds[0] != "go build ./..." {
		t.Errorf("verifyCmds[0] = %q, want %q", verifyCmds[0], "go build ./...")
	}
}

// TestProcessIssue_VerifyBlockingFailure verifies that when verify fails and
// Blocking is true, the issue status is "failed".
func TestProcessIssue_VerifyBlockingFailure(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			return []byte("build failed"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	})

	cfg := verifyLoopConfig(true) // Blocking = true
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed", outcome.Status)
	}
	if outcome.Err == nil || !strings.Contains(outcome.Err.Error(), "verify") {
		t.Errorf("Err = %v, want error containing 'verify'", outcome.Err)
	}
}

// TestProcessIssue_VerifyNonBlockingProceedsToReview verifies that when verify
// fails and Blocking is false, the issue proceeds to review with a warning.
func TestProcessIssue_VerifyNonBlockingProceedsToReview(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			return []byte("build failed"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	})

	cfg := verifyLoopConfig(false) // Blocking = false → proceeds to review despite failure
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
}

// TestProcessIssue_VerifyRunsAfterGuardRails verifies that guard rail drift detection
// occurs before verify — if drift is detected, verify (sh -c) is never called.
func TestProcessIssue_VerifyRunsAfterGuardRails(t *testing.T) {
	stubs := setupLoopTest(t, []string{
		"implementer output",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			// Protected drift — guard rails should fail before verify runs.
			return []byte("CLAUDE.md\n"), nil
		}
		return []byte(""), nil
	})

	cfg := verifyLoopConfig(true)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed (guard rails drift)", outcome.Status)
	}
	for _, call := range stubs.guardCalls {
		if len(call) > 0 && call[0] == "sh" {
			t.Errorf("expected verify (sh -c) not to run after guard rail failure, but got: %v", call)
		}
	}
}

// TestProcessIssue_VerifyRunsBeforeQualityReview verifies that a blocking verify
// failure prevents quality review from running (only 1 agent call: implementer).
func TestProcessIssue_VerifyRunsBeforeQualityReview(t *testing.T) {
	stubs := setupLoopTest(t, []string{
		"implementer output",
		// No quality review output — should never be reached.
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			return []byte("build failed"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	})

	cfg := verifyLoopConfig(true) // Blocking = true → fails at verify
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed", outcome.Status)
	}
	// Only 1 agent call (implementer) — quality review never reached.
	if stubs.agentCalls != 1 {
		t.Errorf("agentCalls = %d, want 1 (quality review should not run when verify blocks)", stubs.agentCalls)
	}
}

// verifyFixLoopConfig returns a loop config with a build command, blocking setting,
// and MaxFixAttempts configured for verify-fix cycle tests.
func verifyFixLoopConfig(blocking bool, maxFixAttempts int) *config.Config {
	cfg := verifyLoopConfig(blocking)
	cfg.Verify.MaxFixAttempts = maxFixAttempts
	return cfg
}

// verifyFixPrompts returns test prompts with a VerifyFix template.
func verifyFixPrompts(t *testing.T) *Prompts {
	t.Helper()
	p := testPrompts(t)
	p.VerifyFix = "Fix PR #{{.PRNumber}} issue #{{.IssueNumber}} errors: {{.VerifyErrors}}"
	return p
}

// TestProcessIssue_VerifyFixSucceedsOnFirstAttempt verifies that when verify
// initially fails but the fix agent succeeds, processing continues to review.
func TestProcessIssue_VerifyFixSucceedsOnFirstAttempt(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	shCallIdx := 0
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			shCallIdx++
			if shCallIdx == 1 {
				// First verify run: fail.
				return []byte("build error output"), fmt.Errorf("exit status 1")
			}
			// Second verify run (after fix): pass.
			return []byte(""), nil
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			out := `{"session_id":"sess-impl","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // verify-fix agent
			out := `{"session_id":"sess-fix","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	cfg := verifyFixLoopConfig(true, 1)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, verifyFixPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
	if shCallIdx != 2 {
		t.Errorf("shCallIdx = %d, want 2 (initial verify + re-verify after fix)", shCallIdx)
	}
}

// TestProcessIssue_VerifyFixExhaustedBlocking verifies that when all fix attempts
// are exhausted and verify still fails with Blocking=true, status is "failed".
func TestProcessIssue_VerifyFixExhaustedBlocking(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			// All verify runs fail.
			return []byte("build error"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		// All agent calls return OK — verify failures drive the outcome.
		out := `{"session_id":"sess-fix","result":"ok","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	}

	// MaxFixAttempts=2 → 2 fix agents called, then blocked.
	cfg := verifyFixLoopConfig(true, 2)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, verifyFixPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed after exhausted blocking fix attempts", outcome.Status)
	}
	if outcome.Err == nil || !strings.Contains(outcome.Err.Error(), "verify") {
		t.Errorf("Err = %v, want error containing 'verify'", outcome.Err)
	}
	// Agent calls: 1 (implementer) + 2 (fix agents) = 3
	if callIdx != 3 {
		t.Errorf("callIdx = %d, want 3 (implementer + 2 fix agents)", callIdx)
	}
}

// TestProcessIssue_VerifyFixExhaustedNonBlocking verifies that when all fix attempts
// are exhausted with Blocking=false, processing proceeds to review with a warning.
func TestProcessIssue_VerifyFixExhaustedNonBlocking(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			// All verify runs fail.
			return []byte("build error"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			out := `{"session_id":"sess-impl","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // verify-fix agent
			out := `{"session_id":"sess-fix","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 3: // quality reviewer (non-blocking proceeds here)
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	// MaxFixAttempts=1, Blocking=false → 1 fix agent, then proceed to review.
	cfg := verifyFixLoopConfig(false, 1)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, verifyFixPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (non-blocking proceeds to review)", outcome.Status)
	}
}

// TestProcessIssue_VerifyFixDriftCheckedAfterFix verifies that protected path drift
// is re-checked after each fix attempt, and a drift detection fails the issue.
func TestProcessIssue_VerifyFixDriftCheckedAfterFix(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	shCallIdx := 0
	driftCheckCount := 0
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			driftCheckCount++
			if driftCheckCount == 2 {
				// Drift detected after the fix attempt.
				return []byte("CLAUDE.md\n"), nil
			}
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			shCallIdx++
			// First verify fails so fix cycle triggers.
			return []byte("build error"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		out := `{"session_id":"sess-fix","result":"ok","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	}

	cfg := verifyFixLoopConfig(true, 2)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, verifyFixPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed (drift detected after fix)", outcome.Status)
	}
	if !strings.Contains(outcome.Err.Error(), "protected path drift") {
		t.Errorf("Err = %v, want 'protected path drift'", outcome.Err)
	}
	if driftCheckCount < 2 {
		t.Errorf("driftCheckCount = %d, want >= 2 (drift re-checked after fix)", driftCheckCount)
	}
}

// TestProcessIssue_VerifyFixSessionContinuity verifies that the fix agent receives
// the implementer's session ID for context resumption.
func TestProcessIssue_VerifyFixSessionContinuity(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	shCallIdx := 0
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			shCallIdx++
			if shCallIdx == 1 {
				return []byte("build error"), fmt.Errorf("exit status 1")
			}
			return []byte(""), nil
		}
		return []byte(""), nil
	}

	callIdx := 0
	var fixAgentEnv map[string]string
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer — returns a session ID
			out := `{"session_id":"sess-impl-xyz","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // verify-fix agent — capture env
			fixAgentEnv = make(map[string]string, len(env))
			for k, v := range env {
				fixAgentEnv[k] = v
			}
			out := `{"session_id":"sess-fix","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	cfg := verifyFixLoopConfig(true, 1)
	ProcessIssue(context.Background(), loopIssue(), cfg, verifyFixPrompts(t), nil, testLogger(t), nil, nil)

	if fixAgentEnv == nil {
		t.Fatal("verify-fix agent was never called")
	}
	if fixAgentEnv["GODARK_SESSION_ID"] != "sess-impl-xyz" {
		t.Errorf("GODARK_SESSION_ID = %q, want sess-impl-xyz", fixAgentEnv["GODARK_SESSION_ID"])
	}
}

// TestProcessIssue_VerifyFixPromptContainsErrorOutput verifies that the rendered
// verify_fix prompt includes check names and output from failed verify checks.
func TestProcessIssue_VerifyFixPromptContainsErrorOutput(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	shCallIdx := 0
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			shCallIdx++
			if shCallIdx == 1 {
				return []byte("undefined: Foo"), fmt.Errorf("exit status 1")
			}
			return []byte(""), nil
		}
		return []byte(""), nil
	}

	callIdx := 0
	var fixAgentPrompt string
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			out := `{"session_id":"sess-impl","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // verify-fix agent — capture prompt
			fixAgentPrompt = env["GODARK_PROMPT"]
			out := `{"session_id":"sess-fix","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	cfg := verifyFixLoopConfig(true, 1)
	ProcessIssue(context.Background(), loopIssue(), cfg, verifyFixPrompts(t), nil, testLogger(t), nil, nil)

	if fixAgentPrompt == "" {
		t.Fatal("verify-fix agent was never called")
	}
	if !strings.Contains(fixAgentPrompt, "build") {
		t.Errorf("fix prompt missing check name 'build', got: %s", fixAgentPrompt)
	}
	if !strings.Contains(fixAgentPrompt, "undefined: Foo") {
		t.Errorf("fix prompt missing check output 'undefined: Foo', got: %s", fixAgentPrompt)
	}
}

// TestProcessIssue_VerifyFixSkippedWhenNoPrompt verifies that when VerifyFix prompt
// is not configured, verify failures use the original blocking/non-blocking behavior.
func TestProcessIssue_VerifyFixSkippedWhenNoPrompt(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			return []byte("build error"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		out := `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	}

	// No VerifyFix prompt → fix cycle should not trigger.
	prompts := testPrompts(t) // VerifyFix is empty
	cfg := verifyFixLoopConfig(true, 2)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, prompts, nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed (no fix prompt, blocking verify failure)", outcome.Status)
	}
	// Only 1 agent call — fix cycle never triggered.
	if callIdx != 1 {
		t.Errorf("callIdx = %d, want 1 (no fix agents when VerifyFix prompt is empty)", callIdx)
	}
}

// sandboxLoopConfig returns a loop config with NoSandbox=false and a Docker image set.
func sandboxLoopConfig() *config.Config {
	cfg := verifyLoopConfig(true)
	cfg.NoSandbox = false
	cfg.Docker.Image = "test-image:latest"
	return cfg
}

// sandboxGuardFn returns a GuardRunner stub suitable for sandbox loop tests.
// It handles git rev-parse, gh pr view, and git diff.
func sandboxGuardFn(name string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
		return []byte("abc123\n"), nil
	}
	if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
		return []byte(`{"number": 10}`), nil
	}
	if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
		return []byte(`{"body": "Closes #5"}`), nil
	}
	if name == "git" && strings.Contains(joined, "diff --name-only") {
		return []byte("src/main.go\n"), nil
	}
	return []byte(""), nil
}

// TestProcessIssue_VerifySandboxMode verifies that when NoSandbox is false the
// sandboxCommandRunner is used (sandboxRunContainer is called instead of GuardRunner sh).
func TestProcessIssue_VerifySandboxMode(t *testing.T) {
	var sandboxCalled bool
	var capturedImage string
	var capturedEnv map[string]string

	origSandboxRunContainer := sandboxRunContainer
	t.Cleanup(func() { sandboxRunContainer = origSandboxRunContainer })
	sandboxRunContainer = func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		sandboxCalled = true
		capturedImage = opts.Image
		capturedEnv = opts.Env
		return &sandbox.RunResult{ExitCode: 0, Stdout: "build ok"}, nil
	}

	// Agent runs go through Docker in sandbox mode; stub Docker infrastructure.
	setupSandboxDockerStubs(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	})

	// git/gh commands still run on the host.
	stubGuardRunner(t, sandboxGuardFn)

	cfg := sandboxLoopConfig()
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
	if !sandboxCalled {
		t.Error("sandboxRunContainer was not called; expected sandbox verify runner to be used")
	}
	if capturedImage != "test-image:latest" {
		t.Errorf("Image = %q, want %q", capturedImage, "test-image:latest")
	}
	// Repo should be passed via environment variable, not embedded in the script.
	if capturedEnv["GODARK_REPO"] != "owner/repo" {
		t.Errorf("GODARK_REPO = %q, want %q", capturedEnv["GODARK_REPO"], "owner/repo")
	}
}

// TestProcessIssue_VerifyHostModeUnchangedWithNoSandbox verifies that NoSandbox=true
// still routes through the host runner (GuardRunner sh -c) and not the sandbox runner.
func TestProcessIssue_VerifyHostModeUnchangedWithNoSandbox(t *testing.T) {
	var sandboxCalled bool
	origSandboxRunContainer := sandboxRunContainer
	t.Cleanup(func() { sandboxRunContainer = origSandboxRunContainer })
	sandboxRunContainer = func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		sandboxCalled = true
		return &sandbox.RunResult{ExitCode: 0}, nil
	}

	var shCalled bool
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			shCalled = true
			return []byte("ok"), nil
		}
		return sandboxGuardFn(name, args...)
	})

	cfg := verifyLoopConfig(true) // NoSandbox: true (default from loopConfig)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
	if sandboxCalled {
		t.Error("sandboxRunContainer was called; host mode should not use the sandbox runner")
	}
	if !shCalled {
		t.Error("sh -c was not called; host mode should run verify commands via sh")
	}
}

// TestProcessIssue_VerifySandboxContainerFailure verifies that a non-zero exit code
// from the sandbox container causes the verify check to fail.
func TestProcessIssue_VerifySandboxContainerFailure(t *testing.T) {
	origSandboxRunContainer := sandboxRunContainer
	t.Cleanup(func() { sandboxRunContainer = origSandboxRunContainer })
	sandboxRunContainer = func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{ExitCode: 1, Stderr: "build failed in container"}, nil
	}

	// Agent runs go through Docker in sandbox mode.
	setupSandboxDockerStubs(t, []string{
		"implementer output",
	})
	stubGuardRunner(t, sandboxGuardFn)

	cfg := sandboxLoopConfig() // NoSandbox: false, Blocking: true
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed when container returns non-zero exit", outcome.Status)
	}
	if outcome.Err == nil || !strings.Contains(outcome.Err.Error(), "verify") {
		t.Errorf("Err = %v, want error containing 'verify'", outcome.Err)
	}
}

// --- Session ID tests for quality-review retries ---

// TestProcessIssue_QualityRetryHasNoSessionID verifies that when the quality
// reviewer requests changes, Retry is called with an empty prevSessionID
// (GODARK_SESSION_ID must not be set in the retry agent's environment).
func TestProcessIssue_QualityRetryHasNoSessionID(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var qualityRetryEnv map[string]string

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer — returns a session ID
			out := `{"session_id":"sess-impl-001","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // quality reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 3: // quality retry — capture env
			qualityRetryEnv = make(map[string]string, len(env))
			for k, v := range env {
				qualityRetryEnv[k] = v
			}
			out := `{"session_id":"sess-qual-retry","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 4: // quality reviewer — approves after retry
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer — approves
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if qualityRetryEnv == nil {
		t.Fatal("quality retry was never called")
	}
	if _, ok := qualityRetryEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("quality retry should not receive GODARK_SESSION_ID, got %q", qualityRetryEnv["GODARK_SESSION_ID"])
	}
}

// TestProcessIssue_FunctionalRetryHasSessionID verifies that when the functional
// reviewer requests changes, Retry is called with the implementer's session ID
// (GODARK_SESSION_ID must be set in the retry agent's environment).
func TestProcessIssue_FunctionalRetryHasSessionID(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var functionalRetryEnv map[string]string

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer — returns a session ID
			out := `{"session_id":"sess-impl-001","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // quality reviewer — approves
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		case 3: // functional reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 4: // functional retry — capture env
			functionalRetryEnv = make(map[string]string, len(env))
			for k, v := range env {
				functionalRetryEnv[k] = v
			}
			out := `{"session_id":"sess-func-retry","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		default: // functional reviewer — approves after retry
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if functionalRetryEnv == nil {
		t.Fatal("functional retry was never called")
	}
	if functionalRetryEnv["GODARK_SESSION_ID"] != "sess-impl-001" {
		t.Errorf("GODARK_SESSION_ID passed to functional retry = %q, want %q", functionalRetryEnv["GODARK_SESSION_ID"], "sess-impl-001")
	}
}

// TestProcessIssue_QualityRetrySessionIDUsedByFunctionalRetry verifies that after a
// quality-review retry, the retry's session ID is captured and used by a subsequent
// functional retry (so it resumes the most recent context, not the original implementer's).
func TestProcessIssue_QualityRetrySessionIDUsedByFunctionalRetry(t *testing.T) {
	cfg := loopConfig()
	cfg.MaxRetries = 2

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	var functionalRetryEnv map[string]string

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer — returns original session ID
			out := `{"session_id":"sess-impl","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 2: // quality reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 3: // quality retry — returns its own session ID
			out := `{"session_id":"sess-qual-retry","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 4: // quality reviewer — approves
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		case 5: // functional reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 6: // functional retry — capture env; should have quality retry's session ID
			functionalRetryEnv = make(map[string]string, len(env))
			for k, v := range env {
				functionalRetryEnv[k] = v
			}
			out := `{"session_id":"sess-func-retry","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		default: // functional reviewer — approves
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if functionalRetryEnv == nil {
		t.Fatal("functional retry was never called")
	}
	if functionalRetryEnv["GODARK_SESSION_ID"] != "sess-qual-retry" {
		t.Errorf("GODARK_SESSION_ID passed to functional retry = %q, want %q (quality retry's session ID)", functionalRetryEnv["GODARK_SESSION_ID"], "sess-qual-retry")
	}
}

// --- PR lifecycle label tests ---

// setupGHLabelTracker replaces github.CommandRunner with a wrapper that
// captures --add-label and --remove-label values from issue edit calls.
// Returns slices that are populated as calls occur.
func setupGHLabelTracker(t *testing.T) (added *[]string, removed *[]string) {
	t.Helper()
	var addedLabels, removedLabels []string
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		for i, a := range args {
			if a == "--add-label" && i+1 < len(args) {
				addedLabels = append(addedLabels, args[i+1])
			}
			if a == "--remove-label" && i+1 < len(args) {
				removedLabels = append(removedLabels, args[i+1])
			}
		}
		return []byte(""), nil
	}
	return &addedLabels, &removedLabels
}

// standardLoopGuard returns a guard function that satisfies the minimum
// responses needed for a successful loop run (SHA, PR number, PR body,
// drift diff). All other calls return empty success.
func standardLoopGuard() func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	}
}

func TestProcessIssue_NoneModeLabelsAwaitingReview(t *testing.T) {
	added, _ := setupGHLabelTracker(t)

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "none"

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, standardLoopGuard())

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "ready-to-merge", outcome.Err)
	}

	found := false
	for _, l := range *added {
		if l == label.AwaitingHumanReview {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q label to be added, got: %v", label.AwaitingHumanReview, *added)
	}
}

func TestProcessIssue_LowRiskMode_HighRiskLabelsAwaitingReview(t *testing.T) {
	// High-risk PR (400 lines changed > 200 default threshold) should be labeled
	// awaiting-human-review and not auto-merged.
	var addedLabels []string
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		for i, a := range args {
			if a == "--add-label" && i+1 < len(args) {
				addedLabels = append(addedLabels, args[i+1])
			}
		}
		if strings.Contains(joined, "--json additions,deletions,changedFiles") {
			// Return high-risk stats: 400 total lines changed (> 200 threshold).
			return []byte(`{"additions":300,"deletions":100,"changedFiles":5}`), nil
		}
		if strings.Contains(joined, "pr diff") && strings.Contains(joined, "--name-only") {
			return []byte("internal/agent/loop.go\n"), nil
		}
		return []byte(""), nil
	}

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "low_risk"

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, standardLoopGuard())

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "ready-to-merge", outcome.Err)
	}

	found := false
	for _, l := range addedLabels {
		if l == label.AwaitingHumanReview {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q label to be added for high-risk PR, got: %v", label.AwaitingHumanReview, addedLabels)
	}
}

func TestProcessIssue_LowRiskMode_LowRiskMerges(t *testing.T) {
	// Small PR (10 lines changed, 2 files, no protected paths, no quality flags) is
	// low-risk and should be auto-merged. The reviewer includes a "Read" trace entry
	// so the no_diff_read quality gate is satisfied, resulting in no quality flags.
	var mergeCallCount int
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json additions,deletions,changedFiles") {
			return []byte(`{"additions":8,"deletions":2,"changedFiles":2}`), nil
		}
		if strings.Contains(joined, "pr diff") && strings.Contains(joined, "--name-only") {
			return []byte("internal/agent/loop.go\ninternal/agent/loop_test.go\n"), nil
		}
		return []byte(""), nil
	}

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "low_risk"
	cfg.ProtectedPaths = []string{"CLAUDE.md"}

	// Stub Runner directly so we can include a tool trace with a "Read" entry
	// for the functional reviewer, preventing the no_diff_read quality flag.
	callIdx := 0
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	reviewerOut := wrapRunnerJSONWithTrace("reviewer output\nAGENT_RESULT=APPROVED\n", []string{"Read pr diff"})
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer
			return []byte(reviewerOut), []byte(""), 0, nil
		}
	}
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr merge") {
			mergeCallCount++
		}
		return []byte(""), nil
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if mergeCallCount != 1 {
		t.Errorf("gh pr merge called %d time(s), want 1 for low-risk PR", mergeCallCount)
	}
}

func TestProcessIssue_LowRiskMode_ProtectedPathLabels(t *testing.T) {
	// PR touching a protected path should be labeled awaiting-human-review even
	// if total line/file counts are within risk thresholds.
	var addedLabels []string
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		for i, a := range args {
			if a == "--add-label" && i+1 < len(args) {
				addedLabels = append(addedLabels, args[i+1])
			}
		}
		if strings.Contains(joined, "--json additions,deletions,changedFiles") {
			return []byte(`{"additions":5,"deletions":2,"changedFiles":2}`), nil
		}
		if strings.Contains(joined, "pr diff") && strings.Contains(joined, "--name-only") {
			// One of the changed files is a protected path.
			return []byte("CLAUDE.md\ninternal/agent/loop.go\n"), nil
		}
		return []byte(""), nil
	}

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "low_risk"
	cfg.ProtectedPaths = []string{"CLAUDE.md"}

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, standardLoopGuard())

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "ready-to-merge", outcome.Err)
	}

	found := false
	for _, l := range addedLabels {
		if l == label.AwaitingHumanReview {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q label for protected-path PR, got: %v", label.AwaitingHumanReview, addedLabels)
	}
}

func TestProcessIssue_LowRiskMode_RiskAssessmentWritten(t *testing.T) {
	// Risk assessment should be written to run data via the hook.
	// Uses a reviewer output with a "Read" trace entry so no_diff_read is not raised,
	// ensuring the PR qualifies as low-risk and the assessment records IsLowRisk=true.
	hook := &testRunDataHook{}
	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--json additions,deletions,changedFiles") {
			return []byte(`{"additions":8,"deletions":2,"changedFiles":2}`), nil
		}
		if strings.Contains(joined, "pr diff") && strings.Contains(joined, "--name-only") {
			return []byte("internal/agent/loop.go\n"), nil
		}
		return []byte(""), nil
	}

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "low_risk"
	cfg.ProtectedPaths = []string{"CLAUDE.md"}

	// Stub Runner with a reviewer output that includes a "Read" trace entry.
	callIdx := 0
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	reviewerOut := wrapRunnerJSONWithTrace("reviewer output\nAGENT_RESULT=APPROVED\n", []string{"Read pr diff"})
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1:
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2:
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default:
			return []byte(reviewerOut), []byte(""), 0, nil
		}
	}
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	}

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook, nil)

	if len(hook.riskAssessments) == 0 {
		t.Fatal("expected risk assessment to be written to hook, but none recorded")
	}
	assessment := hook.riskAssessments[0]
	if !assessment.IsLowRisk {
		t.Errorf("expected IsLowRisk=true for small PR with no protected paths, got false (gates: %v)", assessment.Gates)
	}
}

func TestProcessIssue_AllMode_IgnoresRisk(t *testing.T) {
	// auto_merge: all should merge regardless of PR size — risk classifier not called.
	var mergeCallCount int
	cfg := loopConfig()
	cfg.AutoMerge.Feature = "all"

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr merge") {
			mergeCallCount++
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if mergeCallCount != 1 {
		t.Errorf("gh pr merge called %d time(s), want 1 when auto_merge=all", mergeCallCount)
	}
}

func TestProcessIssue_AllModeSkipsLifecycleLabel(t *testing.T) {
	added, _ := setupGHLabelTracker(t)

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "all"

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, standardLoopGuard())

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}

	for _, l := range *added {
		if l == label.AwaitingHumanReview {
			t.Errorf("expected no %q label for auto_merge=all, but it was added", label.AwaitingHumanReview)
		}
	}
}

func TestProcessIssue_MergeRemovesLifecycleLabels(t *testing.T) {
	added, removed := setupGHLabelTracker(t)

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "all"

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}, standardLoopGuard())

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}

	// auto_merge: all does not apply lifecycle labels before merge, but after
	// merge all lifecycle labels are removed unconditionally (to clean up labels
	// applied by a previous run or manually).
	all := label.All()
	allSet := make(map[string]bool, len(all))
	for _, lbl := range all {
		allSet[lbl] = true
	}
	for _, lbl := range *added {
		if allSet[lbl] {
			t.Errorf("unexpected lifecycle label applied on auto_merge=all path: %q", lbl)
		}
	}
	removedSet := make(map[string]bool, len(*removed))
	for _, lbl := range *removed {
		removedSet[lbl] = true
	}
	for _, lbl := range all {
		if !removedSet[lbl] {
			t.Errorf("expected lifecycle label %q to be removed after merge, but it was not", lbl)
		}
	}
}

func TestProcessIssue_FunctionalEscalationLabelsAwaitingReview(t *testing.T) {
	added, _ := setupGHLabelTracker(t)

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "all"
	cfg.MaxRetries = 0 // exhaust immediately

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
	}, standardLoopGuard())

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "needs-human-review" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "needs-human-review", outcome.Err)
	}

	found := false
	for _, l := range *added {
		if l == label.AwaitingHumanReview {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q label to be added on escalation, got: %v", label.AwaitingHumanReview, *added)
	}
}

func TestProcessIssue_QualityEscalationLabelsAwaitingReview(t *testing.T) {
	added, _ := setupGHLabelTracker(t)

	cfg := loopConfig()
	cfg.AutoMerge.Feature = "all"
	cfg.MaxRetries = 0 // exhaust quality retries immediately

	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=CHANGES_REQUESTED",
	}, standardLoopGuard())

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "needs-human-review" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "needs-human-review", outcome.Err)
	}

	found := false
	for _, l := range *added {
		if l == label.AwaitingHumanReview {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q label to be added on quality escalation, got: %v", label.AwaitingHumanReview, *added)
	}
}

// TestProcessIssue_VerifyRerunsAfterQualityGateRetry verifies that when the
// quality reviewer requests changes and the retry agent runs, verify is
// re-executed before the next quality review cycle.
func TestProcessIssue_VerifyRerunsAfterQualityGateRetry(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	shCallCount := 0
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			shCallCount++
			return []byte(""), nil // all verify runs pass
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 3: // retry agent
			return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
		case 4: // quality reviewer — approves after retry+verify
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	cfg := verifyLoopConfig(true)
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented (err: %v)", outcome.Status, outcome.Err)
	}
	// Expect 2 verify runs: one initial (before quality review) and one after
	// the quality-gate retry.
	if shCallCount != 2 {
		t.Errorf("shCallCount = %d, want 2 (initial verify + verify after quality-gate retry)", shCallCount)
	}
}

// TestProcessIssue_VerifyBlockingFailsAfterQualityGateRetry verifies that when
// verify fails (blocking) after a quality-gate retry, the run is marked failed.
func TestProcessIssue_VerifyBlockingFailsAfterQualityGateRetry(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	shCallCount := 0
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			shCallCount++
			if shCallCount == 1 {
				// Initial verify passes.
				return []byte(""), nil
			}
			// Verify after quality-gate retry fails.
			return []byte("build error"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		default: // retry agent
			return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
		}
	}

	cfg := verifyLoopConfig(true) // Blocking = true
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want failed when verify blocks after quality-gate retry (err: %v)", outcome.Status, outcome.Err)
	}
	if outcome.Err == nil || !strings.Contains(outcome.Err.Error(), "verify") {
		t.Errorf("Err = %v, want error containing 'verify'", outcome.Err)
	}
}

// TestProcessIssue_VerifyNonBlockingContinuesAfterQualityGateRetry verifies
// that when verify fails (non-blocking) after a quality-gate retry, the
// quality review cycle continues rather than aborting.
func TestProcessIssue_VerifyNonBlockingContinuesAfterQualityGateRetry(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		if name == "sh" && len(args) > 0 && args[0] == "-c" {
			// All verify runs fail (non-blocking — proceeds to review).
			return []byte("build error"), fmt.Errorf("exit status 1")
		}
		return []byte(""), nil
	}

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 3: // retry agent
			return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
		case 4: // quality reviewer — approves (verify non-blocking, so we get here)
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	cfg := verifyLoopConfig(false) // Blocking = false → proceeds despite verify failure
	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want implemented when verify is non-blocking after quality-gate retry (err: %v)", outcome.Status, outcome.Err)
	}
}

// TestProcessIssue_ReconRunsBeforeImplement verifies that when prompts.Recon is
// configured, the recon agent is called before the implementer.
func TestProcessIssue_ReconRunsBeforeImplement(t *testing.T) {
	callOrder := []string{}

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		role := env["GODARK_ROLE"]
		callOrder = append(callOrder, role)
		callIdx++
		switch callIdx {
		case 1: // recon
			return []byte(wrapRunnerJSON("recon brief text")), []byte(""), 0, nil
		case 2: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	prompts := &Prompts{
		Recon:           "Recon issue #{{.IssueNumber}}",
		Implementer:     "Implement #{{.IssueNumber}} {{.IssueTitle}}",
		ImplementerRetry: "Retry PR #{{.PRNumber}}",
		Reviewer:        "Review PR #{{.PRNumber}}",
		QualityReviewer: "Quality review PR #{{.PRNumber}}",
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	if len(callOrder) < 2 {
		t.Fatalf("expected at least 2 agent calls, got %d", len(callOrder))
	}
	if callOrder[0] != "recon" {
		t.Errorf("first agent call role = %q, want %q", callOrder[0], "recon")
	}
	if callOrder[1] != "implementer" {
		t.Errorf("second agent call role = %q, want %q", callOrder[1], "implementer")
	}
}

// TestProcessIssue_ReconOutputPassedToImplementer verifies that the recon
// agent's ResultText is passed as reconBrief to the implementer prompt.
func TestProcessIssue_ReconOutputPassedToImplementer(t *testing.T) {
	var implementerPrompt string

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // recon
			return []byte(wrapRunnerJSON("key finding: use approach X")), []byte(""), 0, nil
		case 2: // implementer
			implementerPrompt = env["GODARK_PROMPT"]
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	prompts := &Prompts{
		Recon:            "Recon issue #{{.IssueNumber}}",
		Implementer:      "Implement #{{.IssueNumber}}{{if .ReconBrief}} brief={{.ReconBrief}}{{end}}",
		ImplementerRetry: "Retry PR #{{.PRNumber}}",
		Reviewer:         "Review PR #{{.PRNumber}}",
		QualityReviewer:  "Quality review PR #{{.PRNumber}}",
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	if !strings.Contains(implementerPrompt, "key finding: use approach X") {
		t.Errorf("implementer prompt should contain recon output, got: %s", implementerPrompt)
	}
}

// TestProcessIssue_ReconFailureNonBlocking verifies that when recon fails,
// Implement is still called with an empty reconBrief.
func TestProcessIssue_ReconFailureNonBlocking(t *testing.T) {
	var implementerPrompt string

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // recon — simulate failure via non-zero exit
			return []byte(""), []byte("error output"), 1, nil
		case 2: // implementer
			implementerPrompt = env["GODARK_PROMPT"]
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 3: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	prompts := &Prompts{
		Recon:            "Recon issue #{{.IssueNumber}}",
		Implementer:      "Implement #{{.IssueNumber}}{{if .ReconBrief}} brief={{.ReconBrief}}{{end}}",
		ImplementerRetry: "Retry PR #{{.PRNumber}}",
		Reviewer:         "Review PR #{{.PRNumber}}",
		QualityReviewer:  "Quality review PR #{{.PRNumber}}",
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	// Implementation should still proceed even if recon failed.
	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if strings.Contains(implementerPrompt, "brief=") {
		t.Errorf("implementer prompt should not contain brief when recon failed, got: %s", implementerPrompt)
	}
}

// TestProcessIssue_ReconTimeoutNonBlocking verifies that when the recon agent
// times out, a warning is logged and Implement() is still called with empty reconBrief.
func TestProcessIssue_ReconTimeoutNonBlocking(t *testing.T) {
	var implementerPrompt string
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callIdx := 0
	Runner = func(innerCtx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // recon — cancel context to trigger TimedOut path
			cancel()
			return []byte(""), []byte(""), 0, nil
		case 2: // implementer — capture prompt; context is cancelled so Run returns TimedOut
			implementerPrompt = env["GODARK_PROMPT"]
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		default:
			return []byte(wrapRunnerJSON("")), []byte(""), 0, nil
		}
	}

	prompts := &Prompts{
		Recon:            "Recon issue #{{.IssueNumber}}",
		Implementer:      "Implement #{{.IssueNumber}}{{if .ReconBrief}} brief={{.ReconBrief}}{{end}}",
		ImplementerRetry: "Retry PR #{{.PRNumber}}",
		Reviewer:         "Review PR #{{.PRNumber}}",
		QualityReviewer:  "Quality review PR #{{.PRNumber}}",
	}

	ProcessIssue(ctx, loopIssue(), loopConfig(), prompts, nil, logger, nil, nil)

	// A warning must be logged when recon times out.
	if !strings.Contains(logBuf.String(), "recon agent timed out") {
		t.Errorf("expected warning about recon timeout in log, got: %s", logBuf.String())
	}

	// Implement() must still be called (Runner called at least twice).
	if callIdx < 2 {
		t.Errorf("Implement() was not called after recon timeout (Runner calls = %d)", callIdx)
	}

	// Implement() must receive an empty reconBrief (no "brief=" in its prompt).
	if strings.Contains(implementerPrompt, "brief=") {
		t.Errorf("implementer prompt should not contain brief when recon timed out, got: %s", implementerPrompt)
	}
}

// TestProcessIssue_ReconSkippedWhenUnconfigured verifies that when prompts.Recon
// is empty, the recon agent is not called and the implementer runs first.
func TestProcessIssue_ReconSkippedWhenUnconfigured(t *testing.T) {
	callOrder := []string{}

	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		role := env["GODARK_ROLE"]
		callOrder = append(callOrder, role)
		callIdx++
		switch callIdx {
		case 1: // implementer
			return []byte(wrapRunnerJSON("implementer output")), []byte(""), 0, nil
		case 2: // quality reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		default: // functional reviewer
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	// No Recon prompt configured.
	prompts := testPrompts(t)

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, nil)

	if len(callOrder) == 0 {
		t.Fatal("expected at least one agent call")
	}
	if callOrder[0] == "recon" {
		t.Errorf("recon should not run when prompts.Recon is empty; call order: %v", callOrder)
	}
	if callOrder[0] != "implementer" {
		t.Errorf("first agent call role = %q, want %q; call order: %v", callOrder[0], "implementer", callOrder)
	}
}

// --- assembleHandoffContext tests ---

func TestAssembleHandoffContext_ExtractsImplAndReviewNotes(t *testing.T) {
	prComments := `{"body":"## Implementation Notes\nI tried approach A.","comments":[{"body":"## Review Notes\nApproach A has issues."},{"body":"Some unrelated comment."}]}`
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte(prComments), nil
	})

	result := assembleHandoffContext("owner/repo", 10, testLogger(t))
	if !strings.Contains(result, "## Implementation Notes") {
		t.Errorf("expected ## Implementation Notes in result, got: %s", result)
	}
	if !strings.Contains(result, "## Review Notes") {
		t.Errorf("expected ## Review Notes in result, got: %s", result)
	}
	if strings.Contains(result, "unrelated comment") {
		t.Errorf("unrelated comment should not appear in handoff, got: %s", result)
	}
}

func TestAssembleHandoffContext_ExtractsQualityReviewNotes(t *testing.T) {
	prComments := `{"body":"","comments":[{"body":"## Quality Review Notes\nFailed quality check."}]}`
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte(prComments), nil
	})

	result := assembleHandoffContext("owner/repo", 10, testLogger(t))
	if !strings.Contains(result, "## Quality Review Notes") {
		t.Errorf("expected ## Quality Review Notes in result, got: %s", result)
	}
}

func TestAssembleHandoffContext_EmptyWhenNoStructuredComments(t *testing.T) {
	prComments := `{"body":"Just a PR description.","comments":[{"body":"LGTM"},{"body":"👍"}]}`
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte(prComments), nil
	})

	result := assembleHandoffContext("owner/repo", 10, testLogger(t))
	if result != "" {
		t.Errorf("expected empty result when no structured comments, got: %s", result)
	}
}

func TestAssembleHandoffContext_EmptyOnGhError(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh: not found")
	})

	result := assembleHandoffContext("owner/repo", 10, testLogger(t))
	if result != "" {
		t.Errorf("expected empty result on gh error, got: %s", result)
	}
}

func TestAssembleHandoffContext_UsesCorrectGhArgs(t *testing.T) {
	var capturedArgs []string
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(`{"body":"","comments":[]}`), nil
	})

	assembleHandoffContext("owner/repo", 42, testLogger(t))

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "42") {
		t.Errorf("expected PR number 42 in gh args, got: %v", capturedArgs)
	}
	if !strings.Contains(joined, "--repo") || !strings.Contains(joined, "owner/repo") {
		t.Errorf("expected --repo owner/repo in gh args, got: %v", capturedArgs)
	}
	if !strings.Contains(joined, "body,comments") {
		t.Errorf("expected --json body,comments in gh args, got: %v", capturedArgs)
	}
}

// --- extractSection tests ---

func TestExtractSection_ExtractsContentToNextHeading(t *testing.T) {
	body := "## Implementation Notes\nI tried X.\nIt failed.\n\n## Review Notes\nPlease fix Y."
	result := extractSection(body, "## Implementation Notes")
	if !strings.Contains(result, "I tried X.") {
		t.Errorf("expected implementation content, got: %s", result)
	}
	if strings.Contains(result, "Review Notes") {
		t.Errorf("should not include next section, got: %s", result)
	}
}

func TestExtractSection_ExtractsToEndWhenNoNextHeading(t *testing.T) {
	body := "Some prefix.\n## Review Notes\nThis is the review feedback."
	result := extractSection(body, "## Review Notes")
	if !strings.Contains(result, "This is the review feedback.") {
		t.Errorf("expected review content, got: %s", result)
	}
}

func TestExtractSection_ReturnsEmptyWhenHeadingAbsent(t *testing.T) {
	body := "Some content without any headings."
	result := extractSection(body, "## Implementation Notes")
	if result != "" {
		t.Errorf("expected empty string when heading absent, got: %s", result)
	}
}

// handoffTrackingGuard returns a GuardRunner stub that handles the common calls
// needed by ProcessIssue tests and tracks calls to assembleHandoffContext.
// handoffCallCount, if non-nil, is incremented whenever the
// "gh pr view --json body,comments" call (from assembleHandoffContext) is made.
func handoffTrackingGuard(handoffCallCount *int) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "body,comments") {
			if handoffCallCount != nil {
				*handoffCallCount++
			}
			return []byte(`{"body": "", "comments": []}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	}
}

func TestProcessIssue_ResumeOnAttempt0WithMaxResumeRetries2(t *testing.T) {
	// With MaxResumeRetries: 2, attempt 0 (first retry) uses session resumption.
	// assembleHandoffContext should NOT be called.
	cfg := loopConfig()
	cfg.MaxRetries = 2
	cfg.MaxResumeRetries = 2

	var handoffCallCount int
	// Agent outputs: implementer, quality(APPROVED), reviewer(CHANGES), retry, reviewer(APPROVED)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
		"AGENT_RESULT=APPROVED",
	}, handoffTrackingGuard(&handoffCallCount))

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if handoffCallCount != 0 {
		t.Errorf("assembleHandoffContext called %d time(s) on attempt 0 with MaxResumeRetries=2, expected resume mode (0 calls)", handoffCallCount)
	}
}

func TestProcessIssue_FreshOnAttempt2WithMaxResumeRetries2(t *testing.T) {
	// With MaxResumeRetries: 2, attempt 2 (third retry) uses fresh agent with handoff.
	// assembleHandoffContext SHOULD be called on the third retry (attempt index 2).
	cfg := loopConfig()
	cfg.MaxRetries = 3
	cfg.MaxResumeRetries = 2

	var handoffCallCount int
	// Agent outputs: implementer, quality(APPROVED), review(CHANGES)×3, retry×3, review(APPROVED)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 1",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 2",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 3",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "body,comments") {
			handoffCallCount++
			return []byte(`{"body": "", "comments": []}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	// Attempt 0 and 1 resume (< 2), attempt 2 is fresh (>= 2). So handoff called once.
	if handoffCallCount != 1 {
		t.Errorf("handoff assembled %d time(s), want 1 (only on attempt 2)", handoffCallCount)
	}
}

func TestProcessIssue_AllFreshWithMaxResumeRetries0(t *testing.T) {
	// With MaxResumeRetries: 0, every retry uses fresh mode.
	// assembleHandoffContext should be called on each retry attempt.
	cfg := loopConfig()
	cfg.MaxRetries = 2
	cfg.MaxResumeRetries = 0

	var handoffCallCount int
	// Agent outputs: implementer, quality(APPROVED), review(CHANGES)×2, retry×2, review(APPROVED)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 1",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 2",
		"AGENT_RESULT=APPROVED",
	}, func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "body,comments") {
			handoffCallCount++
			return []byte(`{"body": "", "comments": []}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json number") {
			return []byte(`{"number": 10}`), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") && strings.Contains(joined, "--json body") {
			return []byte(`{"body": "Closes #5"}`), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			return []byte("src/main.go\n"), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	// Both retries (attempt 0 and 1) should use fresh mode with MaxResumeRetries=0.
	if handoffCallCount != 2 {
		t.Errorf("handoff assembled %d time(s), want 2 (all retries fresh)", handoffCallCount)
	}
}

func TestProcessIssue_AllResumeWithMaxResumeRetriesHigherThanMaxRetries(t *testing.T) {
	// With MaxResumeRetries: 10 and MaxRetries: 3, all retry attempts use session resumption.
	// assembleHandoffContext should never be called.
	cfg := loopConfig()
	cfg.MaxRetries = 3
	cfg.MaxResumeRetries = 10

	var handoffCalled int
	// Agent outputs: implementer, quality(APPROVED), review(CHANGES)×3, retry×3, review(APPROVED)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 1",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 2",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output 3",
		"AGENT_RESULT=APPROVED",
	}, handoffTrackingGuard(&handoffCalled))

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if handoffCalled != 0 {
		t.Errorf("assembleHandoffContext called %d time(s) with MaxResumeRetries=10, expected all-resume mode (0 calls)", handoffCalled)
	}
}

func TestHandleNonBlockingResult_Error(t *testing.T) {
	var logBuf bytes.Buffer
	logger := bufferLogger(&logBuf)

	var hookStep rundata.StepResult
	hookCalled := false
	writeHook := func(step rundata.StepResult) error {
		hookStep = step
		hookCalled = true
		return nil
	}

	result := handleNonBlockingResult(nil, fmt.Errorf("something went wrong"), "test-agent", logger, writeHook)

	if result != "" {
		t.Errorf("expected empty string on error, got %q", result)
	}
	if !hookCalled {
		t.Error("expected writeHook to be called on error")
	}
	if hookStep.Error != "something went wrong" {
		t.Errorf("hookStep.Error = %q, want %q", hookStep.Error, "something went wrong")
	}
	if !strings.Contains(logBuf.String(), "test-agent failed, continuing") {
		t.Errorf("expected warning log for failure, got: %s", logBuf.String())
	}
}

func TestHandleNonBlockingResult_Timeout(t *testing.T) {
	var logBuf bytes.Buffer
	logger := bufferLogger(&logBuf)

	var hookStep rundata.StepResult
	hookCalled := false
	writeHook := func(step rundata.StepResult) error {
		hookStep = step
		hookCalled = true
		return nil
	}

	r := &Result{TimedOut: true, ResultText: "ignored"}
	result := handleNonBlockingResult(r, nil, "test-agent", logger, writeHook)

	if result != "" {
		t.Errorf("expected empty string on timeout, got %q", result)
	}
	if !hookCalled {
		t.Error("expected writeHook to be called on timeout")
	}
	if hookStep.Error != "timed out" {
		t.Errorf("hookStep.Error = %q, want %q", hookStep.Error, "timed out")
	}
	if !strings.Contains(logBuf.String(), "test-agent timed out, continuing") {
		t.Errorf("expected timeout warning log, got: %s", logBuf.String())
	}
}

func TestHandleNonBlockingResult_Success(t *testing.T) {
	var logBuf bytes.Buffer
	logger := bufferLogger(&logBuf)

	var hookStep rundata.StepResult
	hookCalled := false
	writeHook := func(step rundata.StepResult) error {
		hookStep = step
		hookCalled = true
		return nil
	}

	r := &Result{ResultText: "the brief", TimedOut: false}
	result := handleNonBlockingResult(r, nil, "test-agent", logger, writeHook)

	if result != "the brief" {
		t.Errorf("expected result text %q, got %q", "the brief", result)
	}
	if !hookCalled {
		t.Error("expected writeHook to be called on success")
	}
	if hookStep.Error != "" {
		t.Errorf("hookStep.Error should be empty on success, got %q", hookStep.Error)
	}
}

func TestHandleNonBlockingResult_NilHook(t *testing.T) {
	logger := slog.Default()

	// Should not panic with nil writeHook in any case.
	result := handleNonBlockingResult(nil, fmt.Errorf("oops"), "test-agent", logger, nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}

	r := &Result{TimedOut: true}
	result = handleNonBlockingResult(r, nil, "test-agent", logger, nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}

	r2 := &Result{ResultText: "ok"}
	result = handleNonBlockingResult(r2, nil, "test-agent", logger, nil)
	if result != "ok" {
		t.Errorf("expected %q, got %q", "ok", result)
	}
}

// qualityGuardFn is a minimal GuardRunner stub for runQualityReviewCycle tests.
// It handles git rev-parse (baseSHA), git diff (drift check), gh pr view, and
// gh pr comment without error.
func qualityGuardFn(name string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
		return []byte("abc123\n"), nil
	}
	if name == "git" && strings.Contains(joined, "diff --name-only") {
		return []byte("src/main.go\n"), nil // non-protected file → no drift
	}
	if name == "gh" && strings.Contains(joined, "pr view") {
		return []byte(""), nil
	}
	if name == "gh" && strings.Contains(joined, "pr comment") {
		return []byte(""), nil
	}
	return []byte(""), nil
}

// TestRunQualityReviewCycle_ApprovedFirstTry verifies that the cycle returns
// (true, nil) when the quality reviewer approves on the first attempt.
func TestRunQualityReviewCycle_ApprovedFirstTry(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = qualityGuardFn

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
	}

	cfg := loopConfig()
	sessionID := "sess-init"
	fixCycles := 0
	passed, err := runQualityReviewCycle(
		context.Background(),
		loopIssue(),
		10,
		"issue-5-test-issue",
		"abc123",
		cfg,
		testPrompts(t),
		nil,
		testLogger(t),
		nil,
		&sessionID,
		&fixCycles,
		nil,
	)

	if !passed {
		t.Errorf("passed = false, want true")
	}
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

// TestRunQualityReviewCycle_ApprovedAfterRetry verifies that the cycle returns
// (true, nil) after CHANGES_REQUESTED on the first attempt and APPROVED on
// the second.
func TestRunQualityReviewCycle_ApprovedAfterRetry(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = qualityGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // quality reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 2: // retry agent
			return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
		default: // quality reviewer — approves
			return []byte(wrapRunnerJSON("AGENT_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	cfg := loopConfig()
	sessionID := "sess-init"
	fixCycles := 0
	passed, err := runQualityReviewCycle(
		context.Background(),
		loopIssue(),
		10,
		"issue-5-test-issue",
		"abc123",
		cfg,
		testPrompts(t),
		nil,
		testLogger(t),
		nil,
		&sessionID,
		&fixCycles,
		nil,
	)

	if !passed {
		t.Errorf("passed = false, want true")
	}
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

// TestRunQualityReviewCycle_ExhaustsRetries verifies that the cycle returns
// (false, nil) when all quality review attempts are exhausted without approval.
func TestRunQualityReviewCycle_ExhaustsRetries(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = qualityGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		if callIdx%2 == 1 {
			// quality reviewer — always requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		}
		// retry agent
		return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
	}

	cfg := loopConfig()
	cfg.MaxRetries = 1 // qualityMaxAttempts = 2
	sessionID := "sess-init"
	fixCycles := 0
	passed, err := runQualityReviewCycle(
		context.Background(),
		loopIssue(),
		10,
		"issue-5-test-issue",
		"abc123",
		cfg,
		testPrompts(t),
		nil,
		testLogger(t),
		nil,
		&sessionID,
		&fixCycles,
		nil,
	)

	if passed {
		t.Errorf("passed = true, want false")
	}
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

// TestRunQualityReviewCycle_DriftDuringQuality verifies that the cycle returns
// (false, driftErr) when drift is detected after a quality-gate retry.
func TestRunQualityReviewCycle_DriftDuringQuality(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		if callIdx == 1 {
			// quality reviewer — requests changes
			return []byte(wrapRunnerJSON("AGENT_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		}
		// retry agent
		return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
	}

	GuardRunner = func(name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return []byte("abc123\n"), nil
		}
		if name == "git" && strings.Contains(joined, "diff --name-only") {
			// Return a protected file to trigger drift detection.
			return []byte("CLAUDE.md\n"), nil
		}
		if name == "gh" && strings.Contains(joined, "pr view") {
			return []byte(""), nil
		}
		return []byte(""), nil
	}

	cfg := loopConfig()
	sessionID := "sess-init"
	fixCycles := 0
	passed, err := runQualityReviewCycle(
		context.Background(),
		loopIssue(),
		10,
		"issue-5-test-issue",
		"abc123",
		cfg,
		testPrompts(t),
		nil,
		testLogger(t),
		nil,
		&sessionID,
		&fixCycles,
		nil,
	)

	if passed {
		t.Errorf("passed = true, want false")
	}
	if err == nil {
		t.Errorf("err = nil, want drift error")
	}
	if !strings.Contains(err.Error(), "protected path drift") {
		t.Errorf("err = %q, want to contain %q", err.Error(), "protected path drift")
	}
}

// mockReporter records IssueStageChanged calls for assertion in stage-emission tests.
// All other ProgressReporter methods are no-ops.
type mockReporter struct {
	stageChanges []struct {
		number int
		stage  string
	}
}

func (r *mockReporter) IssueStageChanged(number int, stage string) {
	r.stageChanges = append(r.stageChanges, struct {
		number int
		stage  string
	}{number, stage})
}
func (r *mockReporter) RunStarted(_, _, _, _, _, _ string, _ []progress.IssueSummary) {}
func (r *mockReporter) IssueStarted(_ int, _ string)              {}
func (r *mockReporter) IssueCompleted(_ int, _ string, _ string, _, _ int, _ string, _ float64) {
}
func (r *mockReporter) WaveStarted(_, _ int)              {}
func (r *mockReporter) AllBlocked(_, _ int)               {}
func (r *mockReporter) RollupCreated(_ int, _ string, _ bool) {}
func (r *mockReporter) RunAborted(_ string)               {}
func (r *mockReporter) RunFinished(_, _, _, _, _ int)     {}
func (r *mockReporter) PunchlistText(_ string)            {}

func (r *mockReporter) hasStageChange(number int, stage string) bool {
	for _, sc := range r.stageChanges {
		if sc.number == number && sc.stage == stage {
			return true
		}
	}
	return false
}

func (r *mockReporter) countStageChanges(stage string) int {
	count := 0
	for _, sc := range r.stageChanges {
		if sc.stage == stage {
			count++
		}
	}
	return count
}

func TestProcessIssue_ImplementStageEmitted(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	reporter := &mockReporter{}
	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, reporter)

	if !reporter.hasStageChange(loopIssue().Number, "implement") {
		t.Errorf("IssueStageChanged(%d, %q) was not called", loopIssue().Number, "implement")
	}
}

func TestProcessIssue_VerifyStageEmitted(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	reporter := &mockReporter{}
	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, reporter)

	if !reporter.hasStageChange(loopIssue().Number, "verify") {
		t.Errorf("IssueStageChanged(%d, %q) was not called", loopIssue().Number, "verify")
	}
}

func TestProcessIssue_ReviewStageEmitted(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	reporter := &mockReporter{}
	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, reporter)

	if !reporter.hasStageChange(loopIssue().Number, "review") {
		t.Errorf("IssueStageChanged(%d, %q) was not called", loopIssue().Number, "review")
	}
}

func TestProcessIssue_ReconStageEmitted(t *testing.T) {
	setupLoopTest(t, []string{
		"recon output",
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	prompts := testPrompts(t)
	prompts.Recon = "recon prompt"

	reporter := &mockReporter{}
	ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil, reporter)

	if !reporter.hasStageChange(loopIssue().Number, "recon") {
		t.Errorf("IssueStageChanged(%d, %q) was not called when recon is configured", loopIssue().Number, "recon")
	}
}

func TestProcessIssue_NilReporterSafe(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	// Should not panic with nil reporter.
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)
	if outcome.Status != StatusImplemented {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, StatusImplemented, outcome.Err)
	}
}

func TestProcessIssue_RetryReentersImplementStage(t *testing.T) {
	// Agent outputs: implementer, quality(APPROVED), reviewer(CHANGES), retry, reviewer(APPROVED)
	setupLoopTest(t, []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"AGENT_RESULT=CHANGES_REQUESTED",
		"retry output",
		"AGENT_RESULT=APPROVED",
	}, loopGuardFn)

	reporter := &mockReporter{}
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, reporter)

	if outcome.Status != StatusImplemented {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, StatusImplemented, outcome.Err)
	}

	// "implement" should be emitted at least twice: once for initial implement, once for retry.
	count := reporter.countStageChanges("implement")
	if count < 2 {
		t.Errorf("IssueStageChanged(_, %q) called %d time(s), want at least 2 (initial + retry)", "implement", count)
	}
}

// rateLimitOutput returns the agent JSON output for an Anthropic account-level rate limit.
func rateLimitOutput() string {
	return `{"session_id":"","result":"You've hit your limit \u00b7 resets 5pm (UTC)","cost_usd":0,"is_error":true}`
}

func TestRateLimitErr_IncludesResetTime(t *testing.T) {
	err := rateLimitErr("You've hit your limit · resets 5pm (UTC)")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "resets 5pm (UTC)") {
		t.Errorf("error %q does not contain reset time", err.Error())
	}
}

func TestRateLimitErr_WrapsErrRateLimited(t *testing.T) {
	err := rateLimitErr("You've hit your limit · resets 5pm (UTC)")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected errors.Is(err, ErrRateLimited) to be true, got err = %v", err)
	}
}

func TestRateLimitErr_NoResetTime_StillWrapsErrRateLimited(t *testing.T) {
	err := rateLimitErr("You've hit your limit")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected errors.Is(err, ErrRateLimited) to be true, got err = %v", err)
	}
}

func TestProcessIssue_RateLimited_ByImplementer(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		// Implementer returns a rate limit response.
		return []byte(rateLimitOutput()), []byte(""), 1, nil
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if !outcome.RateLimited {
		t.Error("expected RateLimited = true when implementer hits rate limit")
	}
	if outcome.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", outcome.Status, StatusFailed)
	}
	if outcome.Err == nil {
		t.Fatal("expected non-nil Err")
	}
	if !errors.Is(outcome.Err, ErrRateLimited) {
		t.Errorf("expected errors.Is(outcome.Err, ErrRateLimited), got %v", outcome.Err)
	}
}

func TestProcessIssue_RateLimited_ByReviewer(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	callIdx := 0
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		callIdx++
		switch callIdx {
		case 1: // implementer — succeeds
			return []byte(wrapRunnerJSON("implementation done")), []byte(""), 0, nil
		case 2: // quality reviewer — rate limited
			return []byte(rateLimitOutput()), []byte(""), 1, nil
		default:
			return []byte(wrapRunnerJSON("unexpected call")), []byte(""), 0, nil
		}
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if !outcome.RateLimited {
		t.Error("expected RateLimited = true when quality reviewer hits rate limit")
	}
	if outcome.Status != StatusFailed {
		t.Errorf("Status = %q, want %q", outcome.Status, StatusFailed)
	}
	if !errors.Is(outcome.Err, ErrRateLimited) {
		t.Errorf("expected errors.Is(outcome.Err, ErrRateLimited), got %v", outcome.Err)
	}
}

func TestProcessIssue_NormalFailure_NotRateLimited(t *testing.T) {
	origRunner := Runner
	origGuard := GuardRunner
	t.Cleanup(func() {
		Runner = origRunner
		GuardRunner = origGuard
	})
	GuardRunner = loopGuardFn

	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		// Implementer fails with a normal error (not a rate limit).
		return []byte(wrapRunnerJSON("something went wrong")), []byte(""), 1, nil
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil, nil)

	if outcome.RateLimited {
		t.Error("expected RateLimited = false for normal failure")
	}
}
