package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestQualityReview_ReturnsVerdict(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"output\nAGENT_RESULT=APPROVED\nmore output","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := QualityReview(context.Background(), testIssue(), 10, false, testConfig(), testPrompts(t), nil, testLogger(t), 0)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestQualityReview_ChangesRequested(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"AGENT_RESULT=CHANGES_REQUESTED\n","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := QualityReview(context.Background(), testIssue(), 10, false, testConfig(), testPrompts(t), nil, testLogger(t), 0)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}
	if result.Verdict != "CHANGES_REQUESTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "CHANGES_REQUESTED")
	}
}

func TestQualityReview_SetsQualityReviewerRole(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: "AGENT_RESULT=APPROVED\n"}, nil
	})

	_, err := QualityReview(context.Background(), testIssue(), 10, false, testConfig(), testPrompts(t), nil, testLogger(t), 0)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "quality_reviewer" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "quality_reviewer")
	}
}

func TestQualityReview_StructuredVerdict(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"some text","cost_usd":0,"is_error":false,"verdict":"APPROVED"}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := QualityReview(context.Background(), testIssue(), 10, false, testConfig(), testPrompts(t), nil, testLogger(t), 0)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

// --- strictnessDirective unit tests ---

func TestStrictnessDirective_Cycle0NoDirective(t *testing.T) {
	got := strictnessDirective(0, 3, true)
	if got != "" {
		t.Errorf("cycle=0 should have no directive, got %q", got)
	}
}

func TestStrictnessDirective_DecayDisabled(t *testing.T) {
	got := strictnessDirective(2, 3, false)
	if got != "" {
		t.Errorf("decay=false should have no directive, got %q", got)
	}
}

func TestStrictnessDirective_SingleAttemptNoDirective(t *testing.T) {
	got := strictnessDirective(0, 1, true)
	if got != "" {
		t.Errorf("maxAttempts=1 should have no directive, got %q", got)
	}
}

func TestStrictnessDirective_MidCycleNarrowsScope(t *testing.T) {
	got := strictnessDirective(1, 3, true)
	if !strings.Contains(got, "security vulnerabilities and correctness issues only") {
		t.Errorf("mid-cycle directive should mention correctness narrowing, got %q", got)
	}
	if !strings.Contains(got, "STRICTNESS OVERRIDE") {
		t.Errorf("mid-cycle directive should have STRICTNESS OVERRIDE prefix, got %q", got)
	}
}

func TestStrictnessDirective_FinalCycleSecurityOnly(t *testing.T) {
	got := strictnessDirective(2, 3, true)
	if !strings.Contains(got, "final quality review cycle") {
		t.Errorf("final cycle directive should mention 'final quality review cycle', got %q", got)
	}
	if !strings.Contains(got, "security vulnerability") {
		t.Errorf("final cycle directive should mention 'security vulnerability', got %q", got)
	}
}

func TestStrictnessDirective_MidCycleContainsRetryNumber(t *testing.T) {
	got := strictnessDirective(1, 5, true)
	if !strings.Contains(got, "retry 2") {
		t.Errorf("mid-cycle directive should contain retry number 2, got %q", got)
	}
}

// --- QualityReview prompt rendering tests ---

func qualityConfigWithDecay(maxRetries int) *config.Config {
	cfg := testConfig()
	cfg.MaxRetries = maxRetries
	cfg.QualityStrictnessDecay = true
	return cfg
}

func qualityPromptsWithDirective(t *testing.T) *Prompts {
	t.Helper()
	p := testPrompts(t)
	p.QualityReviewer = "Quality review PR #{{.PRNumber}} for #{{.IssueNumber}}{{if .StrictnessDirective}}\n{{.StrictnessDirective}}{{end}}"
	return p
}

func TestQualityReview_Cycle0NoDirectiveInPrompt(t *testing.T) {
	var capturedPrompt string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedPrompt = opts.Env["GODARK_PROMPT"]
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	cfg := qualityConfigWithDecay(2) // maxAttempts=3
	_, err := QualityReview(context.Background(), testIssue(), 10, false, cfg, qualityPromptsWithDirective(t), nil, testLogger(t), 0)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	if strings.Contains(capturedPrompt, "STRICTNESS OVERRIDE") {
		t.Errorf("cycle=0 prompt should not contain STRICTNESS OVERRIDE, got: %q", capturedPrompt)
	}
}

func TestQualityReview_Cycle1NarrowsScope(t *testing.T) {
	var capturedPrompt string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedPrompt = opts.Env["GODARK_PROMPT"]
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	cfg := qualityConfigWithDecay(2) // maxAttempts=3
	_, err := QualityReview(context.Background(), testIssue(), 10, false, cfg, qualityPromptsWithDirective(t), nil, testLogger(t), 1)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	if !strings.Contains(capturedPrompt, "security vulnerabilities and correctness issues only") {
		t.Errorf("cycle=1 prompt should contain correctness narrowing, got: %q", capturedPrompt)
	}
}

func TestQualityReview_FinalCycleSecurityOnly(t *testing.T) {
	var capturedPrompt string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedPrompt = opts.Env["GODARK_PROMPT"]
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	cfg := qualityConfigWithDecay(2) // maxAttempts=3, final cycle=2
	_, err := QualityReview(context.Background(), testIssue(), 10, false, cfg, qualityPromptsWithDirective(t), nil, testLogger(t), 2)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	if !strings.Contains(capturedPrompt, "final quality review cycle") {
		t.Errorf("final cycle prompt should contain 'final quality review cycle', got: %q", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "security vulnerability") {
		t.Errorf("final cycle prompt should contain 'security vulnerability', got: %q", capturedPrompt)
	}
}

func TestQualityReview_SingleAttemptNoDirective(t *testing.T) {
	var capturedPrompt string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedPrompt = opts.Env["GODARK_PROMPT"]
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	cfg := qualityConfigWithDecay(0) // maxAttempts=1
	_, err := QualityReview(context.Background(), testIssue(), 10, false, cfg, qualityPromptsWithDirective(t), nil, testLogger(t), 0)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	if strings.Contains(capturedPrompt, "STRICTNESS OVERRIDE") {
		t.Errorf("single-attempt prompt should not contain STRICTNESS OVERRIDE, got: %q", capturedPrompt)
	}
}

func TestQualityReview_DecayDisabledNoDirective(t *testing.T) {
	var capturedPrompt string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedPrompt = opts.Env["GODARK_PROMPT"]
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	cfg := testConfig()
	cfg.MaxRetries = 2
	cfg.QualityStrictnessDecay = false
	_, err := QualityReview(context.Background(), testIssue(), 10, false, cfg, qualityPromptsWithDirective(t), nil, testLogger(t), 2)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	if strings.Contains(capturedPrompt, "STRICTNESS OVERRIDE") {
		t.Errorf("decay=false prompt should not contain STRICTNESS OVERRIDE, got: %q", capturedPrompt)
	}
}

func TestQualityReview_DirectiveAppendedNotReplaced(t *testing.T) {
	var capturedPrompt string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedPrompt = opts.Env["GODARK_PROMPT"]
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	cfg := qualityConfigWithDecay(2) // maxAttempts=3
	_, err := QualityReview(context.Background(), testIssue(), 10, false, cfg, qualityPromptsWithDirective(t), nil, testLogger(t), 1)
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	// Prompt should contain both original instructions and the directive.
	if !strings.Contains(capturedPrompt, "Quality review PR") {
		t.Errorf("prompt should contain original quality reviewer text, got: %q", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "STRICTNESS OVERRIDE") {
		t.Errorf("prompt should contain strictness directive, got: %q", capturedPrompt)
	}
}
