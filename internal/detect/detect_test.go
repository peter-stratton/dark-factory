package detect

import (
	"log/slog"
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

func TestDetectRuntime_VersionParsedFromGoMod(t *testing.T) {
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

func TestDetectRuntime_VersionMissingFromGoMod(t *testing.T) {
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

// TestApplyToConfig_ConfigWins verifies that ApplyToConfig does not overwrite
// explicit config values.
func TestApplyToConfig_ConfigWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")

	cfg := &config.Config{
		Runtime: config.Runtime{Name: "node"},
	}
	logger := slog.Default()

	ApplyToConfig(cfg, dir, logger)

	if cfg.Runtime.Name != "node" {
		t.Errorf("Runtime.Name = %q, want %q (explicit config must not be overwritten)", cfg.Runtime.Name, "node")
	}
}

// TestApplyToConfig_DetectionApplied verifies that ApplyToConfig applies
// detected values when no runtime or commands are explicitly configured.
func TestApplyToConfig_DetectionApplied(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")

	cfg := &config.Config{}
	logger := slog.Default()

	ApplyToConfig(cfg, dir, logger)

	if cfg.Runtime.Name != "go" {
		t.Errorf("Runtime.Name = %q, want %q", cfg.Runtime.Name, "go")
	}
	if cfg.Runtime.Version != "1.26" {
		t.Errorf("Runtime.Version = %q, want %q", cfg.Runtime.Version, "1.26")
	}
	if cfg.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want %q", cfg.TestCommand, "go test ./...")
	}
	if cfg.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q, want %q", cfg.BuildCommand, "go build ./...")
	}
}

// TestApplyToConfig_ExplicitCommandsWin verifies that ApplyToConfig does not
// overwrite an explicitly configured test_command or build_command.
func TestApplyToConfig_ExplicitCommandsWin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")

	cfg := &config.Config{
		TestCommand: "make test",
	}
	logger := slog.Default()

	ApplyToConfig(cfg, dir, logger)

	if cfg.TestCommand != "make test" {
		t.Errorf("TestCommand = %q, want %q (explicit command must not be overwritten)", cfg.TestCommand, "make test")
	}
	// Runtime should also not be applied when any command is set.
	if cfg.Runtime.Name != "" {
		t.Errorf("Runtime.Name = %q, want empty (should not apply when commands configured)", cfg.Runtime.Name)
	}
}

// TestApplyToConfig_DetectionFailure_NoError verifies that ApplyToConfig
// proceeds without error when no marker files are found.
func TestApplyToConfig_DetectionFailure_NoError(t *testing.T) {
	dir := t.TempDir() // empty directory — no marker files

	cfg := &config.Config{}
	logger := slog.Default()

	// Should not panic or crash; config remains unchanged.
	ApplyToConfig(cfg, dir, logger)

	if cfg.Runtime.Name != "" {
		t.Errorf("Runtime.Name = %q, want empty on detection failure", cfg.Runtime.Name)
	}
	if cfg.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty on detection failure", cfg.BuildCommand)
	}
	if cfg.TestCommand != "" {
		t.Errorf("TestCommand = %q, want empty on detection failure", cfg.TestCommand)
	}
}

// TestApplyToConfig_Flutter_EndToEnd verifies the full detection path for
// Flutter repos: ApplyToConfig populates Runtime{Name:"flutter"} with no
// build command and "flutter test" as the test command.
func TestApplyToConfig_Flutter_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pubspec.yaml"), "name: myapp\nenvironment:\n  sdk: '>=3.0.0 <4.0.0'\n")

	cfg := &config.Config{}
	logger := slog.Default()

	ApplyToConfig(cfg, dir, logger)

	if cfg.Runtime.Name != "flutter" {
		t.Errorf("Runtime.Name = %q, want flutter", cfg.Runtime.Name)
	}
	if cfg.TestCommand != "flutter test" {
		t.Errorf("TestCommand = %q, want %q", cfg.TestCommand, "flutter test")
	}
	if cfg.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty for flutter", cfg.BuildCommand)
	}
}

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
