package agent

import (
	"context"
	"testing"
)

func TestReview_ReturnsVerdict(t *testing.T) {
	orig := Runner
	t.Cleanup(func() { Runner = orig })

	Runner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("output\nREVIEW_RESULT=APPROVED\nmore output"), []byte(""), 0, nil
	}

	cfg := testImplementerConfig()
	issue := testIssue()
	prompts := testPrompts(t)

	result, err := Review(context.Background(), issue, 10, cfg, prompts, nil, testLogger(t))
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "APPROVED")
	}
}

func TestReview_ChangesRequested(t *testing.T) {
	orig := Runner
	t.Cleanup(func() { Runner = orig })

	Runner = func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("REVIEW_RESULT=CHANGES_REQUESTED\n"), []byte(""), 0, nil
	}

	cfg := testImplementerConfig()
	result, err := Review(context.Background(), testIssue(), 10, cfg, testPrompts(t), nil, testLogger(t))
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Verdict != "CHANGES_REQUESTED" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "CHANGES_REQUESTED")
	}
}
