package agent

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateSpec_RendersPromptAndCallsRun(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
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
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte(`{"session_id":"","result":"ok","cost_usd":0,"is_error":false}`), []byte(""), 0, nil
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
	stubRunner(t)

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
