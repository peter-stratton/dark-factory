package agent

import (
	"context"
	"strings"
	"testing"
)

func TestGenerateSpec_RendersPromptAndCallsRun(t *testing.T) {
	captured := stubRunner(t)

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

	joined := strings.Join(*captured, " ")
	if !strings.Contains(joined, "Generate spec for #42") {
		t.Errorf("expected rendered prompt with issue number, got: %s", joined)
	}
	if !strings.Contains(joined, "add-widget-support") {
		t.Errorf("expected slug in prompt, got: %s", joined)
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
