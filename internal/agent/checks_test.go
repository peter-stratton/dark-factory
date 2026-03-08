package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/config"
)

// stubPollInterval replaces checksPollInterval with a very short duration for
// tests and restores the original on cleanup.
func stubPollInterval(t *testing.T, d time.Duration) {
	t.Helper()
	orig := checksPollInterval
	t.Cleanup(func() { checksPollInterval = orig })
	checksPollInterval = d
}

// checksJSON builds a JSON array of check objects for GuardRunner stubs.
func checksJSON(entries []checkEntry) []byte {
	var parts []string
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf(`{"name":%q,"state":%q,"conclusion":%q}`, e.Name, e.State, e.Conclusion))
	}
	return []byte("[" + strings.Join(parts, ",") + "]")
}

// TestWaitForChecks_AllPass verifies that WaitForChecks returns no failures
// when all required checks report success on the first poll.
func TestWaitForChecks_AllPass(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "COMPLETED", Conclusion: "SUCCESS"},
			{Name: "ci/test", State: "COMPLETED", Conclusion: "SUCCESS"},
		}), nil
	})

	failures, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build", "ci/test"}, time.Minute, testLogger(t))
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

// TestWaitForChecks_CheckFails verifies that WaitForChecks returns a failure
// when a required check completes with a non-success conclusion.
func TestWaitForChecks_CheckFails(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "COMPLETED", Conclusion: "FAILURE"},
		}), nil
	})

	failures, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build"}, time.Minute, testLogger(t))
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(failures))
	}
	if failures[0].Name != "ci/build" {
		t.Errorf("failure.Name = %q, want %q", failures[0].Name, "ci/build")
	}
	if !strings.Contains(failures[0].Output, "FAILURE") {
		t.Errorf("failure.Output = %q, want it to contain FAILURE", failures[0].Output)
	}
}

// TestWaitForChecks_Timeout verifies that WaitForChecks returns an error when
// the timeout expires while checks are still pending.
func TestWaitForChecks_Timeout(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "queued", Conclusion: ""},
		}), nil
	})

	_, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build"}, 5*time.Millisecond, testLogger(t))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %v, want it to contain 'timed out'", err)
	}
}

// TestWaitForChecks_ContextCancelled verifies that WaitForChecks returns an
// error when the context is cancelled while checks are pending.
func TestWaitForChecks_ContextCancelled(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "queued", Conclusion: ""},
		}), nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := WaitForChecks(ctx, "owner/repo", 42,
		[]string{"ci/build"}, time.Minute, testLogger(t))
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

// TestWaitForChecks_UnrequiredCheckFails verifies that WaitForChecks ignores
// failures in checks that are not in the required list.
func TestWaitForChecks_UnrequiredCheckFails(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "COMPLETED", Conclusion: "SUCCESS"},
			{Name: "ci/optional", State: "COMPLETED", Conclusion: "FAILURE"},
		}), nil
	})

	failures, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build"}, time.Minute, testLogger(t))
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures for unrequired check, got %v", failures)
	}
}

// TestWaitForChecks_PendingThenPass verifies that WaitForChecks keeps polling
// until a check transitions from pending to success.
func TestWaitForChecks_PendingThenPass(t *testing.T) {
	stubPollInterval(t, time.Millisecond)

	callCount := 0
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			// First poll: check still pending.
			return checksJSON([]checkEntry{
				{Name: "ci/build", State: "in_progress", Conclusion: ""},
			}), nil
		}
		// Second poll: check passed.
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "COMPLETED", Conclusion: "SUCCESS"},
		}), nil
	})

	failures, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build"}, time.Minute, testLogger(t))
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 polls, got %d", callCount)
	}
}

// TestWaitForChecks_CheckNotPresentInitially verifies that WaitForChecks waits
// when a required check is not yet listed in the response.
func TestWaitForChecks_CheckNotPresentInitially(t *testing.T) {
	stubPollInterval(t, time.Millisecond)

	callCount := 0
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			// First poll: required check not yet present.
			return checksJSON([]checkEntry{}), nil
		}
		// Second poll: check is now present and passed.
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "COMPLETED", Conclusion: "SUCCESS"},
		}), nil
	})

	failures, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build"}, time.Minute, testLogger(t))
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

// TestWaitForChecks_CommandError verifies that WaitForChecks returns an error
// when the gh command fails.
func TestWaitForChecks_CommandError(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("network error")
	})

	_, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build"}, time.Minute, testLogger(t))
	if err == nil {
		t.Fatal("expected error from gh command failure, got nil")
	}
	if !strings.Contains(err.Error(), "polling PR checks") {
		t.Errorf("error = %v, want it to contain 'polling PR checks'", err)
	}
}

// TestWaitForChecks_FailFastOnFailure verifies that WaitForChecks returns
// failures immediately even if other required checks are still pending.
func TestWaitForChecks_FailFastOnFailure(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "COMPLETED", Conclusion: "FAILURE"},
			{Name: "ci/test", State: "in_progress", Conclusion: ""},
		}), nil
	})

	failures, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build", "ci/test"}, time.Minute, testLogger(t))
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}
	// Should return the failure for ci/build even though ci/test is still running.
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d: %v", len(failures), failures)
	}
	if failures[0].Name != "ci/build" {
		t.Errorf("failure.Name = %q, want %q", failures[0].Name, "ci/build")
	}
}

// TestWaitForChecks_NilLogger verifies that WaitForChecks does not panic
// when logger is nil (falls back to slog.Default).
func TestWaitForChecks_NilLogger(t *testing.T) {
	stubPollInterval(t, time.Millisecond)
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return checksJSON([]checkEntry{
			{Name: "ci/build", State: "COMPLETED", Conclusion: "SUCCESS"},
		}), nil
	})

	failures, err := WaitForChecks(context.Background(), "owner/repo", 42,
		[]string{"ci/build"}, time.Minute, nil)
	if err != nil {
		t.Fatalf("WaitForChecks() error = %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("expected no failures, got %v", failures)
	}
}

// TestPollChecks_MalformedJSON verifies that pollChecks returns an error for
// non-JSON response from gh pr checks.
func TestPollChecks_MalformedJSON(t *testing.T) {
	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		return []byte("not json"), nil
	})

	_, _, err := pollChecks("owner/repo", 42, []string{"ci/build"}, testLogger(t))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parsing checks JSON") {
		t.Errorf("error = %v, want 'parsing checks JSON'", err)
	}
}

// TestFormatCheckFailures verifies the formatting of multiple check failures.
func TestFormatCheckFailures(t *testing.T) {
	failures := []CheckFailure{
		{Name: "ci/build", Output: "build failed"},
		{Name: "ci/test", Output: "tests failed"},
	}
	got := formatCheckFailures(failures)
	if !strings.Contains(got, "ci/build") {
		t.Errorf("formatCheckFailures() = %q, want it to contain 'ci/build'", got)
	}
	if !strings.Contains(got, "ci/test") {
		t.Errorf("formatCheckFailures() = %q, want it to contain 'ci/test'", got)
	}
	if !strings.Contains(got, "build failed") {
		t.Errorf("formatCheckFailures() = %q, want it to contain 'build failed'", got)
	}
}

// TestFormatCheckFailures_Single verifies formatting with a single failure.
func TestFormatCheckFailures_Single(t *testing.T) {
	failures := []CheckFailure{
		{Name: "ci/lint", Output: "lint errors found"},
	}
	got := formatCheckFailures(failures)
	if !strings.Contains(got, "ci/lint") {
		t.Errorf("formatCheckFailures() = %q, want it to contain 'ci/lint'", got)
	}
	if !strings.Contains(got, "lint errors found") {
		t.Errorf("formatCheckFailures() = %q, want it to contain 'lint errors found'", got)
	}
}

// TestProcessIssue_WaitForChecks_AllPass verifies that ProcessIssue merges
// the PR when all required CI checks pass.
func TestProcessIssue_WaitForChecks_AllPass(t *testing.T) {
	stubPollInterval(t, time.Millisecond)

	cfg := loopConfig()
	cfg.WaitForChecks = &config.WaitForChecks{
		Timeout:  "1m",
		Required: []string{"ci/build"},
	}

	// Agent outputs: implementer, quality(APPROVED), reviewer(APPROVED)
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
		if name == "gh" && strings.Contains(joined, "pr checks") {
			return checksJSON([]checkEntry{
				{Name: "ci/build", State: "COMPLETED", Conclusion: "SUCCESS"},
			}), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
}

// TestProcessIssue_WaitForChecks_Nil verifies that ProcessIssue merges
// immediately when WaitForChecks is nil (current behavior preserved).
func TestProcessIssue_WaitForChecks_Nil(t *testing.T) {
	cfg := loopConfig()
	// cfg.WaitForChecks is nil by default.

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
		// gh pr checks should never be called.
		if name == "gh" && strings.Contains(joined, "pr checks") {
			t.Error("unexpected gh pr checks call when WaitForChecks is nil")
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
}

// TestProcessIssue_WaitForChecks_Timeout verifies that ProcessIssue fails
// the issue when CI checks time out.
func TestProcessIssue_WaitForChecks_Timeout(t *testing.T) {
	stubPollInterval(t, time.Millisecond)

	cfg := loopConfig()
	cfg.WaitForChecks = &config.WaitForChecks{
		Timeout:  "5ms",
		Required: []string{"ci/build"},
	}

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
		if name == "gh" && strings.Contains(joined, "pr checks") {
			// Always return pending to trigger timeout.
			return checksJSON([]checkEntry{
				{Name: "ci/build", State: "queued", Conclusion: ""},
			}), nil
		}
		return []byte(""), nil
	})

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, testPrompts(t), nil, testLogger(t), nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want %q", outcome.Status, "failed")
	}
	if outcome.Err == nil || !strings.Contains(outcome.Err.Error(), "waiting for CI checks") {
		t.Errorf("Err = %v, want it to contain 'waiting for CI checks'", outcome.Err)
	}
}

// TestProcessIssue_WaitForChecks_FixSucceeds verifies that ProcessIssue
// triggers a fix cycle when a CI check fails, and merges after the fix.
func TestProcessIssue_WaitForChecks_FixSucceeds(t *testing.T) {
	stubPollInterval(t, time.Millisecond)

	cfg := loopConfig()
	cfg.WaitForChecks = &config.WaitForChecks{
		Timeout:  "1m",
		Required: []string{"ci/build"},
	}
	cfg.Verify.MaxFixAttempts = 2

	checksCallCount := 0
	// Agent outputs: implementer, quality(APPROVED), reviewer(APPROVED), verify-fix agent
	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"reviewer output\nREVIEW_RESULT=APPROVED\n",
		"verify-fix output", // CI fix agent
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
		if name == "gh" && strings.Contains(joined, "pr checks") {
			checksCallCount++
			if checksCallCount == 1 {
				// First poll: check fails.
				return checksJSON([]checkEntry{
					{Name: "ci/build", State: "COMPLETED", Conclusion: "FAILURE"},
				}), nil
			}
			// Second poll (after fix): check passes.
			return checksJSON([]checkEntry{
				{Name: "ci/build", State: "COMPLETED", Conclusion: "SUCCESS"},
			}), nil
		}
		return []byte(""), nil
	})

	prompts := testPrompts(t)
	prompts.VerifyFix = "Fix CI: {{.VerifyErrors}}"

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, prompts, nil, testLogger(t), nil)

	if outcome.Status != "implemented" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "implemented", outcome.Err)
	}
	if checksCallCount < 2 {
		t.Errorf("expected at least 2 gh pr checks calls, got %d", checksCallCount)
	}
}

// TestProcessIssue_WaitForChecks_FixExhausted verifies that ProcessIssue fails
// the issue when CI fix attempts are exhausted.
func TestProcessIssue_WaitForChecks_FixExhausted(t *testing.T) {
	stubPollInterval(t, time.Millisecond)

	cfg := loopConfig()
	cfg.WaitForChecks = &config.WaitForChecks{
		Timeout:  "1m",
		Required: []string{"ci/build"},
	}
	cfg.Verify.MaxFixAttempts = 1

	// Agent outputs: implementer, quality(APPROVED), reviewer(APPROVED), one verify-fix attempt
	setupLoopTest(t, []string{
		"implementer output",
		"QUALITY_RESULT=APPROVED",
		"reviewer output\nREVIEW_RESULT=APPROVED\n",
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
		if name == "gh" && strings.Contains(joined, "pr checks") {
			// Always fail.
			return checksJSON([]checkEntry{
				{Name: "ci/build", State: "COMPLETED", Conclusion: "FAILURE"},
			}), nil
		}
		return []byte(""), nil
	})

	prompts := testPrompts(t)
	prompts.VerifyFix = "Fix CI: {{.VerifyErrors}}"

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, prompts, nil, testLogger(t), nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "failed", outcome.Err)
	}
	if outcome.Err == nil || !strings.Contains(outcome.Err.Error(), "CI checks failed") {
		t.Errorf("Err = %v, want it to contain 'CI checks failed'", outcome.Err)
	}
}

// TestProcessIssue_WaitForChecks_NoVerifyFix verifies that ProcessIssue fails
// immediately when CI checks fail and no VerifyFix prompt is configured.
func TestProcessIssue_WaitForChecks_NoVerifyFix(t *testing.T) {
	stubPollInterval(t, time.Millisecond)

	cfg := loopConfig()
	cfg.WaitForChecks = &config.WaitForChecks{
		Timeout:  "1m",
		Required: []string{"ci/build"},
	}
	cfg.Verify.MaxFixAttempts = 2

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
		if name == "gh" && strings.Contains(joined, "pr checks") {
			return checksJSON([]checkEntry{
				{Name: "ci/build", State: "COMPLETED", Conclusion: "FAILURE"},
			}), nil
		}
		return []byte(""), nil
	})

	// No VerifyFix prompt configured.
	prompts := testPrompts(t)

	outcome := ProcessIssue(context.Background(), loopIssue(), cfg, prompts, nil, testLogger(t), nil)

	if outcome.Status != "failed" {
		t.Errorf("Status = %q, want %q (err: %v)", outcome.Status, "failed", outcome.Err)
	}
	if outcome.Err == nil || !strings.Contains(outcome.Err.Error(), "CI checks failed") {
		t.Errorf("Err = %v, want it to contain 'CI checks failed'", outcome.Err)
	}
}
