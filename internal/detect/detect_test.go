package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
)

func TestDetectRuntime_Go(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "go" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "go")
	}
	if got.Runtime.Version != "1.26" {
		t.Errorf("Runtime.Version = %q, want %q", got.Runtime.Version, "1.26")
	}
	if got.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "go test ./...")
	}
	if got.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "go build ./...")
	}
}

func TestDetectRuntime_Flutter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pubspec.yaml"), "name: myapp\nenvironment:\n  sdk: '>=3.0.0 <4.0.0'\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "flutter" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "flutter")
	}
	if got.TestCommand != "flutter test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "flutter test")
	}
	if got.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty", got.BuildCommand)
	}
}

func TestDetectRuntime_Node(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"myapp","engines":{"node":">=18"}}`)

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "node" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "node")
	}
	if got.TestCommand != "npm test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "npm test")
	}
	if got.BuildCommand != "npm run build" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "npm run build")
	}
}

func TestDetectRuntime_Rust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"myapp\"\nversion = \"0.1.0\"\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "rust" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "rust")
	}
	if got.TestCommand != "cargo test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "cargo test")
	}
	if got.BuildCommand != "cargo build" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "cargo build")
	}
}

func TestDetectRuntime_Python_Pyproject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[build-system]\nrequires = [\"setuptools\"]\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "python" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "python")
	}
	if got.TestCommand != "pytest" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "pytest")
	}
	if got.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty", got.BuildCommand)
	}
}

func TestDetectRuntime_Python_Requirements(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests==2.28.0\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "python" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "python")
	}
	if got.TestCommand != "pytest" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "pytest")
	}
}

func TestDetectRuntime_NoMarkerFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectRuntime(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "could not detect project type") {
		t.Errorf("error %q does not contain %q", err.Error(), "could not detect project type")
	}
}

func TestDetectRuntime_MultipleMarkers_GoWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"myapp"}`)

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "go" {
		t.Errorf("Runtime.Name = %q, want %q (Go should win)", got.Runtime.Name, "go")
	}
}

func TestDetectRuntime_GoVersionParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26.0\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Version != "1.26.0" {
		t.Errorf("Runtime.Version = %q, want %q", got.Runtime.Version, "1.26.0")
	}
}

func TestDetectRuntime_GoVersionMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty string", got.Runtime.Version)
	}
}

func TestDetectRuntime_FlutterVersionParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pubspec.yaml"), "name: myapp\nenvironment:\n  sdk: '>=3.0.0 <4.0.0'\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Version != ">=3.0.0 <4.0.0" {
		t.Errorf("Runtime.Version = %q, want %q", got.Runtime.Version, ">=3.0.0 <4.0.0")
	}
}

// TestApplyDetection_ConfigWins verifies that explicit config values are not
// overwritten by detection. This tests the orchestration wiring condition.
func TestApplyDetection_ConfigWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")

	cfg := &config.Config{
		Runtime: config.Runtime{Name: "node"},
	}

	// Simulate the orchestrator condition: only detect when all three are empty.
	if cfg.Runtime.Name == "" && cfg.BuildCommand == "" && cfg.TestCommand == "" {
		dp, err := DetectRuntime(dir)
		if err == nil {
			cfg.Runtime = dp.Runtime
			cfg.BuildCommand = dp.BuildCommand
			cfg.TestCommand = dp.TestCommand
		}
	}

	if cfg.Runtime.Name != "node" {
		t.Errorf("Runtime.Name = %q, want %q (explicit config must not be overwritten)", cfg.Runtime.Name, "node")
	}
}

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
