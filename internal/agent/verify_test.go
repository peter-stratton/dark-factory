package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/sandbox"
)

func makeRunner(exitCode int, stdout, stderr string) CommandRunner {
	return func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		return []byte(stdout), []byte(stderr), exitCode, nil
	}
}

func TestRunVerify_AllPass(t *testing.T) {
	checks := []Check{
		{Name: "build", Command: "go build ./..."},
		{Name: "lint", Command: "golint ./..."},
		{Name: "test", Command: "go test ./..."},
	}
	run := makeRunner(0, "ok", "")

	result := RunVerify(context.Background(), checks, run)

	if !result.AllPassed {
		t.Errorf("AllPassed = false, want true")
	}
	if len(result.Checks) != 3 {
		t.Errorf("len(Checks) = %d, want 3", len(result.Checks))
	}
	for _, cr := range result.Checks {
		if !cr.Passed {
			t.Errorf("check %q Passed = false, want true", cr.Name)
		}
	}
}

func TestRunVerify_BuildFails(t *testing.T) {
	var calledChecks []string
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		calledChecks = append(calledChecks, command)
		if strings.Contains(command, "build") {
			return []byte("build error"), []byte(""), 1, nil
		}
		return []byte("ok"), []byte(""), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
		{Name: "lint", Command: "golint ./..."},
		{Name: "test", Command: "go test ./..."},
	}

	result := RunVerify(context.Background(), checks, run)

	if result.AllPassed {
		t.Errorf("AllPassed = true, want false")
	}
	if len(result.Checks) != 1 {
		t.Errorf("len(Checks) = %d, want 1 (only build)", len(result.Checks))
	}
	if result.Checks[0].Name != "build" {
		t.Errorf("Checks[0].Name = %q, want %q", result.Checks[0].Name, "build")
	}
	if result.Checks[0].Passed {
		t.Errorf("build check Passed = true, want false")
	}
	if len(calledChecks) != 1 {
		t.Errorf("runner called %d times, want 1 (should stop at first failure)", len(calledChecks))
	}
}

func TestRunVerify_LintFails(t *testing.T) {
	var calledChecks []string
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		calledChecks = append(calledChecks, command)
		if strings.Contains(command, "lint") {
			return []byte("lint error"), []byte(""), 1, nil
		}
		return []byte("ok"), []byte(""), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
		{Name: "lint", Command: "golint ./..."},
		{Name: "test", Command: "go test ./..."},
	}

	result := RunVerify(context.Background(), checks, run)

	if result.AllPassed {
		t.Errorf("AllPassed = true, want false")
	}
	if len(result.Checks) != 2 {
		t.Errorf("len(Checks) = %d, want 2 (build and lint)", len(result.Checks))
	}
	if result.Checks[0].Name != "build" || !result.Checks[0].Passed {
		t.Errorf("expected build to pass, got name=%q passed=%v", result.Checks[0].Name, result.Checks[0].Passed)
	}
	if result.Checks[1].Name != "lint" || result.Checks[1].Passed {
		t.Errorf("expected lint to fail, got name=%q passed=%v", result.Checks[1].Name, result.Checks[1].Passed)
	}
	if len(calledChecks) != 2 {
		t.Errorf("runner called %d times, want 2 (build + lint)", len(calledChecks))
	}
}

func TestRunVerify_EmptyCommandSkipped(t *testing.T) {
	var calledCount int
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		calledCount++
		return []byte("ok"), []byte(""), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
		{Name: "lint", Command: ""},
		{Name: "test", Command: "go test ./..."},
	}

	result := RunVerify(context.Background(), checks, run)

	if !result.AllPassed {
		t.Errorf("AllPassed = false, want true")
	}
	if len(result.Checks) != 2 {
		t.Errorf("len(Checks) = %d, want 2 (lint skipped)", len(result.Checks))
	}
	if calledCount != 2 {
		t.Errorf("runner called %d times, want 2 (empty command should be skipped)", calledCount)
	}
	for _, cr := range result.Checks {
		if cr.Name == "lint" {
			t.Errorf("lint check with empty command should not appear in results")
		}
	}
}

func TestRunVerify_AllEmptyCommands(t *testing.T) {
	var calledCount int
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		calledCount++
		return []byte("ok"), []byte(""), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: ""},
		{Name: "lint", Command: ""},
		{Name: "test", Command: ""},
	}

	result := RunVerify(context.Background(), checks, run)

	if !result.AllPassed {
		t.Errorf("AllPassed = false, want true when all commands empty")
	}
	if len(result.Checks) != 0 {
		t.Errorf("len(Checks) = %d, want 0", len(result.Checks))
	}
	if calledCount != 0 {
		t.Errorf("runner called %d times, want 0", calledCount)
	}
}

func TestRunVerify_OutputTruncation(t *testing.T) {
	// Produce 10 KB of output.
	bigOutput := strings.Repeat("x", 10*1024)
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		return []byte(bigOutput), []byte(""), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
	}

	result := RunVerify(context.Background(), checks, run)

	if !result.AllPassed {
		t.Errorf("AllPassed = false, want true")
	}
	if len(result.Checks) != 1 {
		t.Fatalf("len(Checks) = %d, want 1", len(result.Checks))
	}

	got := result.Checks[0].Output
	if len(got) != verifyOutputLimit {
		t.Errorf("Output length = %d, want %d", len(got), verifyOutputLimit)
	}
	// The tail of the big output should be all 'x'.
	if got != strings.Repeat("x", verifyOutputLimit) {
		t.Errorf("Output is not the tail of the big output")
	}
}

func TestRunVerify_OutputCombinesStdoutAndStderr(t *testing.T) {
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		return []byte("stdout-data"), []byte("stderr-data"), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
	}

	result := RunVerify(context.Background(), checks, run)

	if len(result.Checks) != 1 {
		t.Fatalf("len(Checks) = %d, want 1", len(result.Checks))
	}

	output := result.Checks[0].Output
	if !strings.Contains(output, "stdout-data") {
		t.Errorf("Output missing stdout-data: %q", output)
	}
	if !strings.Contains(output, "stderr-data") {
		t.Errorf("Output missing stderr-data: %q", output)
	}
}

func TestRunVerify_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		callCount++
		if callCount == 1 {
			// Cancel context after first check completes.
			cancel()
			return []byte("ok"), []byte(""), 0, nil
		}
		// Subsequent calls: context is cancelled.
		if ctx.Err() != nil {
			return []byte(""), []byte("context cancelled"), 1, ctx.Err()
		}
		return []byte("ok"), []byte(""), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
		{Name: "lint", Command: "golint ./..."},
		{Name: "test", Command: "go test ./..."},
	}

	result := RunVerify(ctx, checks, run)

	// First check passed; second was cancelled (non-zero exit or err) so we stopped.
	// Results so far are returned.
	if len(result.Checks) == 0 {
		t.Errorf("expected at least one result, got none")
	}
	if result.Checks[0].Name != "build" {
		t.Errorf("Checks[0].Name = %q, want %q", result.Checks[0].Name, "build")
	}
}

func TestRunVerify_ExitCodeRecordedInResult(t *testing.T) {
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		return []byte("output"), []byte(""), 2, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
	}

	result := RunVerify(context.Background(), checks, run)

	if len(result.Checks) != 1 {
		t.Fatalf("len(Checks) = %d, want 1", len(result.Checks))
	}
	if result.Checks[0].ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", result.Checks[0].ExitCode)
	}
	if result.Checks[0].Passed {
		t.Errorf("Passed = true, want false for non-zero exit")
	}
}

func TestRunVerify_NoChecks(t *testing.T) {
	var calledCount int
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		calledCount++
		return nil, nil, 0, nil
	}

	result := RunVerify(context.Background(), nil, run)

	if !result.AllPassed {
		t.Errorf("AllPassed = false, want true for empty checks")
	}
	if len(result.Checks) != 0 {
		t.Errorf("len(Checks) = %d, want 0", len(result.Checks))
	}
	if calledCount != 0 {
		t.Errorf("runner called %d times, want 0", calledCount)
	}
}

func TestRunVerify_OutputWithinLimitNotTruncated(t *testing.T) {
	smallOutput := strings.Repeat("y", 100)
	run := func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		return []byte(smallOutput), []byte(""), 0, nil
	}

	checks := []Check{
		{Name: "build", Command: "go build ./..."},
	}

	result := RunVerify(context.Background(), checks, run)

	if result.Checks[0].Output != smallOutput {
		t.Errorf("Output = %q, want %q (should not be truncated)", result.Checks[0].Output, smallOutput)
	}
}

func TestFormatVerifyErrors_FailedChecksIncluded(t *testing.T) {
	result := VerifyResult{
		AllPassed: false,
		Checks: []CheckResult{
			{Name: "build", Passed: false, Output: "build error output", ExitCode: 1},
		},
	}

	got := formatVerifyErrors(result)

	if !strings.Contains(got, "=== build (exit code 1) ===") {
		t.Errorf("expected check header in output, got: %q", got)
	}
	if !strings.Contains(got, "build error output") {
		t.Errorf("expected check output in result, got: %q", got)
	}
}

func TestFormatVerifyErrors_PassedChecksExcluded(t *testing.T) {
	result := VerifyResult{
		AllPassed: false,
		Checks: []CheckResult{
			{Name: "build", Passed: true, Output: "ok", ExitCode: 0},
			{Name: "lint", Passed: false, Output: "lint error", ExitCode: 1},
		},
	}

	got := formatVerifyErrors(result)

	if strings.Contains(got, "build") {
		t.Errorf("passed check 'build' should not appear in errors, got: %q", got)
	}
	if !strings.Contains(got, "lint") {
		t.Errorf("failed check 'lint' should appear in errors, got: %q", got)
	}
}

func TestFormatVerifyErrors_MultipleFailures(t *testing.T) {
	result := VerifyResult{
		AllPassed: false,
		Checks: []CheckResult{
			{Name: "build", Passed: false, Output: "build failed", ExitCode: 1},
			{Name: "test", Passed: false, Output: "tests failed", ExitCode: 2},
		},
	}

	got := formatVerifyErrors(result)

	if !strings.Contains(got, "=== build (exit code 1) ===") {
		t.Errorf("expected build header in output, got: %q", got)
	}
	if !strings.Contains(got, "=== test (exit code 2) ===") {
		t.Errorf("expected test header in output, got: %q", got)
	}
	if !strings.Contains(got, "build failed") {
		t.Errorf("expected build output, got: %q", got)
	}
	if !strings.Contains(got, "tests failed") {
		t.Errorf("expected test output, got: %q", got)
	}
}

func TestFormatVerifyErrors_EmptyWhenAllPassed(t *testing.T) {
	result := VerifyResult{
		AllPassed: true,
		Checks: []CheckResult{
			{Name: "build", Passed: true, Output: "ok", ExitCode: 0},
		},
	}

	got := formatVerifyErrors(result)

	if got != "" {
		t.Errorf("expected empty string when all checks passed, got: %q", got)
	}
}

// stubSandboxRunContainer replaces sandboxRunContainer with a custom function
// for the duration of the test.
func stubSandboxRunContainer(t *testing.T, fn func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error)) {
	t.Helper()
	orig := sandboxRunContainer
	t.Cleanup(func() { sandboxRunContainer = orig })
	sandboxRunContainer = fn
}

func TestSandboxCommandRunner_UsesCorrectImage(t *testing.T) {
	var capturedOpts sandbox.RunOpts
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		capturedOpts = opts
		return &sandbox.RunResult{ExitCode: 0, Stdout: "ok"}, nil
	})

	runner := sandboxCommandRunner("myimage:latest", "owner/repo", "feature-branch", nil, slog.Default())
	_, _, _, err := runner(context.Background(), "go build ./...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOpts.Image != "myimage:latest" {
		t.Errorf("Image = %q, want %q", capturedOpts.Image, "myimage:latest")
	}
}

func TestSandboxCommandRunner_RepoAndBranchPassedViaEnv(t *testing.T) {
	var capturedOpts sandbox.RunOpts
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		capturedOpts = opts
		return &sandbox.RunResult{ExitCode: 0}, nil
	})

	runner := sandboxCommandRunner("img:tag", "owner/myrepo", "pr-branch-42", nil, slog.Default())
	_, _, _, err := runner(context.Background(), "go test ./...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOpts.Env["GODARK_REPO"] != "owner/myrepo" {
		t.Errorf("GODARK_REPO = %q, want %q", capturedOpts.Env["GODARK_REPO"], "owner/myrepo")
	}
	if capturedOpts.Env["GODARK_BRANCH"] != "pr-branch-42" {
		t.Errorf("GODARK_BRANCH = %q, want %q", capturedOpts.Env["GODARK_BRANCH"], "pr-branch-42")
	}
}

func TestSandboxCommandRunner_CommandPassedViaEnv(t *testing.T) {
	var capturedOpts sandbox.RunOpts
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		capturedOpts = opts
		return &sandbox.RunResult{ExitCode: 0}, nil
	})

	runner := sandboxCommandRunner("img:tag", "owner/repo", "branch", nil, slog.Default())
	const cmd = "go build ./..."
	_, _, _, err := runner(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOpts.Env["GODARK_VERIFY_CMD"] != cmd {
		t.Errorf("GODARK_VERIFY_CMD = %q, want %q", capturedOpts.Env["GODARK_VERIFY_CMD"], cmd)
	}
}

func TestSandboxCommandRunner_ReturnsStdoutAndStderr(t *testing.T) {
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{ExitCode: 0, Stdout: "build ok\n", Stderr: "warning: x\n"}, nil
	})

	runner := sandboxCommandRunner("img:tag", "owner/repo", "branch", nil, slog.Default())
	stdout, stderr, exitCode, err := runner(context.Background(), "go build ./...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(stdout) != "build ok\n" {
		t.Errorf("stdout = %q, want %q", stdout, "build ok\n")
	}
	if string(stderr) != "warning: x\n" {
		t.Errorf("stderr = %q, want %q", stderr, "warning: x\n")
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0", exitCode)
	}
}

func TestSandboxCommandRunner_NonZeroExitCode(t *testing.T) {
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{ExitCode: 2, Stdout: "", Stderr: "build failed\n"}, nil
	})

	runner := sandboxCommandRunner("img:tag", "owner/repo", "branch", nil, slog.Default())
	_, _, exitCode, err := runner(context.Background(), "go build ./...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exitCode != 2 {
		t.Errorf("exitCode = %d, want 2", exitCode)
	}
}

func TestSandboxCommandRunner_ContainerError(t *testing.T) {
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return nil, fmt.Errorf("docker create failed")
	})

	runner := sandboxCommandRunner("img:tag", "owner/repo", "branch", nil, slog.Default())
	_, _, exitCode, err := runner(context.Background(), "go build ./...")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "running verify container") {
		t.Errorf("error = %q, want to contain 'running verify container'", err)
	}
	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1 on container error", exitCode)
	}
}

func TestSandboxCommandRunner_CheckIsPassedWhenExitZero(t *testing.T) {
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{ExitCode: 0, Stdout: "ok"}, nil
	})

	runner := sandboxCommandRunner("img:tag", "owner/repo", "branch", nil, slog.Default())
	checks := []Check{{Name: "build", Command: "go build ./..."}}
	result := RunVerify(context.Background(), checks, runner)

	if !result.AllPassed {
		t.Errorf("AllPassed = false, want true for exit code 0")
	}
}

func TestSandboxCommandRunner_CheckFailsWhenExitNonZero(t *testing.T) {
	stubSandboxRunContainer(t, func(_ context.Context, opts sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{ExitCode: 1, Stderr: "error output"}, nil
	})

	runner := sandboxCommandRunner("img:tag", "owner/repo", "branch", nil, slog.Default())
	checks := []Check{{Name: "build", Command: "go build ./..."}}
	result := RunVerify(context.Background(), checks, runner)

	if result.AllPassed {
		t.Errorf("AllPassed = true, want false for non-zero exit code")
	}
	if len(result.Checks) != 1 || result.Checks[0].ExitCode != 1 {
		t.Errorf("unexpected check result: %+v", result.Checks)
	}
}
