package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
