package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

// rateLimitJSON returns a rate_limit_event JSON line with the given Unix timestamp.
func rateLimitJSON(resetsAtUnix int64) string {
	return `{"type":"rate_limit_event","rate_limit_info":{"status":"limit_reached","resetsAt":` +
		strconv.FormatInt(resetsAtUnix, 10) + `,"rateLimitType":"usage"}}`
}

func TestParseRateLimitEvent_Valid(t *testing.T) {
	resetsAt := time.Now().Add(2 * time.Hour).Unix()
	stdout := "some output\n" + rateLimitJSON(resetsAt) + "\n{\"type\":\"result\"}"

	rawLine, got := parseRateLimitEvent(stdout)
	if got.IsZero() {
		t.Fatal("expected non-zero time, got zero")
	}
	if got.Unix() != resetsAt {
		t.Errorf("got Unix=%d, want %d", got.Unix(), resetsAt)
	}
	if rawLine == "" {
		t.Error("expected non-empty raw JSON line")
	}
}

func TestParseRateLimitEvent_NoEvent(t *testing.T) {
	stdout := `{"type":"result","session_id":"abc","result":"done","is_error":false}`
	_, got := parseRateLimitEvent(stdout)
	if !got.IsZero() {
		t.Errorf("expected zero time for stdout with no rate_limit_event, got %v", got)
	}
}

func TestParseRateLimitEvent_EmptyStdout(t *testing.T) {
	_, got := parseRateLimitEvent("")
	if !got.IsZero() {
		t.Errorf("expected zero time for empty stdout, got %v", got)
	}
}

func TestParseRateLimitEvent_AllowedStatusIgnored(t *testing.T) {
	stdout := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","resetsAt":1774854000,"rateLimitType":"five_hour"}}`
	_, got := parseRateLimitEvent(stdout)
	if !got.IsZero() {
		t.Errorf("expected zero time for allowed status, got %v", got)
	}
}

func TestParseRateLimitEvent_AllowedWarningIgnored(t *testing.T) {
	stdout := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed_warning","resetsAt":1774854000,"rateLimitType":"five_hour","utilization":0.9}}`
	_, got := parseRateLimitEvent(stdout)
	if !got.IsZero() {
		t.Errorf("expected zero time for allowed_warning status, got %v", got)
	}
}

func TestParseRateLimitEvent_UnknownStatusTriggers(t *testing.T) {
	resetsAt := int64(1774854000)
	// Any status that isn't "allowed" or "allowed_warning" should trigger.
	for _, status := range []string{"limit_reached", "exhausted", "rate_limited", "something_new"} {
		stdout := fmt.Sprintf(`{"type":"rate_limit_event","rate_limit_info":{"status":"%s","resetsAt":%d,"rateLimitType":"five_hour"}}`, status, resetsAt)
		_, got := parseRateLimitEvent(stdout)
		if got.IsZero() {
			t.Errorf("expected non-zero time for status %q, got zero", status)
		}
	}
}

func TestParseRateLimitEvent_MalformedJSON(t *testing.T) {
	stdout := `{"type":"rate_limit_event","rate_limit_info":{"resetsAt":` // truncated
	_, got := parseRateLimitEvent(stdout)
	if !got.IsZero() {
		t.Errorf("expected zero time for malformed JSON, got %v", got)
	}
}

func TestRunSandboxOnce_UsageLimitedSetOnRateLimitEvent(t *testing.T) {
	resetsAt := time.Now().Add(1 * time.Hour).Unix()
	stdout := rateLimitJSON(resetsAt) + "\n" +
		`{"type":"result","session_id":"s1","result":"You've hit your limit","total_cost_usd":0.01,"is_error":true}`

	origRunner := SandboxRunner
	defer func() { SandboxRunner = origRunner }()
	SandboxRunner = func(_ context.Context, _ sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{Stdout: stdout, ExitCode: 0}, nil
	}

	opts := RunOpts{Prompt: "test", Role: "implementer"}
	sandboxOpts := sandbox.RunOpts{}
	logger := slog.Default()
	res, err := runSandboxOnce(context.Background(), opts, sandboxOpts, time.Now(), logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.UsageLimited {
		t.Error("expected UsageLimited=true")
	}
	if res.ResetsAt.Unix() != resetsAt {
		t.Errorf("got ResetsAt.Unix()=%d, want %d", res.ResetsAt.Unix(), resetsAt)
	}
	// ExitCode should remain 0 because UsageLimited suppresses the is_error override.
	if res.ExitCode != 0 {
		t.Errorf("expected ExitCode=0 when usage limited, got %d", res.ExitCode)
	}
}

func TestRunSandboxOnce_UsageLimitedFallback(t *testing.T) {
	stdout := `{"type":"result","session_id":"s1","result":"You've hit your limit","total_cost_usd":0.01,"is_error":true}`

	origRunner := SandboxRunner
	defer func() { SandboxRunner = origRunner }()
	SandboxRunner = func(_ context.Context, _ sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{Stdout: stdout, ExitCode: 0}, nil
	}

	opts := RunOpts{Prompt: "test", Role: "implementer"}
	sandboxOpts := sandbox.RunOpts{}
	logger := slog.Default()
	res, err := runSandboxOnce(context.Background(), opts, sandboxOpts, time.Now(), logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.UsageLimited {
		t.Error("expected UsageLimited=true via fallback text check")
	}
	if !res.ResetsAt.IsZero() {
		t.Errorf("expected ResetsAt to be zero for fallback, got %v", res.ResetsAt)
	}
}

func TestRunSandboxOnce_NotUsageLimited(t *testing.T) {
	stdout := `{"type":"result","session_id":"s1","result":"all done","total_cost_usd":0.01,"is_error":false}`

	origRunner := SandboxRunner
	defer func() { SandboxRunner = origRunner }()
	SandboxRunner = func(_ context.Context, _ sandbox.RunOpts, _ *slog.Logger) (*sandbox.RunResult, error) {
		return &sandbox.RunResult{Stdout: stdout, ExitCode: 0}, nil
	}

	opts := RunOpts{Prompt: "test", Role: "implementer"}
	sandboxOpts := sandbox.RunOpts{}
	logger := slog.Default()
	res, err := runSandboxOnce(context.Background(), opts, sandboxOpts, time.Now(), logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.UsageLimited {
		t.Error("expected UsageLimited=false for normal run")
	}
	if !res.ResetsAt.IsZero() {
		t.Errorf("expected ResetsAt to be zero for normal run, got %v", res.ResetsAt)
	}
}
