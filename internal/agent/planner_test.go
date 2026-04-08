package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestPlan_RendersPromptAndCallsRun(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"## Approach\nDo the thing","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{Planner: "Plan issue #{{.IssueNumber}}: {{.IssueTitle}}\nRecon: {{.ReconBrief}}"}
	result, err := Plan(context.Background(), testIssue(), false, testConfig(), prompts, nil, testLogger(t), "recon context here")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if result == nil {
		t.Fatal("Plan() returned nil result")
	}
	if result.ResultText != "## Approach\nDo the thing" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "## Approach\nDo the thing")
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "#42") {
		t.Errorf("expected issue number in planner prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Add Widget Support") {
		t.Errorf("expected issue title in planner prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "recon context here") {
		t.Errorf("expected recon brief in planner prompt, got: %s", prompt)
	}
}

func TestPlan_SetsPlannerRole(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{Planner: "planner prompt"}
	_, err := Plan(context.Background(), testIssue(), false, testConfig(), prompts, nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "planner" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "planner")
	}
}

func TestPlan_EmptyReconBrief(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"plan output","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{Planner: "Plan issue #{{.IssueNumber}}: {{.IssueTitle}}\nRecon: {{.ReconBrief}}"}
	result, err := Plan(context.Background(), testIssue(), false, testConfig(), prompts, nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Plan() with empty recon brief error = %v", err)
	}
	if result == nil {
		t.Fatal("Plan() returned nil result")
	}
	if result.ResultText != "plan output" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "plan output")
	}
}

func TestLoadPrompts_PlannerLoadedFromEmbedded(t *testing.T) {
	cfg := &config.Config{}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.Planner == "" {
		t.Error("Planner should be loaded from embedded default")
	}
}
