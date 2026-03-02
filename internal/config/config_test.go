package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

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
milestone: "Phase 1"
max_retries: 3
issue: 5
claude_flags: ["-p", "--dangerously-skip-permissions"]
build_command: "go build -o bin/ ./cmd/..."
test_command: "go test ./..."
cross_compile:
  GOOS: linux
  GOARCH: arm64
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
	if cfg.Milestone != "Phase 1" {
		t.Errorf("Milestone = %q, want %q", cfg.Milestone, "Phase 1")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.Issue != 5 {
		t.Errorf("Issue = %d, want 5", cfg.Issue)
	}
	if len(cfg.ClaudeFlags) != 2 || cfg.ClaudeFlags[0] != "-p" {
		t.Errorf("ClaudeFlags = %v, want [-p --dangerously-skip-permissions]", cfg.ClaudeFlags)
	}
	if cfg.BuildCommand != "go build -o bin/ ./cmd/..." {
		t.Errorf("BuildCommand = %q", cfg.BuildCommand)
	}
	if cfg.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q", cfg.TestCommand)
	}
	if cfg.CrossCompile.GOOS != "linux" {
		t.Errorf("CrossCompile.GOOS = %q, want linux", cfg.CrossCompile.GOOS)
	}
	if cfg.CrossCompile.GOARCH != "arm64" {
		t.Errorf("CrossCompile.GOARCH = %q, want arm64", cfg.CrossCompile.GOARCH)
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

func TestFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: original/repo
milestone: "Phase 1"
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
	if cfg.Milestone != "Phase 1" {
		t.Errorf("Milestone = %q, want %q (should be preserved from file)", cfg.Milestone, "Phase 1")
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
}

func TestMissingFileFlagsSufficient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.yaml")

	cfg, err := Load(path, CLIFlags{
		Repo:      strPtr("owner/repo"),
		Milestone: strPtr("Phase 1"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "owner/repo")
	}
	if cfg.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2 (default)", cfg.MaxRetries)
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
milestone: "Phase 1"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2 (default)", cfg.MaxRetries)
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

func TestMissingMilestoneAndIssue(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `repo: owner/repo`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for missing milestone/issue, got nil")
	}
	if !strings.Contains(err.Error(), "milestone") {
		t.Errorf("error = %q, want mention of 'milestone'", err.Error())
	}
}

func TestIssueAloneSuffices(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `repo: owner/repo`)

	cfg, err := Load(path, CLIFlags{Issue: intPtr(42)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Issue != 42 {
		t.Errorf("Issue = %d, want 42", cfg.Issue)
	}
}
