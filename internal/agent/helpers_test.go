package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"syscall"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
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

// stubRunner replaces Runner with a stub that returns a minimal JSON result and
// captures the command-line arguments. Returns a pointer to the captured args slice.
func stubRunner(t *testing.T) *[]string {
	t.Helper()
	orig := Runner
	t.Cleanup(func() { Runner = orig })

	var captured []string
	Runner = func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, *syscall.Rusage, error) {
		captured = append([]string{name}, args...)
		// Return a valid final JSON line so parseRunnerOutput succeeds.
		out := env["GODARK_PROMPT"] + "\n" + `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil, nil
	}
	return &captured
}

// stubRunnerFunc replaces Runner with a custom function for tests that need
// to control the return values.
func stubRunnerFunc(t *testing.T, fn func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, *syscall.Rusage, error)) {
	t.Helper()
	orig := Runner
	t.Cleanup(func() { Runner = orig })
	Runner = fn
}

// stubGOOS overrides goosForRusage for the duration of the test, allowing
// both macOS and Linux normalization paths to be exercised on any platform.
func stubGOOS(t *testing.T, goos string) {
	t.Helper()
	orig := goosForRusage
	t.Cleanup(func() { goosForRusage = orig })
	goosForRusage = goos
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
		NoSandbox:      true,
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
