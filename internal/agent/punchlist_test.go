package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/punchlist"
)

func TestExtractJSONArray_PlainJSON(t *testing.T) {
	s, ok := extractJSONArray(`["a","b"]`)
	if !ok {
		t.Fatal("expected ok=true for plain JSON array")
	}
	var got []string
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v, want [a b]", got)
	}
}

func TestExtractJSONArray_CodeFence(t *testing.T) {
	input := "```json\n[\"a\"]\n```"
	s, ok := extractJSONArray(input)
	if !ok {
		t.Fatal("expected ok=true for fenced JSON array")
	}
	var got []string
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("got %v, want [a]", got)
	}
}

func TestExtractJSONArray_SurroundingText(t *testing.T) {
	input := `Here are tests: ["a"] done`
	s, ok := extractJSONArray(input)
	if !ok {
		t.Fatal("expected ok=true for JSON embedded in text")
	}
	var got []string
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("got %v, want [a]", got)
	}
}

func TestExtractJSONArray_NestedBrackets(t *testing.T) {
	// The text contains nested brackets; depth tracking must find outermost.
	input := `text [with [nested] brackets] more`
	// This is not valid JSON so should return false (no valid JSON array found).
	_, ok := extractJSONArray(input)
	if ok {
		t.Error("expected ok=false for text with only non-JSON nested brackets")
	}
}

func TestExtractJSONArray_ValidNestedJSON(t *testing.T) {
	// Valid JSON array with nested structure.
	input := `prefix ["a","b"] suffix`
	s, ok := extractJSONArray(input)
	if !ok {
		t.Fatal("expected ok=true")
	}
	var got []string
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %v, want [a b]", got)
	}
}

func TestExtractJSONArray_NoBrackets(t *testing.T) {
	_, ok := extractJSONArray("no brackets here")
	if ok {
		t.Error("expected ok=false for text without brackets")
	}
}

func TestParseAcceptanceTests_ValidJSON(t *testing.T) {
	input := `["Test one", "Test two", "Test three"]`
	tests := parseAcceptanceTests(input, 0, testLogger(t))
	if len(tests) != 3 {
		t.Fatalf("expected 3 tests, got %d: %v", len(tests), tests)
	}
	if tests[0] != "Test one" {
		t.Errorf("tests[0] = %q, want %q", tests[0], "Test one")
	}
}

func TestParseAcceptanceTests_FencedJSON(t *testing.T) {
	input := "```json\n[\"Test one\", \"Test two\"]\n```"
	tests := parseAcceptanceTests(input, 0, testLogger(t))
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests, got %d: %v", len(tests), tests)
	}
	if tests[0] != "Test one" {
		t.Errorf("tests[0] = %q, want %q", tests[0], "Test one")
	}
}

func TestParseAcceptanceTests_FencedNoLang(t *testing.T) {
	input := "```\n[\"A\", \"B\"]\n```"
	tests := parseAcceptanceTests(input, 0, testLogger(t))
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests, got %d: %v", len(tests), tests)
	}
}

func TestParseAcceptanceTests_InvalidJSON(t *testing.T) {
	input := "This is not JSON at all"
	tests := parseAcceptanceTests(input, 0, testLogger(t))
	if tests != nil {
		t.Errorf("expected nil for invalid JSON, got %v", tests)
	}
}

func TestParseAcceptanceTests_EmptyString(t *testing.T) {
	tests := parseAcceptanceTests("", 0, testLogger(t))
	if tests != nil {
		t.Errorf("expected nil for empty string, got %v", tests)
	}
}

func TestParseAcceptanceTests_CapsAtFive(t *testing.T) {
	input := `["A","B","C","D","E","F","G"]`
	tests := parseAcceptanceTests(input, 0, testLogger(t))
	if len(tests) != 5 {
		t.Fatalf("expected 5 tests (capped), got %d: %v", len(tests), tests)
	}
}

func TestParseAcceptanceTests_EmbeddedInText(t *testing.T) {
	input := "Here are the tests:\n[\"Test one\", \"Test two\"]\nDone."
	tests := parseAcceptanceTests(input, 0, testLogger(t))
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests from embedded JSON, got %d: %v", len(tests), tests)
	}
}

func TestParseAcceptanceTests_EmptyArray(t *testing.T) {
	input := `[]`
	tests := parseAcceptanceTests(input, 0, testLogger(t))
	if len(tests) != 0 {
		t.Errorf("expected 0 tests for empty array, got %d", len(tests))
	}
}

// logCapturer is a slog.Handler that captures log records for test assertions.
type logCapturer struct {
	records []slog.Record
}

func (c *logCapturer) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (c *logCapturer) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return nil
}
func (c *logCapturer) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *logCapturer) WithGroup(_ string) slog.Handler      { return c }

func TestParseAcceptanceTests_InvalidJSON_LogsWarn(t *testing.T) {
	cap := &logCapturer{}
	logger := slog.New(cap)

	result := parseAcceptanceTests("not json at all", 0, logger)
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
	if len(cap.records) == 0 {
		t.Fatal("expected at least one log record, got none")
	}
	rec := cap.records[0]
	if rec.Level != slog.LevelWarn {
		t.Errorf("expected Warn level, got %v", rec.Level)
	}
	// Verify raw_response is included.
	found := false
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "raw_response" {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Error("expected raw_response attr in warn log, not found")
	}
}

func TestEnrichPunchlistEntries_SkipWhenNoPrompt(t *testing.T) {
	cap := &logCapturer{}
	logger := slog.New(cap)

	entries := []punchlist.Entry{
		{IssueNumber: 1, IssueTitle: "Test"},
	}
	prompts := &Prompts{Punchlist: ""}
	cfg := &config.Config{Repo: "owner/repo", NoSandbox: true}

	EnrichPunchlistEntries(context.Background(), entries, prompts, cfg, nil, logger)

	// Entry AcceptanceTests must remain nil.
	if entries[0].AcceptanceTests != nil {
		t.Errorf("expected AcceptanceTests to remain nil when no prompt, got %v", entries[0].AcceptanceTests)
	}

	// Must have logged at Info level.
	found := false
	for _, rec := range cap.records {
		if rec.Level == slog.LevelInfo && rec.Message == "punchlist enrichment skipped: no prompt configured" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Info log 'punchlist enrichment skipped: no prompt configured', not found")
	}
}

func TestGenerateAcceptanceTests_SkipWhenNoPrompt(t *testing.T) {
	cap := &logCapturer{}
	logger := slog.New(cap)

	entry := punchlist.Entry{IssueNumber: 42, IssueTitle: "Test"}
	prompts := &Prompts{Punchlist: ""}
	cfg := &config.Config{Repo: "owner/repo", NoSandbox: true}

	result := GenerateAcceptanceTests(context.Background(), entry, prompts, cfg, nil, logger)
	if result != nil {
		t.Errorf("expected nil when no prompt, got %v", result)
	}

	// Must have logged at Info level.
	found := false
	for _, rec := range cap.records {
		if rec.Level == slog.LevelInfo {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Info log when skipping due to empty prompt")
	}
}

func TestGenerateAcceptanceTests_LogsSuccessWithCount(t *testing.T) {
	cap := &logCapturer{}
	logger := slog.New(cap)

	// Stub Runner to return a valid JSON array.
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		resultJSON := `{"session_id":"","result":"[\"Test one\",\"Test two\"]","cost_usd":0,"is_error":false}`
		return []byte(resultJSON), []byte(""), 0, nil
	})
	// Stub punchlist CommandRunner (FetchPRDiff) to avoid real gh call.
	origCmd := punchlist.CommandRunner
	defer func() { punchlist.CommandRunner = origCmd }()
	punchlist.CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("diff --git a/foo.go b/foo.go\n"), nil
	}

	entry := punchlist.Entry{IssueNumber: 42, IssueTitle: "Test", PRNumber: 10}
	prompts := &Prompts{Punchlist: "Generate tests for issue {{.IssueNumber}}"}
	cfg := &config.Config{Repo: "owner/repo", NoSandbox: true}

	result := GenerateAcceptanceTests(context.Background(), entry, prompts, cfg, map[string]string{}, logger)
	if len(result) != 2 {
		t.Fatalf("expected 2 tests, got %d: %v", len(result), result)
	}

	// Must have Info log with count.
	found := false
	for _, rec := range cap.records {
		if rec.Level == slog.LevelInfo && rec.Message == "punchlist acceptance tests generated" {
			rec.Attrs(func(a slog.Attr) bool {
				if a.Key == "count" && a.Value.Int64() == 2 {
					found = true
				}
				return true
			})
			break
		}
	}
	if !found {
		t.Error("expected Info log 'punchlist acceptance tests generated' with count=2")
	}
}

func TestGenerateAcceptanceTests_LogsWarnOnUnparseable(t *testing.T) {
	cap := &logCapturer{}
	logger := slog.New(cap)

	// Stub Runner to return unparseable output.
	stubRunnerFunc(t, func(ctx context.Context, env map[string]string, name string, args ...string) ([]byte, []byte, int, error) {
		resultJSON := `{"session_id":"","result":"not a json array","cost_usd":0,"is_error":false}`
		return []byte(resultJSON), []byte(""), 0, nil
	})
	origCmd := punchlist.CommandRunner
	defer func() { punchlist.CommandRunner = origCmd }()
	punchlist.CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("diff\n"), nil
	}

	// Suppress stderr noise from os.Stderr logger.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	entry := punchlist.Entry{IssueNumber: 42, PRNumber: 10}
	prompts := &Prompts{Punchlist: "Generate tests for issue {{.IssueNumber}}"}
	cfg := &config.Config{Repo: "owner/repo", NoSandbox: true}

	result := GenerateAcceptanceTests(context.Background(), entry, prompts, cfg, map[string]string{}, logger)

	w.Close()
	os.Stderr = origStderr
	r.Close()

	if result != nil {
		t.Errorf("expected nil for unparseable output, got %v", result)
	}

	// Expect a Warn log about unparseable output.
	found := false
	for _, rec := range cap.records {
		if rec.Level == slog.LevelWarn {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Warn log when acceptance tests cannot be parsed")
	}
}
