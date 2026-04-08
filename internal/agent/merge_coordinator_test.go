package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestMergeCoordinate_RendersPromptAndCallsRun(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"merge output","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{MergeCoordinator: "Merge PR #{{.PRNumber}} for issue #{{.IssueNumber}}: {{.IssueTitle}}"}
	result, err := MergeCoordinate(context.Background(), testIssue(), 99, "conflict in main.go", false, testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("MergeCoordinate() error = %v", err)
	}
	if result == nil {
		t.Fatal("MergeCoordinate() returned nil result")
	}
	if result.ResultText != "merge output" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "merge output")
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "#42") {
		t.Errorf("expected issue number in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "#99") {
		t.Errorf("expected PR number in prompt, got: %s", prompt)
	}
}

func TestMergeCoordinate_ConflictInfoInjected(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	conflictInfo := "CONFLICT (content): Merge conflict in pkg/server.go"
	prompts := &Prompts{MergeCoordinator: "Conflicts: {{.ConflictInfo}}"}
	_, err := MergeCoordinate(context.Background(), testIssue(), 10, conflictInfo, false, testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("MergeCoordinate() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, conflictInfo) {
		t.Errorf("expected conflict info in prompt, got: %s", prompt)
	}
}

func TestMergeCoordinate_PRNumberInjected(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{MergeCoordinator: "PR={{.PRNumber}}"}
	_, err := MergeCoordinate(context.Background(), testIssue(), 55, "", false, testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("MergeCoordinate() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	expected := fmt.Sprintf("PR=%d", 55)
	if !strings.Contains(prompt, expected) {
		t.Errorf("expected %q in prompt, got: %s", expected, prompt)
	}
}

func TestMergeCoordinate_Role(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{MergeCoordinator: "merge prompt"}
	_, err := MergeCoordinate(context.Background(), testIssue(), 1, "", false, testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("MergeCoordinate() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "merge_coordinator" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "merge_coordinator")
	}
}

func TestMergeCoordinate_InvalidTimeout(t *testing.T) {
	stubSandboxRunner(t)

	cfg := testConfig()
	cfg.AgentTimeout = "invalid"

	prompts := &Prompts{MergeCoordinator: "merge prompt"}
	_, err := MergeCoordinate(context.Background(), testIssue(), 1, "", false, cfg, prompts, nil, testLogger(t))
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}
