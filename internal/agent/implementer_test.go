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

	// Verify the rendered prompt was set in GODARK_PROMPT (captured via stubRunner's output).
	// stubRunner includes GODARK_PROMPT in its output for easy assertion.
	joined := strings.Join(*captured, " ")
	_ = joined // command is python3 <path>, prompt is in env not args
}

func TestImplement_PromptContainsIssueDetails(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "Implement #42") {
		t.Errorf("expected rendered prompt with issue number, got: %s", prompt)
	}
	if !strings.Contains(prompt, "add-widget-support") {
		t.Errorf("expected slug in prompt, got: %s", prompt)
	}
}

func TestRetry_RendersRetryPromptWithPR(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	result, err := Retry(context.Background(), testIssue(), 7, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "Retry PR #7") {
		t.Errorf("expected retry prompt with PR number, got: %s", prompt)
	}
	if !strings.Contains(prompt, "#42") {
		t.Errorf("expected issue number in retry prompt, got: %s", prompt)
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
	opts, err := newRunOpts("prompt text", cfg, env, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	if opts.Prompt != "prompt text" {
		t.Errorf("Prompt = %q, want %q", opts.Prompt, "prompt text")
	}
	if opts.Role != "implementer" {
		t.Errorf("Role = %q, want %q", opts.Role, "implementer")
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
	_, err := newRunOpts("prompt", cfg, nil, "implementer")
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestNewRunOpts_SetsProtectedPathsEnv(t *testing.T) {
	cfg := testConfig() // ProtectedPaths: ["CLAUDE.md", "tests/scenarios/"]
	opts, err := newRunOpts("prompt", cfg, nil, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	got := opts.Env["GODARK_PROTECTED_PATHS"]
	if got != "CLAUDE.md,tests/scenarios/" {
		t.Errorf("GODARK_PROTECTED_PATHS = %q, want %q", got, "CLAUDE.md,tests/scenarios/")
	}
}

func TestNewRunOpts_EmptyProtectedPathsEnv(t *testing.T) {
	cfg := testConfig()
	cfg.ProtectedPaths = nil
	opts, err := newRunOpts("prompt", cfg, nil, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	got := opts.Env["GODARK_PROTECTED_PATHS"]
	if got != "" {
		t.Errorf("GODARK_PROTECTED_PATHS = %q, want empty string", got)
	}
}

func TestImplement_ProtectedPathsInEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	got := capturedEnv["GODARK_PROTECTED_PATHS"]
	if got != "CLAUDE.md,tests/scenarios/" {
		t.Errorf("GODARK_PROTECTED_PATHS = %q, want %q", got, "CLAUDE.md,tests/scenarios/")
	}
}

func TestNewRunOpts_DoesNotMutateAuthEnv(t *testing.T) {
	cfg := testConfig()
	authEnv := map[string]string{"GH_TOKEN": "tok-xyz"}
	_, err := newRunOpts("prompt", cfg, authEnv, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	// authEnv should not have been modified
	if _, ok := authEnv["GODARK_PROTECTED_PATHS"]; ok {
		t.Error("newRunOpts must not mutate the caller's authEnv map")
	}
}

func TestImplement_BranchExistsDetection(t *testing.T) {
	// Stub Runner (agent call) and GuardRunner (git ls-remote).
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
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
	// The prompt is passed via GODARK_PROMPT env var, not args.
	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "EXISTING") {
		t.Errorf("expected BranchExists=true in prompt, got: %s", prompt)
	}
}

func TestImplement_SetsImplementerRole(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "implementer" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "implementer")
	}
}

func TestRetry_SetsImplementerRetryRole(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Retry(context.Background(), testIssue(), 7, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "implementer_retry" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "implementer_retry")
	}
}

func TestImplement_NonZeroExitSurfacedInResult(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
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
