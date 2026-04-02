package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestReview_ReturnsVerdict(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"output\nAGENT_RESULT=APPROVED\nmore output","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t).Reviewer, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestReview_ChangesRequested(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"AGENT_RESULT=CHANGES_REQUESTED\n","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t).Reviewer, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "CHANGES_REQUESTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "CHANGES_REQUESTED")
	}
}

func TestReview_SetsReviewerRole(t *testing.T) {
	var capturedEnv map[string]string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedEnv = opts.Env
		return &sandbox.RunResult{Stdout: "AGENT_RESULT=APPROVED\n"}, nil
	})

	_, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t).Reviewer, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "reviewer" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "reviewer")
	}
}

func TestReview_StructuredVerdictApproved(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"some text","cost_usd":0,"is_error":false,"verdict":"APPROVED"}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t).Reviewer, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestReview_StructuredVerdictChangesRequested(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"some text","cost_usd":0,"is_error":false,"verdict":"CHANGES_REQUESTED"}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t).Reviewer, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "CHANGES_REQUESTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "CHANGES_REQUESTED")
	}
}

func TestReview_NoStructuredVerdict_FallsBackToStdout(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		// No verdict field in JSON; result text contains the sentinel.
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED\n","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t).Reviewer, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q (fallback)", result.Verdict, "APPROVED")
	}
}

func TestReview_NeitherSource_EmptyVerdict(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"no sentinel here","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t).Reviewer, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "")
	}
}

// TestPromptSelection_SemiformalEnabled verifies that when cfg.Review.Semiformal
// is true, the semiformal prompt is selected regardless of attempt number.
func TestPromptSelection_SemiformalEnabled(t *testing.T) {
	p := &Prompts{
		Reviewer:           "standard",
		ReviewerSemiformal: "semiformal",
	}
	cfg := &config.Config{}
	cfg.Review.Semiformal = true

	for _, attempt := range []int{0, 1, 2} {
		got := selectReviewerPrompt(cfg, p, attempt)
		if got != "semiformal" {
			t.Errorf("attempt=%d: got %q, want %q", attempt, got, "semiformal")
		}
	}
}

// TestPromptSelection_SemiformalOnRetry verifies that when cfg.Review.SemiformalOnRetry
// is true, the standard prompt is used on attempt 0 and the semiformal on attempt > 0.
func TestPromptSelection_SemiformalOnRetry(t *testing.T) {
	p := &Prompts{
		Reviewer:           "standard",
		ReviewerSemiformal: "semiformal",
	}
	cfg := &config.Config{}
	cfg.Review.SemiformalOnRetry = true

	got0 := selectReviewerPrompt(cfg, p, 0)
	if got0 != "standard" {
		t.Errorf("attempt=0: got %q, want %q", got0, "standard")
	}

	got1 := selectReviewerPrompt(cfg, p, 1)
	if got1 != "semiformal" {
		t.Errorf("attempt=1: got %q, want %q", got1, "semiformal")
	}
}

// TestPromptSelection_BothFalse verifies that standard prompt is used when both
// config fields are false.
func TestPromptSelection_BothFalse(t *testing.T) {
	p := &Prompts{
		Reviewer:           "standard",
		ReviewerSemiformal: "semiformal",
	}
	cfg := &config.Config{}

	for _, attempt := range []int{0, 1, 2} {
		got := selectReviewerPrompt(cfg, p, attempt)
		if got != "standard" {
			t.Errorf("attempt=%d: got %q, want %q", attempt, got, "standard")
		}
	}
}

// TestPromptSelection_SemiformalEmptyFallback verifies that when the semiformal
// prompt is empty, selectReviewerPrompt falls back to the standard reviewer prompt
// even when cfg.Review.Semiformal is true.
func TestPromptSelection_SemiformalEmptyFallback(t *testing.T) {
	p := &Prompts{
		Reviewer:           "standard",
		ReviewerSemiformal: "",
	}
	cfg := &config.Config{}
	cfg.Review.Semiformal = true

	for _, attempt := range []int{0, 1, 2} {
		got := selectReviewerPrompt(cfg, p, attempt)
		if got != "standard" {
			t.Errorf("attempt=%d: got %q, want %q (fallback when semiformal empty)", attempt, got, "standard")
		}
	}
}

func TestReview_AcceptsPromptParam(t *testing.T) {
	var capturedPrompt string
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		capturedPrompt = opts.Env["GODARK_PROMPT"]
		out := `{"session_id":"","result":"AGENT_RESULT=APPROVED\n","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	customPrompt := "custom semiformal prompt for PR #{{.PRNumber}}"
	_, err := Review(context.Background(), testIssue(), 10, testConfig(), customPrompt, nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if capturedPrompt == "" {
		t.Fatal("GODARK_PROMPT not set in sandbox env")
	}
	if !strings.Contains(capturedPrompt, "custom semiformal prompt for PR #10") {
		t.Errorf("rendered prompt %q does not contain expected custom text", capturedPrompt)
	}
}
