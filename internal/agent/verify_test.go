package agent

import (
	"context"
	"strings"
	"testing"
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
