package agent

import (
	"context"
	"testing"
)

func TestReview_ReturnsVerdict(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"","result":"output\nREVIEW_RESULT=APPROVED\nmore output","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestReview_ChangesRequested(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"","result":"REVIEW_RESULT=CHANGES_REQUESTED\n","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "CHANGES_REQUESTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "CHANGES_REQUESTED")
	}
}

func TestReview_SetsReviewerRole(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte("REVIEW_RESULT=APPROVED\n"), []byte(""), 0, nil
	})

	_, err := Review(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "reviewer" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "reviewer")
	}
}
