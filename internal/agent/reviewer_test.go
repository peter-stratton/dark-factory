package agent

import (
	"context"
	"log/slog"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

func TestReview_ReturnsVerdict(t *testing.T) {
	stubSandboxRunnerFunc(t, func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
		out := `{"session_id":"","result":"output\nAGENT_RESULT=APPROVED\nmore output","cost_usd":0,"is_error":false}`
		return &sandbox.RunResult{Stdout: out}, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t), false)
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

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t), false)
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

	_, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t), false)
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

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t), false)
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

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t), false)
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

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t), false)
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

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t), false)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "")
	}
}
