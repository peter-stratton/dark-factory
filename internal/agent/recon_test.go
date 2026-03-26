package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestRecon_RendersPromptAndCallsRun(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"recon output","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{Recon: "Recon issue #{{.IssueNumber}}: {{.IssueTitle}}"}
	result, err := Recon(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("Recon() error = %v", err)
	}
	if result == nil {
		t.Fatal("Recon() returned nil result")
	}
	if result.ResultText != "recon output" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "recon output")
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, "#42") {
		t.Errorf("expected issue number in recon prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Add Widget Support") {
		t.Errorf("expected issue title in recon prompt, got: %s", prompt)
	}
}

func TestRecon_SetsReconRole(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{Recon: "recon prompt"}
	_, err := Recon(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("Recon() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "recon" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "recon")
	}
}

func TestRecon_InvalidTimeout(t *testing.T) {
	stubSandboxRunner(t)

	cfg := testConfig()
	cfg.AgentTimeout = "invalid"

	prompts := &Prompts{Recon: "recon prompt"}
	_, err := Recon(context.Background(), testIssue(), cfg, prompts, nil, testLogger(t))
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestImplement_ReconBriefPassedToPrompt(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	brief := "key finding: use X approach"
	prompts := &Prompts{
		Implementer: "Implement #{{.IssueNumber}}{{if .ReconBrief}} brief={{.ReconBrief}}{{end}}",
	}

	_, err := Implement(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t), brief)
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if !strings.Contains(prompt, brief) {
		t.Errorf("expected recon brief in implementer prompt, got: %s", prompt)
	}
}

func TestImplement_EmptyReconBriefDoesNotAffectPrompt(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: `{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`}, nil
	})

	prompts := &Prompts{
		Implementer: "Implement #{{.IssueNumber}}{{if .ReconBrief}} brief={{.ReconBrief}}{{end}}",
	}

	_, err := Implement(context.Background(), testIssue(), testConfig(), prompts, nil, testLogger(t), "")
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	prompt := capturedEnv["GODARK_PROMPT"]
	if strings.Contains(prompt, "brief=") {
		t.Errorf("prompt should not contain brief section when reconBrief is empty, got: %s", prompt)
	}
}
