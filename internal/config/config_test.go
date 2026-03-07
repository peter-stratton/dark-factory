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
}

// TestConfigWithoutLogDir verifies that a YAML file without log_dir loads without error.
func TestConfigWithoutLogDir(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	_, err := Load(path, CLIFlags{})
	if err != nil {
		t.Errorf("unexpected error loading config without log_dir: %v", err)
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

func TestAuthPreferenceDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthPreference != "oauth" {
		t.Errorf("AuthPreference = %q, want %q", cfg.AuthPreference, "oauth")
	}
}

func TestAuthPreferenceOAuthFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
auth_preference: oauth
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthPreference != "oauth" {
		t.Errorf("AuthPreference = %q, want %q", cfg.AuthPreference, "oauth")
	}
}

func TestAuthPreferenceAPIKeyFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
auth_preference: api_key
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthPreference != "api_key" {
		t.Errorf("AuthPreference = %q, want %q", cfg.AuthPreference, "api_key")
	}
}

func TestAuthPreferenceInvalidValue(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
auth_preference: token
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for invalid auth_preference, got nil")
	}
	if !strings.Contains(err.Error(), "auth_preference") {
		t.Errorf("error = %q, want mention of 'auth_preference'", err.Error())
	}
}

func TestQualityConfigParsed(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
quality:
  min_review_cost_usd: 0.10
  min_review_duration_seconds: 30
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Quality.MinReviewCostUSD != 0.10 {
		t.Errorf("Quality.MinReviewCostUSD = %v, want 0.10", cfg.Quality.MinReviewCostUSD)
	}
	if cfg.Quality.MinReviewDurationSeconds != 30 {
		t.Errorf("Quality.MinReviewDurationSeconds = %d, want 30", cfg.Quality.MinReviewDurationSeconds)
	}
}

func TestQualityConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Quality.MinReviewCostUSD != 0 {
		t.Errorf("Quality.MinReviewCostUSD = %v, want 0 (default/disabled)", cfg.Quality.MinReviewCostUSD)
	}
	if cfg.Quality.MinReviewDurationSeconds != 0 {
		t.Errorf("Quality.MinReviewDurationSeconds = %d, want 0 (default/disabled)", cfg.Quality.MinReviewDurationSeconds)
	}
}

func TestQualityConfigPartial(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
quality:
  min_review_cost_usd: 0.10
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Quality.MinReviewCostUSD != 0.10 {
		t.Errorf("Quality.MinReviewCostUSD = %v, want 0.10", cfg.Quality.MinReviewCostUSD)
	}
	if cfg.Quality.MinReviewDurationSeconds != 0 {
		t.Errorf("Quality.MinReviewDurationSeconds = %d, want 0 (unset)", cfg.Quality.MinReviewDurationSeconds)
	}
}

func TestArchitectureJSONDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ArchitectureJSON != "docs/architecture.json" {
		t.Errorf("ArchitectureJSON = %q, want %q", cfg.ArchitectureJSON, "docs/architecture.json")
	}
}

func TestArchitectureDocDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ArchitectureDoc != "docs/architecture.md" {
		t.Errorf("ArchitectureDoc = %q, want %q", cfg.ArchitectureDoc, "docs/architecture.md")
	}
}

func TestConventionsDocDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConventionsDoc != "docs/conventions.md" {
		t.Errorf("ConventionsDoc = %q, want %q", cfg.ConventionsDoc, "docs/conventions.md")
	}
}

func TestArchitectureDocOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
architecture_doc: custom/arch.md
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ArchitectureDoc != "custom/arch.md" {
		t.Errorf("ArchitectureDoc = %q, want %q", cfg.ArchitectureDoc, "custom/arch.md")
	}
}

func TestConventionsDocOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
conventions_doc: custom/conventions.md
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConventionsDoc != "custom/conventions.md" {
		t.Errorf("ConventionsDoc = %q, want %q", cfg.ConventionsDoc, "custom/conventions.md")
	}
}

func TestEnforceArchitectureDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EnforceArchitecture {
		t.Error("EnforceArchitecture should default to false")
	}
}

func TestEnforceArchitectureFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
enforce_architecture: true
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.EnforceArchitecture {
		t.Error("EnforceArchitecture = false, want true from YAML")
	}
}

func TestVerifyDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LintCommand != "" {
		t.Errorf("LintCommand = %q, want empty string", cfg.LintCommand)
	}
	if cfg.Verify.MaxFixAttempts != 2 {
		t.Errorf("Verify.MaxFixAttempts = %d, want 2", cfg.Verify.MaxFixAttempts)
	}
	if !cfg.Verify.Blocking {
		t.Error("Verify.Blocking = false, want true")
	}
}

func TestLintCommandFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
lint_command: "./scripts/lint.sh"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LintCommand != "./scripts/lint.sh" {
		t.Errorf("LintCommand = %q, want %q", cfg.LintCommand, "./scripts/lint.sh")
	}
}

func TestVerifyBlockFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
verify:
  max_fix_attempts: 5
  blocking: false
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Verify.MaxFixAttempts != 5 {
		t.Errorf("Verify.MaxFixAttempts = %d, want 5", cfg.Verify.MaxFixAttempts)
	}
	if cfg.Verify.Blocking {
		t.Error("Verify.Blocking = true, want false")
	}
}

func TestVerifyFixPromptPathFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
prompts:
  verify_fix: "custom/fix.txt"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Prompts.VerifyFix != "custom/fix.txt" {
		t.Errorf("Prompts.VerifyFix = %q, want %q", cfg.Prompts.VerifyFix, "custom/fix.txt")
	}
}

func TestDeniedCommandsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defaults := []string{"rm -rf", "git push --force", "git push -f", "git reset --hard", "git clean -f"}
	if len(cfg.DeniedCommands) != len(defaults) {
		t.Fatalf("DeniedCommands len = %d, want %d", len(cfg.DeniedCommands), len(defaults))
	}
	for i, want := range defaults {
		if cfg.DeniedCommands[i] != want {
			t.Errorf("DeniedCommands[%d] = %q, want %q", i, cfg.DeniedCommands[i], want)
		}
	}
}

func TestDeniedCommandsFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
denied_commands:
  - "rm -rf"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DeniedCommands) != 1 {
		t.Fatalf("DeniedCommands len = %d, want 1", len(cfg.DeniedCommands))
	}
	if cfg.DeniedCommands[0] != "rm -rf" {
		t.Errorf("DeniedCommands[0] = %q, want %q", cfg.DeniedCommands[0], "rm -rf")
	}
}

func TestDeniedCommandsEmptyFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
denied_commands: []
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DeniedCommands) != 0 {
		t.Errorf("DeniedCommands = %v, want empty slice", cfg.DeniedCommands)
	}
}

func TestGenerateCommandDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GenerateCommand != "" {
		t.Errorf("GenerateCommand = %q, want empty string", cfg.GenerateCommand)
	}
}

func TestGeneratedPathsDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GeneratedPaths != nil {
		t.Errorf("GeneratedPaths = %v, want nil", cfg.GeneratedPaths)
	}
}

func TestGenerateCommandFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
generate_command: "make generate"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GenerateCommand != "make generate" {
		t.Errorf("GenerateCommand = %q, want %q", cfg.GenerateCommand, "make generate")
	}
}

func TestGeneratedPathsDirectoriesFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
generated_paths:
  - service/api/grpc/gen/
  - service/test/mocks/
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"service/api/grpc/gen/", "service/test/mocks/"}
	if len(cfg.GeneratedPaths) != len(want) {
		t.Fatalf("GeneratedPaths len = %d, want %d", len(cfg.GeneratedPaths), len(want))
	}
	for i, w := range want {
		if cfg.GeneratedPaths[i] != w {
			t.Errorf("GeneratedPaths[%d] = %q, want %q", i, cfg.GeneratedPaths[i], w)
		}
	}
}

func TestGeneratedPathsGlobsFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
generated_paths:
  - "**/*.freezed.dart"
  - "**/*.g.dart"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"**/*.freezed.dart", "**/*.g.dart"}
	if len(cfg.GeneratedPaths) != len(want) {
		t.Fatalf("GeneratedPaths len = %d, want %d", len(cfg.GeneratedPaths), len(want))
	}
	for i, w := range want {
		if cfg.GeneratedPaths[i] != w {
			t.Errorf("GeneratedPaths[%d] = %q, want %q", i, cfg.GeneratedPaths[i], w)
		}
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
