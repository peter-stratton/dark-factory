package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t))

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t))

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

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t))

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

	ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), testPrompts(t), nil, testLogger(t))

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

	ProcessIssue(context.Background(), loopIssue(), cfg, prompts, nil, testLogger(t))

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
	outcome := ProcessIssue(context.Background(), loopIssue(), loopConfig(), prompts, nil, testLogger(t))

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if agentCallCount != 2 {
		t.Errorf("expected 2 agent calls (implement + review), got %d", agentCallCount)
	}
}
