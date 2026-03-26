package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestGenerateSpec_RendersPromptAndCallsRun(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{
		SpecGenerator: "Generate spec for #{{.IssueNumber}} {{.IssueTitle}} repo={{.Repo}} slug={{.Slug}}",
	}

	result, err := GenerateSpec(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("GenerateSpec() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "Generate spec for #42") {
		t.Errorf("expected rendered prompt with issue number, got: %s", prompt)
	}
	if !strings.Contains(prompt, "add-widget-support") {
		t.Errorf("expected slug in prompt, got: %s", prompt)
	}
}

func TestGenerateSpec_SetsSpecGeneratorRole(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{
		SpecGenerator: "Generate spec for #{{.IssueNumber}}",
	}

	_, err := GenerateSpec(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("GenerateSpec() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "spec_generator" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "spec_generator")
	}
}

func TestGenerateSpec_InvalidTimeout(t *testing.T) {
	stubSandboxRunner(t)

	cfg := testConfig()
	cfg.AgentTimeout = "invalid"

	prompts := &Prompts{
		SpecGenerator: "test {{.IssueNumber}}",
	}

	_, err := GenerateSpec(context.Background(), testIssue(), cfg, prompts, nil, testLogger(t))
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}
