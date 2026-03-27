package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestImplement_RendersPromptAndCallsRun(t *testing.T) {
	stubSandboxRunner(t)

	result, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestImplement_PromptContainsIssueDetails(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t), "")
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
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	result, err := Retry(context.Background(), testIssue(), 7, "", "", "", testConfig(), testPrompts(t), nil, testLogger(t))
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

func TestRetry_WithSessionID_DoesNotSetGODARK_SESSION_ID(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"sess-new","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	_, err := Retry(context.Background(), testIssue(), 7, "sess-abc123", "", "", testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("GODARK_SESSION_ID should not be set (session resume disabled), got %q", capturedEnv["GODARK_SESSION_ID"])
	}
}

func TestRetry_WithoutSessionID_NoSessionEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	_, err := Retry(context.Background(), testIssue(), 7, "", "", "", testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("GODARK_SESSION_ID should not be set when prevSessionID is empty, got %q", capturedEnv["GODARK_SESSION_ID"])
	}
}

func TestRetry_WithHandoffContext_SkipsSessionID(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"sess-new","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	// Even with a prevSessionID, handoffContext non-empty means fresh session.
	_, err := Retry(context.Background(), testIssue(), 7, "sess-abc123", "", "Prior attempt context.", testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("GODARK_SESSION_ID must not be set when handoffContext is non-empty, got %q", capturedEnv["GODARK_SESSION_ID"])
	}
}

func TestRetry_WithHandoffContext_RendersHandoffInPrompt(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	handoff := "## Implementation Notes\nTried X but it failed."
	prompts := &Prompts{
		ImplementerRetry: `PR #{{.PRNumber}}{{if .HandoffContext}} FRESH: {{.HandoffContext}}{{end}}`,
	}

	_, err := Retry(context.Background(), testIssue(), 7, "", "", handoff, testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "FRESH:") {
		t.Errorf("expected FRESH: in rendered prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, handoff) {
		t.Errorf("expected handoff text in rendered prompt, got: %s", prompt)
	}
}

func TestRetry_WithEmptyHandoffContext_NoFreshBlock(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{
		ImplementerRetry: `PR #{{.PRNumber}}{{if .HandoffContext}} FRESH: {{.HandoffContext}}{{end}}`,
	}

	_, err := Retry(context.Background(), testIssue(), 7, "", "", "", testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if strings.Contains(prompt, "FRESH:") {
		t.Errorf("expected no FRESH: block when HandoffContext is empty, got: %s", prompt)
	}
}

func TestImplement_DoesNotSetSessionIDEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("Implement should not set GODARK_SESSION_ID, got %q", capturedEnv["GODARK_SESSION_ID"])
	}
}

func TestImplement_AgentTimeoutParsed(t *testing.T) {
	stubSandboxRunner(t)

	cfg := testConfig()
	cfg.AgentTimeout = "5m"

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
}

func TestImplement_InvalidTimeout(t *testing.T) {
	stubSandboxRunner(t)

	cfg := testConfig()
	cfg.AgentTimeout = "invalid"

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t), "")
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
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	got := capturedEnv["GODARK_PROTECTED_PATHS"]
	if got != "CLAUDE.md,tests/scenarios/" {
		t.Errorf("GODARK_PROTECTED_PATHS = %q, want %q", got, "CLAUDE.md,tests/scenarios/")
	}
}

func TestNewRunOpts_SetsDeniedCommandsEnv(t *testing.T) {
	cfg := testConfig()
	cfg.DeniedCommands = []string{"rm -rf", "git push --force"}
	opts, err := newRunOpts("prompt", cfg, nil, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	got := opts.Env["GODARK_DENIED_COMMANDS"]
	if got != "rm -rf,git push --force" {
		t.Errorf("GODARK_DENIED_COMMANDS = %q, want %q", got, "rm -rf,git push --force")
	}
}

func TestNewRunOpts_EmptyDeniedCommandsEnv(t *testing.T) {
	cfg := testConfig()
	cfg.DeniedCommands = nil
	opts, err := newRunOpts("prompt", cfg, nil, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	got := opts.Env["GODARK_DENIED_COMMANDS"]
	if got != "" {
		t.Errorf("GODARK_DENIED_COMMANDS = %q, want empty string", got)
	}
}

func TestImplement_DeniedCommandsInEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	cfg := testConfig()
	cfg.DeniedCommands = []string{"rm -rf", "git reset --hard"}

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	got := capturedEnv["GODARK_DENIED_COMMANDS"]
	if got != "rm -rf,git reset --hard" {
		t.Errorf("GODARK_DENIED_COMMANDS = %q, want %q", got, "rm -rf,git reset --hard")
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
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
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

	_, err := Implement(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t), "")
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
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "implementer" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "implementer")
	}
}

func TestRetry_SetsImplementerRetryRole(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	_, err := Retry(context.Background(), testIssue(), 7, "", "", "", testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "implementer_retry" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "implementer_retry")
	}
}

func TestImplement_NonZeroExitSurfacedInResult(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{Stdout: "fail output", ExitCode: 1}, nil
	})

	result, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() should not return error for non-zero exit, got: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}

func TestNewPromptData_ArchitectureDocFileExists(t *testing.T) {
	dir := t.TempDir()
	archPath := filepath.Join(dir, "architecture.md")
	content := "# Architecture\nSome content here."
	if err := os.WriteFile(archPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.ArchitectureDoc = archPath
	cfg.ConventionsDoc = filepath.Join(dir, "nonexistent.md")

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.ArchitectureDocContent != content {
		t.Errorf("ArchitectureDocContent = %q, want %q", data.ArchitectureDocContent, content)
	}
}

func TestNewPromptData_ArchitectureDocFileMissing(t *testing.T) {
	cfg := testConfig()
	cfg.ArchitectureDoc = "/nonexistent/path/architecture.md"

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.ArchitectureDocContent != "" {
		t.Errorf("ArchitectureDocContent = %q, want empty string for missing file", data.ArchitectureDocContent)
	}
}

func TestNewPromptData_BothFilesExist(t *testing.T) {
	dir := t.TempDir()
	archPath := filepath.Join(dir, "architecture.md")
	convPath := filepath.Join(dir, "conventions.md")
	archContent := "# Architecture"
	convContent := "# Conventions"

	if err := os.WriteFile(archPath, []byte(archContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(convPath, []byte(convContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.ArchitectureDoc = archPath
	cfg.ConventionsDoc = convPath

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.ArchitectureDocContent != archContent {
		t.Errorf("ArchitectureDocContent = %q, want %q", data.ArchitectureDocContent, archContent)
	}
	if data.ConventionsDocContent != convContent {
		t.Errorf("ConventionsDocContent = %q, want %q", data.ConventionsDocContent, convContent)
	}
}

func TestNewPromptData_ConventionsDocFileMissing(t *testing.T) {
	cfg := testConfig()
	cfg.ConventionsDoc = "/nonexistent/path/conventions.md"

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.ConventionsDocContent != "" {
		t.Errorf("ConventionsDocContent = %q, want empty string for missing file", data.ConventionsDocContent)
	}
}

func TestNewPromptData_ArchitectureJSONFileExists(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "architecture.json")
	content := `{"layers":[{"name":"foundation"}]}`
	if err := os.WriteFile(jsonPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.ArchitectureJSON = jsonPath

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.ArchitectureJSON != content {
		t.Errorf("ArchitectureJSON = %q, want %q", data.ArchitectureJSON, content)
	}
}

func TestNewPromptData_ArchitectureJSONFileMissing(t *testing.T) {
	cfg := testConfig()
	cfg.ArchitectureJSON = "/nonexistent/path/architecture.json"

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.ArchitectureJSON != "" {
		t.Errorf("ArchitectureJSON = %q, want empty string for missing file", data.ArchitectureJSON)
	}
}

func TestNewPromptData_EnforceArchitectureFromConfig(t *testing.T) {
	cfg := testConfig()
	cfg.EnforceArchitecture = true

	data := newPromptData(testIssue(), cfg, "test-slug")

	if !data.EnforceArchitecture {
		t.Error("EnforceArchitecture should be true when set in config")
	}
}

func TestNewPromptData_EnforceArchitectureDefaultFalse(t *testing.T) {
	cfg := testConfig()
	// EnforceArchitecture not set — zero value is false.

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.EnforceArchitecture {
		t.Error("EnforceArchitecture should be false when not set in config")
	}
}

func TestVerifyFix_RendersPromptWithErrors(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{
		VerifyFix: "PR #{{.PRNumber}} issue #{{.IssueNumber}} errors: {{.VerifyErrors}}",
	}
	verifyErrors := "=== build (exit code 1) ===\nbuild failed\n"

	_, err := VerifyFix(context.Background(), testIssue(), 7, verifyErrors, "", testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("VerifyFix() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "PR #7") {
		t.Errorf("expected PR number in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "#42") {
		t.Errorf("expected issue number in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "build failed") {
		t.Errorf("expected verify errors in prompt, got: %s", prompt)
	}
}

func TestVerifyFix_WithSessionID_DoesNotSetGODARK_SESSION_ID(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"sess-new","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{VerifyFix: "fix prompt"}
	_, err := VerifyFix(context.Background(), testIssue(), 7, "some errors", "sess-abc123", testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("VerifyFix() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("GODARK_SESSION_ID should not be set (session resume disabled), got %q", capturedEnv["GODARK_SESSION_ID"])
	}
}

func TestVerifyFix_WithoutSessionID_NoSessionEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{VerifyFix: "fix prompt"}
	_, err := VerifyFix(context.Background(), testIssue(), 7, "some errors", "", testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("VerifyFix() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("GODARK_SESSION_ID should not be set when prevSessionID is empty, got %q", capturedEnv["GODARK_SESSION_ID"])
	}
}

func TestVerifyFix_SetsImplementerRetryRole(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{VerifyFix: "fix prompt"}
	_, err := VerifyFix(context.Background(), testIssue(), 7, "errors", "", testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("VerifyFix() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "implementer_retry" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "implementer_retry")
	}
}

func TestNewRunOpts_SetsGeneratedPathsEnv(t *testing.T) {
	cfg := testConfig()
	cfg.GeneratedPaths = []string{"service/api/grpc/gen/", "**/*.freezed.dart"}
	opts, err := newRunOpts("prompt", cfg, nil, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	got := opts.Env["GODARK_GENERATED_PATHS"]
	if got != "service/api/grpc/gen/,**/*.freezed.dart" {
		t.Errorf("GODARK_GENERATED_PATHS = %q, want %q", got, "service/api/grpc/gen/,**/*.freezed.dart")
	}
}

func TestNewRunOpts_EmptyGeneratedPathsEnv(t *testing.T) {
	cfg := testConfig()
	cfg.GeneratedPaths = nil
	opts, err := newRunOpts("prompt", cfg, nil, "implementer")
	if err != nil {
		t.Fatalf("newRunOpts() error = %v", err)
	}
	got := opts.Env["GODARK_GENERATED_PATHS"]
	if got != "" {
		t.Errorf("GODARK_GENERATED_PATHS = %q, want empty string", got)
	}
}

func TestImplement_GeneratedPathsInEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	cfg := testConfig()
	cfg.GeneratedPaths = []string{"gen/", "**/*.pb.go"}

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	got := capturedEnv["GODARK_GENERATED_PATHS"]
	if got != "gen/,**/*.pb.go" {
		t.Errorf("GODARK_GENERATED_PATHS = %q, want %q", got, "gen/,**/*.pb.go")
	}
}

func TestNewPromptData_GeneratedPathsJoined(t *testing.T) {
	cfg := testConfig()
	cfg.GeneratedPaths = []string{"service/api/grpc/gen/", "**/*.freezed.dart"}

	data := newPromptData(testIssue(), cfg, "test-slug")

	want := "service/api/grpc/gen/, **/*.freezed.dart"
	if data.GeneratedPaths != want {
		t.Errorf("GeneratedPaths = %q, want %q", data.GeneratedPaths, want)
	}
}

func TestNewPromptData_EmptyGeneratedPaths(t *testing.T) {
	cfg := testConfig()
	cfg.GeneratedPaths = nil

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.GeneratedPaths != "" {
		t.Errorf("GeneratedPaths = %q, want empty string", data.GeneratedPaths)
	}
}

func TestNewPromptData_SharedRulesContainsProtectedPaths(t *testing.T) {
	cfg := testConfig() // ProtectedPaths: ["CLAUDE.md", "tests/scenarios/"]
	cfg.ScenarioDir = ""

	data := newPromptData(testIssue(), cfg, "test-slug")

	want := "CLAUDE.md, tests/scenarios/"
	if !strings.Contains(data.SharedRules, want) {
		t.Errorf("SharedRules = %q, want it to contain %q", data.SharedRules, want)
	}
}

func TestNewPromptData_SharedRulesContainsScenarioDir(t *testing.T) {
	cfg := testConfig() // ScenarioDir: "tests/scenarios/"
	cfg.ProtectedPaths = nil

	data := newPromptData(testIssue(), cfg, "test-slug")

	want := "tests/scenarios/"
	if !strings.Contains(data.SharedRules, want) {
		t.Errorf("SharedRules = %q, want it to contain %q", data.SharedRules, want)
	}
}

func TestNewPromptData_SharedRulesEmptyWhenNoPaths(t *testing.T) {
	cfg := testConfig()
	cfg.ProtectedPaths = nil
	cfg.ScenarioDir = ""

	data := newPromptData(testIssue(), cfg, "test-slug")

	if data.SharedRules != "" {
		t.Errorf("SharedRules = %q, want empty string when both ProtectedPaths and ScenarioDir are empty", data.SharedRules)
	}
}

func TestBuildSharedRules_BothSet(t *testing.T) {
	got := buildSharedRules("CLAUDE.md, tests/scenarios/", "tests/scenarios/")
	if !strings.Contains(got, "- Do NOT modify any protected paths: CLAUDE.md, tests/scenarios/") {
		t.Errorf("buildSharedRules() missing protected paths line, got: %q", got)
	}
	if !strings.Contains(got, "- Do NOT modify files in tests/scenarios/") {
		t.Errorf("buildSharedRules() missing scenario dir line, got: %q", got)
	}
}

func TestBuildSharedRules_OnlyProtectedPaths(t *testing.T) {
	got := buildSharedRules("CLAUDE.md", "")
	if !strings.Contains(got, "- Do NOT modify any protected paths: CLAUDE.md") {
		t.Errorf("buildSharedRules() missing protected paths line, got: %q", got)
	}
	if strings.Contains(got, "- Do NOT modify files in") {
		t.Errorf("buildSharedRules() should not contain scenario dir line when ScenarioDir is empty, got: %q", got)
	}
}

func TestBuildSharedRules_OnlyScenarioDir(t *testing.T) {
	got := buildSharedRules("", "tests/scenarios/")
	if strings.Contains(got, "protected paths") {
		t.Errorf("buildSharedRules() should not contain protected paths line when ProtectedPaths is empty, got: %q", got)
	}
	if !strings.Contains(got, "- Do NOT modify files in tests/scenarios/") {
		t.Errorf("buildSharedRules() missing scenario dir line, got: %q", got)
	}
}

func TestBuildSharedRules_BothEmpty(t *testing.T) {
	got := buildSharedRules("", "")
	if got != "" {
		t.Errorf("buildSharedRules() = %q, want empty string when both inputs are empty", got)
	}
}
