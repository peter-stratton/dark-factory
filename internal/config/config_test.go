package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }

func writeYAML(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "godark.yaml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFullConfigParse(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
max_retries: 3
build_command: "go build -o bin/ ./cmd/..."
test_command: "go test ./..."
sandbox_env:
  GOOS: linux
  GOARCH: arm64
runtime:
  name: go
  version: "1.26.0"
protected_paths:
  - tests/scenarios/
  - CLAUDE.md
scenario_dir: tests/scenarios/
review_dir: tests/review/
log_dir: logs/
docker:
  image: dark-factory-runner
  dockerfile: Dockerfile.devloop
  mount: /workspace
  user: devloop
prompts:
  implementer: prompts/implementer.md
  implementer_retry: prompts/implementer-retry.md
  reviewer: prompts/reviewer.md
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "owner/repo")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.BuildCommand != "go build -o bin/ ./cmd/..." {
		t.Errorf("BuildCommand = %q", cfg.BuildCommand)
	}
	if cfg.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q", cfg.TestCommand)
	}
	if cfg.SandboxEnv["GOOS"] != "linux" {
		t.Errorf("SandboxEnv[GOOS] = %q, want linux", cfg.SandboxEnv["GOOS"])
	}
	if cfg.SandboxEnv["GOARCH"] != "arm64" {
		t.Errorf("SandboxEnv[GOARCH] = %q, want arm64", cfg.SandboxEnv["GOARCH"])
	}
	if cfg.Runtime.Name != "go" {
		t.Errorf("Runtime.Name = %q, want go", cfg.Runtime.Name)
	}
	if cfg.Runtime.Version != "1.26.0" {
		t.Errorf("Runtime.Version = %q, want 1.26.0", cfg.Runtime.Version)
	}
	if len(cfg.ProtectedPaths) != 2 {
		t.Errorf("ProtectedPaths = %v, want 2 entries", cfg.ProtectedPaths)
	}
	if cfg.Docker.Image != "dark-factory-runner" {
		t.Errorf("Docker.Image = %q", cfg.Docker.Image)
	}
	if cfg.Docker.User != "devloop" {
		t.Errorf("Docker.User = %q", cfg.Docker.User)
	}
	if cfg.Prompts.Implementer != "prompts/implementer.md" {
		t.Errorf("Prompts.Implementer = %q", cfg.Prompts.Implementer)
	}
	if cfg.Prompts.Reviewer != "prompts/reviewer.md" {
		t.Errorf("Prompts.Reviewer = %q", cfg.Prompts.Reviewer)
	}
}

func TestRuntimeFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

runtime:
  name: flutter
  version: "3.41"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Runtime.Name != "flutter" {
		t.Errorf("Runtime.Name = %q, want flutter", cfg.Runtime.Name)
	}
	if cfg.Runtime.Version != "3.41" {
		t.Errorf("Runtime.Version = %q, want 3.41", cfg.Runtime.Version)
	}
}

func TestEmptyRuntimeIsZeroValued(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Runtime.Name != "" {
		t.Errorf("Runtime.Name = %q, want empty string", cfg.Runtime.Name)
	}
	if cfg.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty string", cfg.Runtime.Version)
	}
}

func TestSandboxEnvFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

sandbox_env:
  GOOS: linux
  GOARCH: arm64
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SandboxEnv["GOOS"] != "linux" {
		t.Errorf("SandboxEnv[GOOS] = %q, want linux", cfg.SandboxEnv["GOOS"])
	}
	if cfg.SandboxEnv["GOARCH"] != "arm64" {
		t.Errorf("SandboxEnv[GOARCH] = %q, want arm64", cfg.SandboxEnv["GOARCH"])
	}
}

func TestEmptySandboxEnvIsNil(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SandboxEnv != nil {
		t.Errorf("SandboxEnv = %v, want nil", cfg.SandboxEnv)
	}
}

func TestFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: original/repo

max_retries: 2
`)

	cfg, err := Load(path, CLIFlags{
		Repo:       strPtr("override/repo"),
		MaxRetries: intPtr(5),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Repo != "override/repo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "override/repo")
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
}

func TestMissingFileFlagsSufficient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")

	cfg, err := Load(path, CLIFlags{
		Repo: strPtr("owner/repo"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "owner/repo")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3 (default)", cfg.MaxRetries)
	}
}

func TestMissingFileFlagsInsufficient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for missing repo, got nil")
	}
	if !strings.Contains(err.Error(), "repo") {
		t.Errorf("error = %q, want mention of 'repo'", err.Error())
	}
}

func TestMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3 (default)", cfg.MaxRetries)
	}
	if cfg.ScenarioDir != "tests/scenarios/" {
		t.Errorf("ScenarioDir = %q, want default", cfg.ScenarioDir)
	}
	if cfg.ReviewDir != "tests/review/" {
		t.Errorf("ReviewDir = %q, want default", cfg.ReviewDir)
	}
	if cfg.LogDir != "logs/" {
		t.Errorf("LogDir = %q, want default", cfg.LogDir)
	}
}

func TestInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `{{{not valid yaml`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing config file") {
		t.Errorf("error = %q, want mention of parsing", err.Error())
	}
}


func TestNoSandboxDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.NoSandbox {
		t.Error("NoSandbox should default to false")
	}
}

func TestNoSandboxFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

no_sandbox: true
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoSandbox {
		t.Error("NoSandbox = false, want true from YAML")
	}
}

func TestNoSandboxFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

no_sandbox: false
`)

	cfg, err := Load(path, CLIFlags{NoSandbox: boolPtr(true)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoSandbox {
		t.Error("NoSandbox = false, want true (flag should override)")
	}
}

func TestNoMergeDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.NoMerge {
		t.Error("NoMerge should default to false")
	}
}

func TestNoMergeFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

no_merge: true
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoMerge {
		t.Error("NoMerge = false, want true from YAML")
	}
}

func TestNoMergeFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

no_merge: false
`)

	cfg, err := Load(path, CLIFlags{NoMerge: boolPtr(true)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.NoMerge {
		t.Error("NoMerge = false, want true (flag should override)")
	}
}

// TestClaudeFlagsIgnored verifies that a YAML file containing the legacy
// claude_flags field loads without error. The field is silently ignored for
// backward compatibility.
func TestClaudeFlagsIgnored(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

claude_flags: ["-v"]
`)

	_, err := Load(path, CLIFlags{})
	if err != nil {
		t.Errorf("unexpected error loading config with legacy claude_flags: %v", err)
	}
}
