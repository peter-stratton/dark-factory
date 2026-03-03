package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

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

func testIssue() github.Issue {
	return github.Issue{
		Number: 42,
		Title:  "Add Widget Support",
		Body:   "Implement widgets for the dashboard.",
	}
}

func testImplementerConfig() *config.Config {
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

func testPrompts(t *testing.T) *Prompts {
	t.Helper()
	return &Prompts{
		Implementer:      "Implement #{{.IssueNumber}} {{.IssueTitle}} repo={{.Repo}} slug={{.Slug}}",
		ImplementerRetry: "Retry PR #{{.PRNumber}} for #{{.IssueNumber}} repo={{.Repo}}",
		Reviewer:         "Review PR #{{.PRNumber}} for #{{.IssueNumber}}",
	}
}

func TestImplement_RendersPromptAndCallsRun(t *testing.T) {
	captured := stubRunner(t)

	result, err := Implement(context.Background(), testIssue(), testImplementerConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}

	// Verify the rendered prompt was passed to claude
	joined := strings.Join(*captured, " ")
	if !strings.Contains(joined, "Implement #42") {
		t.Errorf("expected rendered prompt with issue number, got: %s", joined)
	}
	if !strings.Contains(joined, "add-widget-support") {
		t.Errorf("expected slug in prompt, got: %s", joined)
	}
}

func TestRetry_RendersRetryPromptWithPR(t *testing.T) {
	captured := stubRunner(t)

	result, err := Retry(context.Background(), testIssue(), 7, testImplementerConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}

	joined := strings.Join(*captured, " ")
	if !strings.Contains(joined, "Retry PR #7") {
		t.Errorf("expected retry prompt with PR number, got: %s", joined)
	}
	if !strings.Contains(joined, "#42") {
		t.Errorf("expected issue number in retry prompt, got: %s", joined)
	}
}

func TestImplement_AgentTimeoutParsed(t *testing.T) {
	stubRunner(t)

	cfg := testImplementerConfig()
	cfg.AgentTimeout = "5m"

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
}

func TestImplement_InvalidTimeout(t *testing.T) {
	stubRunner(t)

	cfg := testImplementerConfig()
	cfg.AgentTimeout = "invalid"

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t))
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestImplement_NonZeroExitSurfacedInResult(t *testing.T) {
	orig := Runner
	t.Cleanup(func() { Runner = orig })

	Runner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("fail output"), []byte(""), 1, nil
	}

	result, err := Implement(context.Background(), testIssue(), testImplementerConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() should not return error for non-zero exit, got: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}
