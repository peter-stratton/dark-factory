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

	Runner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		out := stubs.nextAgentOutput()
		return []byte(out), []byte(""), 0, nil
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
	// Agent outputs: 1=implementer, 2=reviewer(APPROVED)
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
	// Agent outputs: implementer, reviewer(CHANGES), retry, reviewer(APPROVED)
	callIdx := 0
	setupLoopTest(t, []string{
		"implementer output",
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

	// Agent outputs: implementer, reviewer(CHANGES), retry, reviewer(CHANGES)
	setupLoopTest(t, []string{
		"implementer output",
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

func TestProcessIssue_MergeAndPullOnApproval(t *testing.T) {
	var mergedCalled, pullCalled bool
	setupLoopTest(t, []string{
		"implementer output",
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
		if name == "git" && strings.Contains(joined, "pull --rebase") {
			pullCalled = true
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
	if !pullCalled {
		t.Error("expected git pull --rebase to be called")
	}
}
