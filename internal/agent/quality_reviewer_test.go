package agent

import (
	"context"
	"testing"
)

func TestQualityReview_ReturnsVerdict(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"","result":"output\nQUALITY_RESULT=APPROVED\nmore output","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := QualityReview(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestQualityReview_ChangesRequested(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"","result":"QUALITY_RESULT=CHANGES_REQUESTED\n","cost_usd":0,"is_error":false}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := QualityReview(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}
	if result.Verdict != "CHANGES_REQUESTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "CHANGES_REQUESTED")
	}
}

func TestQualityReview_SetsQualityReviewerRole(t *testing.T) {
	var capturedEnv map[string]string
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		capturedEnv = env
		return []byte("QUALITY_RESULT=APPROVED\n"), []byte(""), 0, nil
	})

	_, err := QualityReview(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}

	if capturedEnv["GODARK_ROLE"] != "quality_reviewer" {
		t.Errorf("GODARK_ROLE = %q, want %q", capturedEnv["GODARK_ROLE"], "quality_reviewer")
	}
}

func TestQualityReview_StructuredVerdict(t *testing.T) {
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		out := `{"session_id":"","result":"some text","cost_usd":0,"is_error":false,"verdict":"APPROVED"}`
		return []byte(out), []byte(""), 0, nil
	})

	result, err := QualityReview(context.Background(), testIssue(), 10, testConfig(), testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("QualityReview() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestParseQualityResult_Approved(t *testing.T) {
	got := ParseQualityResult("some output\nQUALITY_RESULT=APPROVED\nmore output")
	if got != "APPROVED" {
		t.Errorf("ParseQualityResult() = %q, want %q", got, "APPROVED")
	}
}

func TestParseQualityResult_ChangesRequested(t *testing.T) {
	got := ParseQualityResult("QUALITY_RESULT=CHANGES_REQUESTED\n")
	if got != "CHANGES_REQUESTED" {
		t.Errorf("ParseQualityResult() = %q, want %q", got, "CHANGES_REQUESTED")
	}
}

func TestParseQualityResult_NoMatch(t *testing.T) {
	got := ParseQualityResult("no sentinel here")
	if got != "" {
		t.Errorf("ParseQualityResult() = %q, want %q", got, "")
	}
}
