package agent

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
	gsandbox "github.com/peter-stratton/dark-factory/internal/sandbox"
)

// hostMutatingGitVerbs lists git subcommands that, when run on the host,
// would corrupt shared state across concurrent ProcessIssueWithMode goroutines
// in the orchestrator's wave dispatcher. Read-only verbs (rev-parse, log,
// status, ls-files, show) are intentionally excluded.
//
// Keep this list aligned with the host-isolation invariant comment at the
// top of loop.go. If you genuinely need to mutate the host tree from a path
// reachable from ProcessIssueWithMode, you must first relax this invariant
// (and revisit the orchestrator's wave dispatcher's serialization
// assumptions).
var hostMutatingGitVerbs = []string{
	"checkout",
	"reset",
	"pull",
	"merge",
	"rebase",
	"commit",
	"push",
	"stash",
	"clean",
	"branch",
	"switch",
	"add",
	"restore",
	"apply",
	"cherry-pick",
}

// recordingGuardCall is one captured invocation of GuardRunner from a test.
type recordingGuardCall struct {
	name string
	args []string
}

// isHostMutatingGitCall returns true when the call would mutate the host
// repository's working tree, index, or HEAD. Calls to `gh` are always
// considered safe (gh is a remote API client and does not touch local git
// state). Calls to `git` are inspected by their first positional argument.
func isHostMutatingGitCall(c recordingGuardCall) bool {
	if c.name != "git" {
		return false
	}
	if len(c.args) == 0 {
		return false
	}
	verb := c.args[0]
	for _, banned := range hostMutatingGitVerbs {
		if verb == banned {
			return true
		}
	}
	return false
}

// TestProcessIssueWithMode_HostIsolation runs two ProcessIssueWithMode
// goroutines concurrently with deferMerge=true and asserts that no
// GuardRunner call from either goroutine mutated the host working tree.
//
// This test is the runtime tripwire for the host-isolation invariant
// documented at the top of loop.go. Together with the doc block, it locks
// in the assumption the orchestrator's wave dispatcher relies on:
// concurrent ProcessIssueWithMode goroutines never touch each other's
// state because they never touch the host tree at all.
//
// Run with `go test -race ./internal/agent/` to also catch any data race
// in this package's stubs (the recording slice is mutex-protected
// specifically so this test stays clean under -race).
func TestProcessIssueWithMode_HostIsolation(t *testing.T) {
	var (
		mu       sync.Mutex
		recorded []recordingGuardCall
	)

	origGuard := GuardRunner
	origSandboxRunner := SandboxRunner
	origSandboxRunContainer := sandboxRunContainer
	origGHRunner := github.CommandRunner
	t.Cleanup(func() {
		GuardRunner = origGuard
		SandboxRunner = origSandboxRunner
		sandboxRunContainer = origSandboxRunContainer
		github.CommandRunner = origGHRunner
	})

	// Recording GuardRunner: synthesize the same minimal happy-path
	// responses the existing TestProcessIssue_ImplementedOnFirstApproval
	// uses, but record every call so the test can scan for forbidden
	// host-mutating git verbs after the goroutines finish.
	GuardRunner = func(name string, args ...string) ([]byte, error) {
		mu.Lock()
		recorded = append(recorded, recordingGuardCall{name: name, args: append([]string(nil), args...)})
		mu.Unlock()

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

	// Stub the github package's runner the same way: AddIssueLabel and
	// friends route through it. Returning empty output is enough for the
	// happy path; we just need to ensure no real shell exec happens.
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	// Sandboxed agent runs return a verdict-shaped JSON. Two goroutines
	// will race through here; SandboxRunner is shared state, but the
	// implementation only constructs the response and returns it, so it's
	// safe to call concurrently.
	var sandboxIdx atomic.Int32
	verdicts := []string{
		"implementer output",
		"AGENT_RESULT=APPROVED",
		"reviewer output\nAGENT_RESULT=APPROVED\n",
	}
	SandboxRunner = func(_ context.Context, _ gsandbox.RunOpts, _ *slog.Logger) (*gsandbox.RunResult, error) {
		idx := int(sandboxIdx.Add(1)-1) % len(verdicts)
		return &gsandbox.RunResult{Stdout: wrapRunnerJSON(verdicts[idx])}, nil
	}
	sandboxRunContainer = func(_ context.Context, _ gsandbox.RunOpts, _ *slog.Logger) (*gsandbox.RunResult, error) {
		return &gsandbox.RunResult{ExitCode: 0}, nil
	}

	cfg := &config.Config{
		Repo:           "owner/repo",
		Docker:         config.Docker{Image: "test-image:latest"},
		AutoMerge:      config.AutoMerge{Feature: "all", Rollup: "none"},
		MaxRetries:     2,
		AgentTimeout:   "10m",
		ProtectedPaths: []string{"CLAUDE.md"},
		ScenarioDir:    "/nonexistent-scenario-dir",
		ReviewDir:      "tests/review/",
	}

	issues := []github.Issue{
		{Number: 5, Title: "Issue Five", Body: "body 5"},
		{Number: 6, Title: "Issue Six", Body: "body 6"},
	}

	var wg sync.WaitGroup
	outcomes := make([]IssueOutcome, len(issues))
	for i := range issues {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			outcomes[idx] = ProcessIssueWithMode(
				context.Background(),
				issues[idx],
				cfg,
				testPrompts(t),
				nil,
				testLogger(t),
				nil,
				nil,
				false,
				true, // deferMerge
			)
		}(i)
	}
	wg.Wait()

	// At least one outcome should reach the approval gate. We don't assert
	// that both outcomes reach it because the verdict stub is shared and
	// the second goroutine may consume an out-of-order response — that's
	// fine for this test, which only cares about host isolation, not
	// merge correctness.
	approvedCount := 0
	for _, oc := range outcomes {
		if oc.Status == StatusApprovedReadyForMerge {
			approvedCount++
		}
	}
	if approvedCount == 0 {
		t.Logf("outcomes: %+v", outcomes)
		t.Fatalf("expected at least one outcome to reach StatusApprovedReadyForMerge, got 0; the test stub may have drifted from ProcessIssueWithMode's call sequence")
	}

	// The actual assertion: scan every recorded GuardRunner call and fail
	// loudly if any of them would have mutated the host repository.
	mu.Lock()
	defer mu.Unlock()
	if len(recorded) == 0 {
		t.Fatalf("recorded zero GuardRunner calls; the assertion below would pass vacuously, the test is broken")
	}
	for _, c := range recorded {
		if isHostMutatingGitCall(c) {
			t.Errorf("host-isolation invariant violated: GuardRunner was called with %q %v, which mutates the host working tree. See the doc block at the top of loop.go before relaxing this invariant.", c.name, c.args)
		}
	}
}
