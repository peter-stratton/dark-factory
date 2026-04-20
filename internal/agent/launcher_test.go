package agent

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
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

func TestParseTextResetTime_8pmUTC(t *testing.T) {
	got := parseTextResetTime("You've hit your limit · resets 8pm (UTC)")
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Hour() != 20 || got.Minute() != 0 {
		t.Errorf("got %v, want 20:00 UTC", got)
	}
	if got.Location() != time.UTC {
		t.Errorf("got location %v, want UTC", got.Location())
	}
}

func TestParseTextResetTime_2amUTC(t *testing.T) {
	got := parseTextResetTime("You've hit your limit · resets 2am (UTC)")
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Hour() != 2 || got.Minute() != 0 {
		t.Errorf("got %v, want 02:00 UTC", got)
	}
}

func TestParseTextResetTime_NoMatch(t *testing.T) {
	got := parseTextResetTime("some random output")
	if !got.IsZero() {
		t.Errorf("expected zero time, got %v", got)
	}
}

func TestParseTextResetTime_12pmUTC(t *testing.T) {
	got := parseTextResetTime("You've hit your limit · resets 12pm (UTC)")
	if got.IsZero() {
		t.Fatal("expected non-zero time")
	}
	if got.Hour() != 12 {
		t.Errorf("got hour %d, want 12", got.Hour())
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

func TestFilterTranscript_KeepsAssistantAndUserEvents(t *testing.T) {
	stdout := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s1"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"thinking"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo.go"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"file contents"}]}}`,
		`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"}}`,
		`CLONE_SHA=deadbeef`,
		`{"type":"result","session_id":"s1","result":"done"}`,
		``,
	}, "\n")

	compressed, err := FilterTranscript(stdout)
	if err != nil {
		t.Fatalf("FilterTranscript() error: %v", err)
	}
	if len(compressed) == 0 {
		t.Fatal("expected non-empty transcript")
	}

	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader() error: %v", err)
	}
	defer gz.Close()
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("io.ReadAll() error: %v", err)
	}

	var gotTypes []string
	scanner := bufio.NewScanner(bytes.NewReader(decompressed))
	for scanner.Scan() {
		var peek struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &peek); err != nil {
			t.Fatalf("each kept line must be valid JSON, got %q: %v", scanner.Text(), err)
		}
		gotTypes = append(gotTypes, peek.Type)
	}

	wantTypes := []string{"assistant", "assistant", "user"}
	if len(gotTypes) != len(wantTypes) {
		t.Fatalf("kept %d lines, want %d: %v", len(gotTypes), len(wantTypes), gotTypes)
	}
	for i, want := range wantTypes {
		if gotTypes[i] != want {
			t.Errorf("line %d type = %q, want %q", i, gotTypes[i], want)
		}
	}
}

func TestFilterTranscript_EmptyStdoutReturnsNil(t *testing.T) {
	got, err := FilterTranscript("")
	if err != nil {
		t.Fatalf("FilterTranscript() error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for empty stdout, got %d bytes", len(got))
	}
}

func TestFilterTranscript_NoRelevantEventsReturnsNil(t *testing.T) {
	stdout := `{"type":"system","subtype":"init"}` + "\n" +
		`{"type":"result","session_id":"s1","result":"done"}`
	got, err := FilterTranscript(stdout)
	if err != nil {
		t.Fatalf("FilterTranscript() error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil when only system/result events present, got %d bytes", len(got))
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
