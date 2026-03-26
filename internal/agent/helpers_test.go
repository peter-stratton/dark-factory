package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

// wrapRunnerJSON wraps agent text output in the ndjson final result line
// that parseRunnerOutput expects from agent_runner.py.
func wrapRunnerJSON(text string) string {
	final := runnerFinalResult{Result: text}
	b, _ := json.Marshal(final)
	return string(b)
}

// wrapRunnerJSONWithTrace wraps agent text output with a tool trace in the
// ndjson final result line that parseRunnerOutput expects.
func wrapRunnerJSONWithTrace(text string, trace []string) string {
	final := runnerFinalResult{Result: text, ToolTrace: trace}
	b, _ := json.Marshal(final)
	return string(b)
}

// testLogger returns a logger that only emits errors, suitable for tests.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// stubSandboxRunner replaces SandboxRunner with a stub that returns a minimal
// JSON result. Suitable for tests that only need Run() to succeed.
func stubSandboxRunner(t *testing.T) {
	t.Helper()
	orig := SandboxRunner
	t.Cleanup(func() { SandboxRunner = orig })
	SandboxRunner = func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := opts.Env["GODARK_PROMPT"] + "\n" + `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	}
}

// stubSandboxRunnerFunc replaces SandboxRunner with a custom function for
// tests that need to control the return values or capture opts.
func stubSandboxRunnerFunc(t *testing.T, fn func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error)) {
	t.Helper()
	orig := SandboxRunner
	t.Cleanup(func() { SandboxRunner = orig })
	SandboxRunner = fn
}

// stubGuardRunner replaces GuardRunner with the given function for the
// duration of the test.
func stubGuardRunner(t *testing.T, fn func(string, ...string) ([]byte, error)) {
	t.Helper()
	orig := GuardRunner
	t.Cleanup(func() { GuardRunner = orig })
	GuardRunner = fn
}

// testIssue returns a sample issue for tests.
func testIssue() github.Issue {
	return github.Issue{
		Number: 42,
		Title:  "Add Widget Support",
		Body:   "Implement widgets for the dashboard.",
	}
}

// testConfig returns a standard config for agent tests.
func testConfig() *config.Config {
	return &config.Config{
		Repo:           "owner/repo",
		AgentTimeout:   "10m",
		BuildCommand:   "go build ./...",
		TestCommand:    "go test ./...",
		ProtectedPaths: []string{"CLAUDE.md", "tests/scenarios/"},
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
	}
}

// testPrompts returns minimal prompt templates for tests.
func testPrompts(t *testing.T) *Prompts {
	t.Helper()
	return &Prompts{
		Implementer:      "Implement #{{.IssueNumber}} {{.IssueTitle}} repo={{.Repo}} slug={{.Slug}}",
		ImplementerRetry: "Retry PR #{{.PRNumber}} for #{{.IssueNumber}} repo={{.Repo}}",
		Reviewer:         "Review PR #{{.PRNumber}} for #{{.IssueNumber}}",
		QualityReviewer:  "Quality review PR #{{.PRNumber}} for #{{.IssueNumber}}",
	}
}
