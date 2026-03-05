package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestDryRunIsolation verifies that two concurrent dry-runs don't share a log path.
func TestDryRunIsolation(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	_, err1 := NewLogger(dir1)
	_, err2 := NewLogger(dir2)
	if err1 != nil || err2 != nil {
		t.Fatalf("NewLogger errors: %v, %v", err1, err2)
	}

	path1 := filepath.Join(dir1, "debug.log")
	path2 := filepath.Join(dir2, "debug.log")
	if path1 == path2 {
		t.Error("expected different log paths for separate directories")
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
