package agent

import (
	"context"
	"strings"
	"testing"
)

func TestImplement_RendersPromptAndCallsRun(t *testing.T) {
	captured := stubRunner(t)

	result, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t))
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

	result, err := Retry(context.Background(), testIssue(), 7, testConfig(), testPrompts(t), nil, testLogger(t))
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

	cfg := testConfig()
	cfg.AgentTimeout = "5m"

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
}

func TestImplement_InvalidTimeout(t *testing.T) {
	stubRunner(t)

	cfg := testConfig()
	cfg.AgentTimeout = "invalid"

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t))
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestBranchName(t *testing.T) {
	got := BranchName(42, "add-widget-support")
	if got != "42-add-widget-support" {
		t.Errorf("BranchName() = %q, want %q", got, "42-add-widget-support")
	}
}

func TestNewRunOpts_SetsAllFields(t *testing.T) {
	cfg := testConfig()
	env := map[string]string{"FOO": "bar"}
	opts, err := newRunOpts("prompt text", cfg, env)
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	if opts.Prompt != "prompt text" {
		t.Errorf("Prompt = %q, want %q", opts.Prompt, "prompt text")
	}
	if opts.Image != cfg.Docker.Image {
		t.Errorf("Image = %q, want %q", opts.Image, cfg.Docker.Image)
	}
	if opts.Repo != cfg.Repo {
		t.Errorf("Repo = %q, want %q", opts.Repo, cfg.Repo)
	}
	if opts.WorkDir != "/workspace" {
		t.Errorf("WorkDir = %q, want %q", opts.WorkDir, "/workspace")
	}
	if opts.Env["FOO"] != "bar" {
		t.Errorf("Env missing FOO=bar")
	}
}

func TestNewRunOpts_InvalidTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.AgentTimeout = "bad"
	_, err := newRunOpts("prompt", cfg, nil)
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestImplement_BranchExistsDetection(t *testing.T) {
	// Stub Runner (agent call) and GuardRunner (git ls-remote).
	var capturedPrompt string
	stubRunnerFunc(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		// The prompt is the 3rd argument: claude -p --dangerously-skip-permissions <prompt>
		if len(args) >= 3 {
			capturedPrompt = args[2]
		}
		return []byte("ok"), []byte(""), 0, nil
	})

	stubGuardRunner(t, func(name string, args ...string) ([]byte, error) {
		// Simulate branch existing on remote.
		if name == "git" && len(args) > 0 && args[0] == "ls-remote" {
			return []byte("abc123\trefs/heads/42-add-widget-support\n"), nil
		}
		return []byte(""), nil
	})

	// Use a template that includes BranchExists.
	prompts := &Prompts{
		Implementer: "{{if .BranchExists}}EXISTING{{else}}NEW{{end}}",
	}

	_, err := Implement(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if !strings.Contains(capturedPrompt, "EXISTING") {
		t.Errorf("expected BranchExists=true in prompt, got: %s", capturedPrompt)
	}
}

func TestImplement_NonZeroExitSurfacedInResult(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("fail output"), []byte(""), 1, nil
	})

	result, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() should not return error for non-zero exit, got: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}
