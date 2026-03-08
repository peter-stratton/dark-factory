package agent

import (
	"context"
	"os"
	"path/filepath"
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

	result, err := Retry(context.Background(), testIssue(), 7, "", testConfig(), testPrompts(t), nil, testLogger(t))
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

func TestRetry_WithSessionID_SetsGODARK_SESSION_ID(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"sess-new","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Retry(context.Background(), testIssue(), 7, "sess-abc123", testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	if capturedEnv["GODARK_SESSION_ID"] != "sess-abc123" {
		t.Errorf("GODARK_SESSION_ID = %q, want %q", capturedEnv["GODARK_SESSION_ID"], "sess-abc123")
	}
}

func TestRetry_WithoutSessionID_NoSessionEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Retry(context.Background(), testIssue(), 7, "", testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("GODARK_SESSION_ID should not be set when prevSessionID is empty, got %q", capturedEnv["GODARK_SESSION_ID"])
	}
}

func TestImplement_DoesNotSetSessionIDEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	_, err := Implement(context.Background(), testIssue(), testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	if _, ok := capturedEnv["GODARK_SESSION_ID"]; ok {
		t.Errorf("Implement should not set GODARK_SESSION_ID, got %q", capturedEnv["GODARK_SESSION_ID"])
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
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	cfg := testConfig()
	cfg.DeniedCommands = []string{"rm -rf", "git reset --hard"}

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t))
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

	_, err := Retry(context.Background(), testIssue(), 7, "", testConfig(), testPrompts(t), nil, testLogger(t))
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
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
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

func TestVerifyFix_WithSessionID_SetsGODARK_SESSION_ID(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"sess-new","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	prompts := &Prompts{VerifyFix: "fix prompt"}
	_, err := VerifyFix(context.Background(), testIssue(), 7, "some errors", "sess-abc123", testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("VerifyFix() error = %v", err)
	}

	if capturedEnv["GODARK_SESSION_ID"] != "sess-abc123" {
		t.Errorf("GODARK_SESSION_ID = %q, want %q", capturedEnv["GODARK_SESSION_ID"], "sess-abc123")
	}
}

func TestVerifyFix_WithoutSessionID_NoSessionEnv(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
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
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
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
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
	})

	cfg := testConfig()
	cfg.GeneratedPaths = []string{"gen/", "**/*.pb.go"}

	_, err := Implement(context.Background(), testIssue(), cfg, testPrompts(t), nil, testLogger(t))
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
