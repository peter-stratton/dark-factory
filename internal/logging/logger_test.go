package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestNewLogger_FileNaming(t *testing.T) {
	dir := t.TempDir()
	fixedTime := time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC)
	timeNow = func() time.Time { return fixedTime }
	t.Cleanup(func() { timeNow = time.Now })

	_, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	expected := "run-20250615-143045.json"
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != expected {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("expected file %q, got %v", expected, names)
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	logger.Info("test message")

	entries, _ := os.ReadDir(dir)
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
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

	entries, _ := os.ReadDir(dir)
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
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
