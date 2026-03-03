package agent

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// testLogger returns a logger that only emits errors, suitable for tests.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// stubRunner replaces Runner with a stub that returns "ok" and captures the
// command-line arguments. Returns a pointer to the captured args slice.
func stubRunner(t *testing.T) *[]string {
	t.Helper()
	orig := Runner
	t.Cleanup(func() { Runner = orig })

	var captured []string
	Runner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		captured = append([]string{name}, args...)
		return []byte("ok"), []byte(""), 0, nil
	}
	return &captured
}

// stubRunnerFunc replaces Runner with a custom function for tests that need
// to control the return values.
func stubRunnerFunc(t *testing.T, fn func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error)) {
	t.Helper()
	orig := Runner
	t.Cleanup(func() { Runner = orig })
	Runner = fn
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
	}
}
