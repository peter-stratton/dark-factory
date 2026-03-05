package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/rundata"
)

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
		"QUALITY_RESULT=APPROVED",
		"reviewer output\nREVIEW_RESULT=APPROVED\n",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=CHANGES_REQUESTED",
		"retry output",
		"REVIEW_RESULT=APPROVED",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=CHANGES_REQUESTED",
		"retry output",
		"REVIEW_RESULT=CHANGES_REQUESTED",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=CHANGES_REQUESTED",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if !mergedCalled {
		t.Error("expected gh pr merge to be called")
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
		"reviewer output\nREVIEW_RESULT=APPROVED\n",
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
			return []byte(wrapRunnerJSON("QUALITY_RESULT=APPROVED")), []byte(""), 0, nil
		default:
			return []byte(wrapRunnerJSON("reviewer output\nREVIEW_RESULT=APPROVED\n")), []byte(""), 0, nil
		}
	}

	// No SpecGenerator prompt → should only have 3 agent calls (implement + quality + review).
	prompts := testPrompts(t)
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil)

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
			return []byte(wrapRunnerJSON("QUALITY_RESULT=APPROVED")), []byte(""), 0, nil
		case 3: // reviewer — requests changes
			return []byte(wrapRunnerJSON("REVIEW_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 4: // first retry — capture env
			retryEnv = make(map[string]string, len(env))
			for k, v := range env {
				retryEnv[k] = v
			}
			out := `{"session_id":"sess-retry-002","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		default: // reviewer after retry — approve
			return []byte(wrapRunnerJSON("REVIEW_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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
			return []byte(wrapRunnerJSON("QUALITY_RESULT=APPROVED")), []byte(""), 0, nil
		case 3: // reviewer — changes requested
			return []byte(wrapRunnerJSON("REVIEW_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 4: // first retry — returns its own session ID
			out := `{"session_id":"sess-retry-1","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		case 5: // reviewer — changes requested again
			return []byte(wrapRunnerJSON("REVIEW_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 6: // second retry — capture env
			secondRetryEnv = make(map[string]string, len(env))
			for k, v := range env {
				secondRetryEnv[k] = v
			}
			out := `{"session_id":"sess-retry-2","result":"ok","cost_usd":0,"is_error":false}`
			return []byte(out), []byte(""), 0, nil
		default: // reviewer after second retry — approve
			return []byte(wrapRunnerJSON("REVIEW_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

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
			return []byte(wrapRunnerJSON("QUALITY_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer calls — capture env
			captured := make(map[string]string, len(env))
			for k, v := range env {
				captured[k] = v
			}
			reviewerEnvs = append(reviewerEnvs, captured)
			return []byte(wrapRunnerJSON("REVIEW_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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
		"QUALITY_RESULT=CHANGES_REQUESTED",
		"retry output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)

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
			return []byte(wrapRunnerJSON("QUALITY_RESULT=CHANGES_REQUESTED")), []byte(""), 0, nil
		case 3: // retry
			return []byte(wrapRunnerJSON("retry output")), []byte(""), 0, nil
		case 4: // quality review cycle=1 (should have directive)
			if env["GODARK_ROLE"] == "quality_reviewer" {
				qualityPrompts = append(qualityPrompts, env["GODARK_PROMPT"])
			}
			return []byte(wrapRunnerJSON("QUALITY_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("REVIEW_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	ProcessIssue(context.Background(), loopIssue(), cfg, prompts, nil, testLogger(t), nil)

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

func TestProcessIssue_NoMerge_SkipsMergeOnApproval(t *testing.T) {
	var mergeCallCount int
	cfg := loopConfig()
	cfg.NoMerge = true

	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "ready-to-merge", outcome.Err)
	}
	if mergeCallCount != 0 {
		t.Errorf("gh pr merge was called %d time(s), want 0 when NoMerge=true", mergeCallCount)
	}
	if outcome.PRNumber != 10 {
		t.Errorf("PRNumber = %d, want 10", outcome.PRNumber)
	}
}

func TestProcessIssue_NoMerge_OutcomeStatus(t *testing.T) {
	cfg := loopConfig()
	cfg.NoMerge = true

	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

	if outcome.Status != "ready-to-merge" {
		t.Errorf("Status = %q, want %q", outcome.Status, "ready-to-merge")
	}
}

func TestProcessIssue_NoMerge_DefaultMerges(t *testing.T) {
	// Without NoMerge, approved PR should still be merged.
	var mergeCallCount int
	cfg := loopConfig() // NoMerge defaults to false

	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
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

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if mergeCallCount != 1 {
		t.Errorf("gh pr merge was called %d time(s), want 1 when NoMerge=false", mergeCallCount)
	}
}

func TestProcessIssue_NoMerge_QualityReviewStillRuns(t *testing.T) {
	// With NoMerge=true, quality review should still execute normally.
	cfg := loopConfig()
	cfg.NoMerge = true

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
			return []byte(wrapRunnerJSON("QUALITY_RESULT=APPROVED")), []byte(""), 0, nil
		default: // reviewer
			return []byte(wrapRunnerJSON("REVIEW_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

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
			return []byte(wrapRunnerJSON("REVIEW_RESULT=APPROVED")), []byte(""), 0, nil
		}
	}

	prompts := testPrompts(t)
	prompts.QualityReviewer = "" // disable quality reviewer
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t), nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if agentCallCount != 2 {
		t.Errorf("expected 2 agent calls (implement + review), got %d", agentCallCount)
	}
}

// testRunDataHook is a simple RunDataHook implementation for unit tests.
type testRunDataHook struct {
	implementCalls   int
	reviewKinds      []string
	retryCalls       int
	retryReviewCalls int
	outcomes         []rundata.Outcome
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
func (h *testRunDataHook) WriteOutcome(o rundata.Outcome) error {
	h.outcomes = append(h.outcomes, o)
	return nil
}

func TestProcessIssue_HookCalledOnImplement(t *testing.T) {
	hook := &testRunDataHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook)

	if hook.implementCalls != 1 {
		t.Errorf("WriteImplementResult called %d times, want 1", hook.implementCalls)
	}
}

func TestProcessIssue_HookCalledOnReview(t *testing.T) {
	hook := &testRunDataHook{}
	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook)

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
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), hook)

	if len(hook.outcomes) != 1 {
		t.Fatalf("WriteOutcome called %d times, want 1", len(hook.outcomes))
	}
	if hook.outcomes[0].Status != outcome.Status {
		t.Errorf("WriteOutcome status = %q, want %q", hook.outcomes[0].Status, outcome.Status)
	}
	if hook.outcomes[0].IssueNumber != loopIssue().Number {
		t.Errorf("WriteOutcome issue_number = %d, want %d", hook.outcomes[0].IssueNumber, loopIssue().Number)
	}
}

func TestProcessIssue_NilHookSafe(t *testing.T) {
	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	// Should not panic with nil hook.
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t), nil)
	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
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

	// Quality reviewer (checkTestExecution=false): should NOT produce test-execution flags.
	qFlags := computeReviewFlags(result, cfg, false)
	for _, f := range qFlags {
		if f.Code == "no_review_tests_written" || f.Code == "no_review_tests_run" {
			t.Errorf("quality reviewer should be exempt from %q, got flag: %+v", f.Code, f)
		}
	}

	// Functional reviewer (checkTestExecution=true): SHOULD produce test-execution flags.
	fFlags := computeReviewFlags(result, cfg, true)
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

	flags := computeReviewFlags(result, cfg, false)

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

	flags := computeReviewFlags(result, cfg, false)
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
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, logger, nil)
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
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook)
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
	cfg := loopConfig()
	cfg.TestCommand = "go test ./..."
	cfg.ReviewDir = "tests/review/"

	hook := newCaptureHook()

	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"REVIEW_RESULT=APPROVED",
	}, loopGuardFn)

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), hook)

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
