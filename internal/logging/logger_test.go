package logging

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewLoggerFileOnly_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	_, err := NewLoggerFileOnly(dir)
	if err != nil {
		t.Fatalf("NewLoggerFileOnly() error = %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected log directory to be created")
	}
}

func TestNewLoggerFileOnly_CreatesDebugLog(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLoggerFileOnly(dir)
	if err != nil {
		t.Fatalf("NewLoggerFileOnly() error = %v", err)
	}

	logger.Info("file-only test")

	data, err := os.ReadFile(filepath.Join(dir, "debug.log"))
	if err != nil {
		t.Fatalf("reading debug.log: %v", err)
	}
	if !strings.Contains(string(data), "file-only test") {
		t.Errorf("expected debug.log to contain 'file-only test', got: %s", data)
	}
}

func TestNewLoggerFileOnly_NoStdout(t *testing.T) {
	dir := t.TempDir()

	// Capture stdout.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	logger, err := NewLoggerFileOnly(dir)
	if err != nil {
		w.Close()
		os.Stdout = origStdout
		t.Fatalf("NewLoggerFileOnly() error = %v", err)
	}

	logger.Info("should not appear on stdout")
	w.Close()

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if strings.Contains(output, "should not appear on stdout") {
		t.Errorf("NewLoggerFileOnly wrote to stdout but should not: %s", output)
	}
}

func TestNewFileLogger_CreatesDebugLog(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewFileLogger(dir)
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}

	logger.Info("per-issue test entry", "issue", 42)

	data, err := os.ReadFile(filepath.Join(dir, "debug.log"))
	if err != nil {
		t.Fatalf("reading debug.log: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nraw: %s", err, data)
	}
	if record["msg"] != "per-issue test entry" {
		t.Errorf("expected msg 'per-issue test entry', got %q", record["msg"])
	}
	if record["issue"] != float64(42) {
		t.Errorf("expected issue 42, got %v", record["issue"])
	}
}

func TestNewFileLogger_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "issues", "42")
	_, err := NewFileLogger(dir)
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "debug.log")); os.IsNotExist(err) {
		t.Fatal("expected debug.log to be created in nested directory")
	}
}

func TestNewLogger_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	_, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected log directory to be created")
	}
}

func TestNewLogger_CreatesDebugLog(t *testing.T) {
	dir := t.TempDir()
	_, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	path := filepath.Join(dir, "debug.log")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected debug.log to be created in dir")
	}
}

// TestDryRunIsolation verifies that two concurrent dry-runs using os.MkdirTemp
// each get their own private log path with no shared state.
func TestDryRunIsolation(t *testing.T) {
	type result struct {
		path string
		err  error
	}

	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dir, err := os.MkdirTemp("", "godark-log-*")
			if err != nil {
				results <- result{err: err}
				return
			}
			t.Cleanup(func() { os.RemoveAll(dir) })
			_, logErr := NewLogger(dir)
			results <- result{path: filepath.Join(dir, "debug.log"), err: logErr}
		}()
	}
	wg.Wait()
	close(results)

	var paths []string
	for r := range results {
		if r.err != nil {
			t.Fatalf("unexpected error in concurrent dry-run: %v", r.err)
		}
		paths = append(paths, r.path)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 log paths, got %d", len(paths))
	}
	if paths[0] == paths[1] {
		t.Error("concurrent dry-runs share a log path — os.MkdirTemp must return unique directories")
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	logger.Info("test message")

	data, err := os.ReadFile(filepath.Join(dir, "debug.log"))
	if err != nil {
		t.Fatalf("reading debug.log: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nraw: %s", err, data)
	}

	if _, ok := record["time"]; !ok {
		t.Error("expected 'time' field in JSON output")
	}
	if _, ok := record["level"]; !ok {
		t.Error("expected 'level' field in JSON output")
	}
	if _, ok := record["msg"]; !ok {
		t.Error("expected 'msg' field in JSON output")
	}
	if record["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got %q", record["msg"])
	}
}

func TestLogger_StructuredFields(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	child := logger.With("component", "orchestrator", "issue_number", 5)
	child.Info("processing issue")

	data, err := os.ReadFile(filepath.Join(dir, "debug.log"))
	if err != nil {
		t.Fatalf("reading debug.log: %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nraw: %s", err, data)
	}

	if record["component"] != "orchestrator" {
		t.Errorf("expected component 'orchestrator', got %v", record["component"])
	}
	if record["issue_number"] != float64(5) {
		t.Errorf("expected issue_number 5, got %v", record["issue_number"])
	}
}

// TestLoggerWrites verifies that log entries appear in debug.log.
func TestLoggerWrites(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	logger.Debug("debug entry")
	logger.Info("info entry")
	logger.Warn("warn entry")

	data, err := os.ReadFile(filepath.Join(dir, "debug.log"))
	if err != nil {
		t.Fatalf("reading debug.log: %v", err)
	}
	content := string(data)
	for _, want := range []string{"debug entry", "info entry", "warn entry"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in debug.log, got:\n%s", want, content)
		}
	}
}

func TestVerdictColor(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{`msg="reviewer finished" verdict=APPROVED`, ansiGreen},
		{`msg="reviewer finished" verdict="APPROVED"`, ansiGreen},
		{`msg="reviewer finished" verdict=CHANGES_REQUESTED`, ansiYellow},
		{`msg="quality reviewer finished" verdict=CHANGES_REQUESTED`, ansiYellow},
		{`msg="verify step passed"`, ansiGreen},
		{`msg="container finished" timed_out=true`, ansiRed},
		{`msg="container finished" timed_out="true"`, ansiRed},
		{`msg="processing issue"`, ""},
		{`msg="running container"`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := verdictColor(tt.line)
			if got != tt.want {
				t.Errorf("verdictColor(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestColorWriter(t *testing.T) {
	var buf bytes.Buffer
	cw := &colorWriter{w: &buf}

	line := `time=2026-03-08T17:00:00 level=INFO msg="reviewer finished" verdict=APPROVED` + "\n"
	n, err := cw.Write([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(line) {
		t.Errorf("Write returned %d, want %d", n, len(line))
	}
	out := buf.String()
	if !strings.HasPrefix(out, ansiBold+ansiGreen) {
		t.Errorf("expected ANSI green prefix, got: %q", out[:20])
	}
	if !strings.HasSuffix(out, ansiReset) {
		t.Errorf("expected ANSI reset suffix, got: %q", out[len(out)-10:])
	}
}

func TestColorWriter_NoColor(t *testing.T) {
	var buf bytes.Buffer
	cw := &colorWriter{w: &buf}

	line := `time=2026-03-08T17:00:00 level=INFO msg="processing issue"` + "\n"
	cw.Write([]byte(line))
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected no ANSI codes for plain line, got: %q", buf.String())
	}
}

func TestLogger_StdoutOutput(t *testing.T) {
	dir := t.TempDir()

	// Capture stdout
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	logger, err := NewLogger(dir)
	if err != nil {
		w.Close()
		os.Stdout = origStdout
		t.Fatalf("NewLogger() error = %v", err)
	}

	logger.Info("hello stdout")
	w.Close()

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "hello stdout") {
		t.Errorf("expected stdout to contain 'hello stdout', got: %s", output)
	}
}
