package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

func TestAutoMergeDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge.Feature != "none" {
		t.Errorf("AutoMerge.Feature = %q, want %q", cfg.AutoMerge.Feature, "none")
	}
	if cfg.AutoMerge.Rollup != "manual" {
		t.Errorf("AutoMerge.Rollup = %q, want %q", cfg.AutoMerge.Rollup, "manual")
	}
}

func TestAutoMergeFeatureValidValues(t *testing.T) {
	for _, value := range []string{"none", "low_risk", "all"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			path := writeYAML(t, dir, "repo: owner/repo\nauto_merge:\n  feature: "+value+"\n")
			cfg, err := Load(path, CLIFlags{})
			if err != nil {
				t.Fatalf("unexpected error for auto_merge.feature=%q: %v", value, err)
			}
			if cfg.AutoMerge.Feature != FeatureMergeStrategy(value) {
				t.Errorf("AutoMerge.Feature = %q, want %q", cfg.AutoMerge.Feature, value)
			}
		})
	}
}

func TestAutoMergeRollupValidValues(t *testing.T) {
	for _, value := range []string{"none", "manual", "auto"} {
		t.Run(value, func(t *testing.T) {
			dir := t.TempDir()
			path := writeYAML(t, dir, "repo: owner/repo\nauto_merge:\n  rollup: "+value+"\n")
			cfg, err := Load(path, CLIFlags{})
			if err != nil {
				t.Fatalf("unexpected error for auto_merge.rollup=%q: %v", value, err)
			}
			if cfg.AutoMerge.Rollup != RollupMergeStrategy(value) {
				t.Errorf("AutoMerge.Rollup = %q, want %q", cfg.AutoMerge.Rollup, value)
			}
		})
	}
}

func TestAutoMergeFeatureInvalidValue(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
auto_merge:
  feature: always
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for invalid auto_merge.feature, got nil")
	}
	if !strings.Contains(err.Error(), "auto_merge.feature") {
		t.Errorf("error = %q, want mention of 'auto_merge.feature'", err.Error())
	}
}

func TestAutoMergeRollupInvalidValue(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
auto_merge:
  rollup: merge
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for invalid auto_merge.rollup, got nil")
	}
	if !strings.Contains(err.Error(), "auto_merge.rollup") {
		t.Errorf("error = %q, want mention of 'auto_merge.rollup'", err.Error())
	}
}

func TestAutoMergeFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo

auto_merge:
  feature: none
  rollup: none
`)

	feat := "all"
	rollup := "auto"
	cfg, err := Load(path, CLIFlags{AutoMergeFeature: &feat, AutoMergeRollup: &rollup})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge.Feature != "all" {
		t.Errorf("AutoMerge.Feature = %q, want %q (flag should override)", cfg.AutoMerge.Feature, "all")
	}
	if cfg.AutoMerge.Rollup != "auto" {
		t.Errorf("AutoMerge.Rollup = %q, want %q (flag should override)", cfg.AutoMerge.Rollup, "auto")
	}
}

func TestFeatureMergeStrategyValid(t *testing.T) {
	valid := []FeatureMergeStrategy{MergeNone, MergeLowRisk, MergeAll}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("FeatureMergeStrategy(%q).Valid() = false, want true", s)
		}
	}
	if FeatureMergeStrategy("invalid").Valid() {
		t.Errorf("FeatureMergeStrategy(\"invalid\").Valid() = true, want false")
	}
	if FeatureMergeStrategy("").Valid() {
		t.Errorf("FeatureMergeStrategy(\"\").Valid() = true, want false")
	}
}

func TestRollupMergeStrategyValid(t *testing.T) {
	valid := []RollupMergeStrategy{RollupNone, RollupManual, RollupAuto}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("RollupMergeStrategy(%q).Valid() = false, want true", s)
		}
	}
	if RollupMergeStrategy("invalid").Valid() {
		t.Errorf("RollupMergeStrategy(\"invalid\").Valid() = true, want false")
	}
	if RollupMergeStrategy("").Valid() {
		t.Errorf("RollupMergeStrategy(\"\").Valid() = true, want false")
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

func TestReconPromptPathFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
prompts:
  recon: "custom/recon.txt"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Prompts.Recon != "custom/recon.txt" {
		t.Errorf("Prompts.Recon = %q, want %q", cfg.Prompts.Recon, "custom/recon.txt")
	}
}

func TestReconPromptPathDefaultEmpty(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Prompts.Recon != "" {
		t.Errorf("Prompts.Recon = %q, want empty string", cfg.Prompts.Recon)
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

// --- Module tests ---

func TestModulesDefaultNil(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Modules != nil {
		t.Errorf("Modules = %v, want nil", cfg.Modules)
	}
}

func TestSingleModuleConfigNoModulesKey(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
build_command: "go build ./..."
test_command: "go test ./..."
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Modules != nil {
		t.Errorf("Modules = %v, want nil (single-module mode)", cfg.Modules)
	}
	if cfg.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q, want %q", cfg.BuildCommand, "go build ./...")
	}
}

func TestTwoModulesParseCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
modules:
  service:
    build_command: "go build ./..."
  admin-cli:
    depends_on: [service]
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Modules) != 2 {
		t.Fatalf("Modules len = %d, want 2", len(cfg.Modules))
	}
	svc, ok := cfg.Modules["service"]
	if !ok {
		t.Fatal("expected module 'service' to exist")
	}
	if svc.BuildCommand != "go build ./..." {
		t.Errorf("service.BuildCommand = %q, want %q", svc.BuildCommand, "go build ./...")
	}
	admin, ok := cfg.Modules["admin-cli"]
	if !ok {
		t.Fatal("expected module 'admin-cli' to exist")
	}
	if len(admin.DependsOn) != 1 || admin.DependsOn[0] != "service" {
		t.Errorf("admin-cli.DependsOn = %v, want [service]", admin.DependsOn)
	}
}

func TestModulesUnknownDependencyFails(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
modules:
  service:
    depends_on: [nonexistent]
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for unknown depends_on, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error = %q, want mention of 'nonexistent'", err.Error())
	}
}

func TestModulesCycleDetected(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
modules:
  a:
    depends_on: [b]
  b:
    depends_on: [a]
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for cycle in dependencies, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want mention of 'cycle'", err.Error())
	}
}

func TestModulesPerModuleCommandsIndependent(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
modules:
  service:
    build_command: "go build ./service/..."
    test_command: "go test ./service/..."
    lint_command: "golangci-lint run ./service/..."
    generate_command: "go generate ./service/..."
  admin-cli:
    build_command: "go build ./admin/..."
    test_command: "go test ./admin/..."
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Modules["service"]
	if svc.BuildCommand != "go build ./service/..." {
		t.Errorf("service.BuildCommand = %q", svc.BuildCommand)
	}
	if svc.TestCommand != "go test ./service/..." {
		t.Errorf("service.TestCommand = %q", svc.TestCommand)
	}
	if svc.LintCommand != "golangci-lint run ./service/..." {
		t.Errorf("service.LintCommand = %q", svc.LintCommand)
	}
	if svc.GenerateCommand != "go generate ./service/..." {
		t.Errorf("service.GenerateCommand = %q", svc.GenerateCommand)
	}
	admin := cfg.Modules["admin-cli"]
	if admin.BuildCommand != "go build ./admin/..." {
		t.Errorf("admin-cli.BuildCommand = %q", admin.BuildCommand)
	}
	if admin.TestCommand != "go test ./admin/..." {
		t.Errorf("admin-cli.TestCommand = %q", admin.TestCommand)
	}
}

func TestModulesLongChainNoCycle(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
modules:
  a:
    depends_on: []
  b:
    depends_on: [a]
  c:
    depends_on: [b]
`)

	_, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error for valid chain a->b->c: %v", err)
	}
}

func TestModulesSelfCycleDetected(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
modules:
  a:
    depends_on: [a]
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for self-cycle, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want mention of 'cycle'", err.Error())
	}
}

func TestRequiredEnvDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RequiredEnv != nil {
		t.Errorf("RequiredEnv = %v, want nil", cfg.RequiredEnv)
	}
}

func TestRequiredEnvFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
required_env:
  - CLOUDSMITH_TOKEN
  - PUBSUB_EMULATOR_HOST
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"CLOUDSMITH_TOKEN", "PUBSUB_EMULATOR_HOST"}
	if len(cfg.RequiredEnv) != len(want) {
		t.Fatalf("RequiredEnv len = %d, want %d", len(cfg.RequiredEnv), len(want))
	}
	for i, w := range want {
		if cfg.RequiredEnv[i] != w {
			t.Errorf("RequiredEnv[%d] = %q, want %q", i, cfg.RequiredEnv[i], w)
		}
	}
}

// --- WaitForChecks tests ---

func TestWaitForChecksDefaultNil(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WaitForChecks != nil {
		t.Errorf("WaitForChecks = %v, want nil", cfg.WaitForChecks)
	}
}

func TestWaitForChecksValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
wait_for_checks:
  timeout: "10m"
  required:
    - golangci-lint
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WaitForChecks == nil {
		t.Fatal("WaitForChecks = nil, want non-nil")
	}
	if cfg.WaitForChecks.Timeout != "10m" {
		t.Errorf("WaitForChecks.Timeout = %q, want %q", cfg.WaitForChecks.Timeout, "10m")
	}
	if len(cfg.WaitForChecks.Required) != 1 || cfg.WaitForChecks.Required[0] != "golangci-lint" {
		t.Errorf("WaitForChecks.Required = %v, want [golangci-lint]", cfg.WaitForChecks.Required)
	}
}

func TestWaitForChecksMultipleRequired(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
wait_for_checks:
  timeout: "10m"
  required:
    - lint
    - test
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WaitForChecks == nil {
		t.Fatal("WaitForChecks = nil, want non-nil")
	}
	want := []string{"lint", "test"}
	if len(cfg.WaitForChecks.Required) != len(want) {
		t.Fatalf("WaitForChecks.Required len = %d, want %d", len(cfg.WaitForChecks.Required), len(want))
	}
	for i, w := range want {
		if cfg.WaitForChecks.Required[i] != w {
			t.Errorf("WaitForChecks.Required[%d] = %q, want %q", i, cfg.WaitForChecks.Required[i], w)
		}
	}
}

func TestWaitForChecksInvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
wait_for_checks:
  timeout: "not-a-duration"
  required:
    - lint
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for invalid timeout, got nil")
	}
	if !strings.Contains(err.Error(), "wait_for_checks.timeout") {
		t.Errorf("error = %q, want mention of 'wait_for_checks.timeout'", err.Error())
	}
}

func TestWaitForChecksEmptyRequired(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
wait_for_checks:
  timeout: "5m"
  required: []
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for empty required, got nil")
	}
	if !strings.Contains(err.Error(), "wait_for_checks.required") {
		t.Errorf("error = %q, want mention of 'wait_for_checks.required'", err.Error())
	}
}

func TestWaitForChecksZeroTimeout(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
wait_for_checks:
  timeout: "0"
  required:
    - lint
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for zero timeout, got nil")
	}
	if !strings.Contains(err.Error(), "wait_for_checks.timeout") {
		t.Errorf("error = %q, want mention of 'wait_for_checks.timeout'", err.Error())
	}
}

func TestWaitForChecksNegativeTimeout(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
wait_for_checks:
  timeout: "-5m"
  required:
    - lint
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for negative timeout, got nil")
	}
	if !strings.Contains(err.Error(), "wait_for_checks.timeout") {
		t.Errorf("error = %q, want mention of 'wait_for_checks.timeout'", err.Error())
	}
}

// --- Watch tests ---

func TestWatchDefaultNil(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Watch != nil {
		t.Errorf("Watch = %v, want nil", cfg.Watch)
	}
}

func TestWatchValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
watch:
  poll_interval: "30s"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Watch == nil {
		t.Fatal("Watch = nil, want non-nil")
	}
	if cfg.Watch.PollInterval != "30s" {
		t.Errorf("Watch.PollInterval = %q, want %q", cfg.Watch.PollInterval, "30s")
	}
}

func TestWatchInvalidPollInterval(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
watch:
  poll_interval: "not-a-duration"
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for invalid poll_interval, got nil")
	}
	if !strings.Contains(err.Error(), "watch.poll_interval") {
		t.Errorf("error = %q, want mention of 'watch.poll_interval'", err.Error())
	}
}

func TestWatchZeroPollInterval(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
watch:
  poll_interval: "0s"
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for zero poll_interval, got nil")
	}
	if !strings.Contains(err.Error(), "watch.poll_interval") {
		t.Errorf("error = %q, want mention of 'watch.poll_interval'", err.Error())
	}
}

func TestWatchNegativePollInterval(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
watch:
  poll_interval: "-30s"
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for negative poll_interval, got nil")
	}
	if !strings.Contains(err.Error(), "watch.poll_interval") {
		t.Errorf("error = %q, want mention of 'watch.poll_interval'", err.Error())
	}
}

func TestWatchNotConfigured(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
build_command: "go build ./..."
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Watch != nil {
		t.Errorf("Watch = %v, want nil (not configured)", cfg.Watch)
	}
}

// --- RiskThresholds tests ---

func TestRiskThresholdsDefaultNil(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RiskThresholds != nil {
		t.Errorf("RiskThresholds = %v, want nil", cfg.RiskThresholds)
	}
}

func TestRiskThresholdsValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
risk_thresholds:
  max_lines: 100
  max_files: 5
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RiskThresholds == nil {
		t.Fatal("RiskThresholds = nil, want non-nil")
	}
	if cfg.RiskThresholds.MaxLines != 100 {
		t.Errorf("RiskThresholds.MaxLines = %d, want 100", cfg.RiskThresholds.MaxLines)
	}
	if cfg.RiskThresholds.MaxFiles != 5 {
		t.Errorf("RiskThresholds.MaxFiles = %d, want 5", cfg.RiskThresholds.MaxFiles)
	}
}

func TestRiskThresholdsZeroLines(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
risk_thresholds:
  max_lines: 0
  max_files: 5
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for max_lines: 0, got nil")
	}
	if !strings.Contains(err.Error(), "risk_thresholds.max_lines") {
		t.Errorf("error = %q, want mention of 'risk_thresholds.max_lines'", err.Error())
	}
}

func TestRiskThresholdsNegativeFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
risk_thresholds:
  max_lines: 100
  max_files: -1
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for max_files: -1, got nil")
	}
	if !strings.Contains(err.Error(), "risk_thresholds.max_files") {
		t.Errorf("error = %q, want mention of 'risk_thresholds.max_files'", err.Error())
	}
}

func TestRiskThresholdsNotConfigured(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
build_command: "go build ./..."
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RiskThresholds != nil {
		t.Errorf("RiskThresholds = %v, want nil (not configured)", cfg.RiskThresholds)
	}
}

// --- Notify tests ---

func TestNotifyDefaultNil(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Notify != nil {
		t.Errorf("Notify = %v, want nil", cfg.Notify)
	}
}

func TestNotifyValidTelegramConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
notify:
  - provider: telegram
    events: [run_complete, abort]
    settings:
      bot_token: mytoken
      chat_id: "123456789"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error for valid telegram config: %v", err)
	}
	if len(cfg.Notify) != 1 {
		t.Fatalf("Notify len = %d, want 1", len(cfg.Notify))
	}
	n := cfg.Notify[0]
	if n.Provider != "telegram" {
		t.Errorf("Notify[0].Provider = %q, want %q", n.Provider, "telegram")
	}
	if len(n.Events) != 2 {
		t.Errorf("Notify[0].Events = %v, want [run_complete abort]", n.Events)
	}
}

func TestNotifyUnknownProvider(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
notify:
  - provider: carrier_pigeon
    events: [run_complete]
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "carrier_pigeon") {
		t.Errorf("error = %q, want mention of 'carrier_pigeon'", err.Error())
	}
}

func TestNotifyUnknownEvent(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
notify:
  - provider: carrier_pigeon
    events: [unknown_event]
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for unknown event, got nil")
	}
	if !strings.Contains(err.Error(), "unknown_event") {
		t.Errorf("error = %q, want mention of 'unknown_event'", err.Error())
	}
}

func TestNotifyEmptyListValid(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
notify: []
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error for empty notify list: %v", err)
	}
	if len(cfg.Notify) != 0 {
		t.Errorf("Notify len = %d, want 0", len(cfg.Notify))
	}
}

func TestNotifyEnvExpansion(t *testing.T) {
	t.Setenv("FOO", "expanded_value")
	cfg := &Config{
		Notify: []NotifyProviderConfig{
			{Provider: "any", Events: []string{"run_complete"}, Settings: map[string]string{"bot_token": "${FOO}"}},
		},
	}
	expandNotifySettings(cfg)
	got := cfg.Notify[0].Settings["bot_token"]
	if got != "expanded_value" {
		t.Errorf("Settings[bot_token] = %q, want %q", got, "expanded_value")
	}
}

func TestNotifyMissingEnvVarResolvesToEmpty(t *testing.T) {
	// MISSING_NOTIFY_VAR_XYZ is expected to be absent from the environment;
	// the uniquely-named variable requires no explicit unset.
	cfg := &Config{
		Notify: []NotifyProviderConfig{
			{Provider: "any", Events: []string{"run_complete"}, Settings: map[string]string{"bot_token": "${MISSING_NOTIFY_VAR_XYZ}"}},
		},
	}
	expandNotifySettings(cfg)
	got := cfg.Notify[0].Settings["bot_token"]
	if got != "" {
		t.Errorf("Settings[bot_token] = %q, want empty string for missing env var", got)
	}
}

// --- BaseBranch tests ---

func TestBaseBranchDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseBranch != "" {
		t.Errorf("BaseBranch = %q, want empty string", cfg.BaseBranch)
	}
}

func TestBaseBranchFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
base_branch: my-feature
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseBranch != "my-feature" {
		t.Errorf("BaseBranch = %q, want %q", cfg.BaseBranch, "my-feature")
	}
}

func TestBaseBranchFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
base_branch: yaml-branch
`)

	v := "flag-branch"
	cfg, err := Load(path, CLIFlags{BaseBranch: &v})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseBranch != "flag-branch" {
		t.Errorf("BaseBranch = %q, want %q (flag should override)", cfg.BaseBranch, "flag-branch")
	}
}

func TestBaseBranchFlagNotSetPreservesYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
base_branch: yaml-branch
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseBranch != "yaml-branch" {
		t.Errorf("BaseBranch = %q, want %q (YAML value should be preserved)", cfg.BaseBranch, "yaml-branch")
	}
}

// --- DefaultBranch tests ---

func TestEffectiveDefaultBranch_ExplicitValue(t *testing.T) {
	cfg := &Config{DefaultBranch: "develop"}
	if got := cfg.EffectiveDefaultBranch("owner/repo"); got != "develop" {
		t.Errorf("EffectiveDefaultBranch() = %q, want %q", got, "develop")
	}
}

func TestEffectiveDefaultBranch_APILookup(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("master\n"), nil
	}

	cfg := &Config{}
	if got := cfg.EffectiveDefaultBranch("owner/repo"); got != "master" {
		t.Errorf("EffectiveDefaultBranch() = %q, want %q", got, "master")
	}
}

func TestEffectiveDefaultBranch_FallbackOnAPIError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh not found")
	}

	cfg := &Config{}
	if got := cfg.EffectiveDefaultBranch("owner/repo"); got != "main" {
		t.Errorf("EffectiveDefaultBranch() = %q, want %q", got, "main")
	}
}

func TestDefaultBranchFlagOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
default_branch: yaml-default
`)

	v := "flag-default"
	cfg, err := Load(path, CLIFlags{DefaultBranch: &v})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultBranch != "flag-default" {
		t.Errorf("DefaultBranch = %q, want %q (flag should override)", cfg.DefaultBranch, "flag-default")
	}
}

func TestDefaultBranchFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
default_branch: master
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultBranch != "master" {
		t.Errorf("DefaultBranch = %q, want %q", cfg.DefaultBranch, "master")
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

func TestMaxResumeRetriesDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `repo: owner/repo`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxResumeRetries != 2 {
		t.Errorf("MaxResumeRetries = %d, want 2", cfg.MaxResumeRetries)
	}
}

func TestMaxResumeRetriesOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
max_resume_retries: 0
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxResumeRetries != 0 {
		t.Errorf("MaxResumeRetries = %d, want 0", cfg.MaxResumeRetries)
	}
}

func TestMaxRebaseAttemptsDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxRebaseAttempts != 1 {
		t.Errorf("MaxRebaseAttempts = %d, want 1", cfg.MaxRebaseAttempts)
	}
}

func TestMaxRebaseAttemptsOverride(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
max_rebase_attempts: 3
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxRebaseAttempts != 3 {
		t.Errorf("MaxRebaseAttempts = %d, want 3", cfg.MaxRebaseAttempts)
	}
}

func TestMaxRebaseAttemptsZeroDisables(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
max_rebase_attempts: 0
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxRebaseAttempts != 0 {
		t.Errorf("MaxRebaseAttempts = %d, want 0", cfg.MaxRebaseAttempts)
	}
}

func TestTruncationLimitsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Truncation.VerifyOutput != 4096 {
		t.Errorf("Truncation.VerifyOutput = %d, want 4096", cfg.Truncation.VerifyOutput)
	}
	if cfg.Truncation.PRDiff != 30000 {
		t.Errorf("Truncation.PRDiff = %d, want 30000", cfg.Truncation.PRDiff)
	}
}

func TestTruncationLimitsValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
truncation:
  verify_output: 8192
  pr_diff: 60000
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Truncation.VerifyOutput != 8192 {
		t.Errorf("Truncation.VerifyOutput = %d, want 8192", cfg.Truncation.VerifyOutput)
	}
	if cfg.Truncation.PRDiff != 60000 {
		t.Errorf("Truncation.PRDiff = %d, want 60000", cfg.Truncation.PRDiff)
	}
}

func TestTruncationLimitsZeroVerifyOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
truncation:
  verify_output: 0
  pr_diff: 30000
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for verify_output: 0, got nil")
	}
	if !strings.Contains(err.Error(), "truncation.verify_output") {
		t.Errorf("error = %q, want mention of 'truncation.verify_output'", err.Error())
	}
}

func TestTruncationLimitsNegativePRDiff(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
truncation:
  verify_output: 4096
  pr_diff: -1
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for pr_diff: -1, got nil")
	}
	if !strings.Contains(err.Error(), "truncation.pr_diff") {
		t.Errorf("error = %q, want mention of 'truncation.pr_diff'", err.Error())
	}
}

// --- ResolveBranch tests ---

func TestResolveBranch_ExplicitBaseBranch(t *testing.T) {
	cfg := &Config{BaseBranch: "feature/custom"}
	got := cfg.ResolveBranch("Phase 23: Watch", []int{42, 43})
	if got != "feature/custom" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "feature/custom")
	}
}

func TestResolveBranch_MilestonePhase(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("Phase 23: Watch & Daemon Mode", nil)
	if got != "godark/phase-23" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "godark/phase-23")
	}
}

func TestResolveBranch_MilestonePhaseNoSubtitle(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("Phase 5", nil)
	if got != "godark/phase-5" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "godark/phase-5")
	}
}

func TestResolveBranch_MilestoneNonPhase(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("Q1 Hardening", nil)
	if got != "godark/q1-hardening" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "godark/q1-hardening")
	}
}

func TestResolveBranch_SingleIssue(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("", []int{42})
	if got != "godark/issue-42" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "godark/issue-42")
	}
}

func TestResolveBranch_MultipleIssues(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("", []int{42, 43})
	if got != "godark/issues-42-43" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "godark/issues-42-43")
	}
}

func TestResolveBranch_MultipleIssuesThree(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("", []int{10, 20, 30})
	if got != "godark/issues-10-20-30" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "godark/issues-10-20-30")
	}
}

func TestResolveBranch_EmptyReturnsEmpty(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("", nil)
	if got != "" {
		t.Errorf("ResolveBranch() = %q, want empty string", got)
	}
}

func TestResolveBranch_MilestoneTakesPriorityOverIssueNums(t *testing.T) {
	cfg := &Config{}
	got := cfg.ResolveBranch("Phase 7: Infra", []int{42})
	if got != "godark/phase-7" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "godark/phase-7")
	}
}

func TestResolveBranch_ExplicitMainAllowsOptOut(t *testing.T) {
	// Setting base_branch: main signals "merge directly to default branch, no rollup".
	cfg := &Config{BaseBranch: "main"}
	got := cfg.ResolveBranch("Phase 23: Watch", nil)
	if got != "main" {
		t.Errorf("ResolveBranch() = %q, want %q", got, "main")
	}
}

func TestDefaultRollupIsManual(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `repo: owner/repo`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge.Rollup != RollupManual {
		t.Errorf("AutoMerge.Rollup = %q, want %q", cfg.AutoMerge.Rollup, RollupManual)
	}
}

func TestRollupNoneCanBeExplicitlySet(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
auto_merge:
  rollup: none
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoMerge.Rollup != RollupNone {
		t.Errorf("AutoMerge.Rollup = %q, want %q", cfg.AutoMerge.Rollup, RollupNone)
	}
}

// TestDockerComposeValidConfig verifies that a docker_compose block with a
// file field is parsed and the struct is populated.
func TestDockerComposeValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DockerCompose == nil {
		t.Fatal("DockerCompose is nil, want non-nil")
	}
	if cfg.DockerCompose.File != "docker-compose.test.yml" {
		t.Errorf("DockerCompose.File = %q, want %q", cfg.DockerCompose.File, "docker-compose.test.yml")
	}
}

// TestDockerComposeMissingFile verifies that validation rejects a docker_compose
// block with no file field.
func TestDockerComposeMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  project_name: myproject
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for missing docker_compose.file, got nil")
	}
	if !strings.Contains(err.Error(), "docker_compose.file") {
		t.Errorf("error %q does not mention docker_compose.file", err.Error())
	}
}

// TestDockerComposeUnsafeProjectName verifies that validation rejects an
// unsafe project_name value.
func TestDockerComposeUnsafeProjectName(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
  project_name: "../bad"
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for unsafe docker_compose.project_name, got nil")
	}
	if !strings.Contains(err.Error(), "docker_compose.project_name") {
		t.Errorf("error %q does not mention docker_compose.project_name", err.Error())
	}
}

// TestDockerComposeDotDotProjectName verifies that validation rejects ".." as
// a project_name value, which passes the regex but is an unsafe path component.
func TestDockerComposeDotDotProjectName(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
  project_name: ".."
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for project_name \"..\" , got nil")
	}
	if !strings.Contains(err.Error(), "docker_compose.project_name") {
		t.Errorf("error %q does not mention docker_compose.project_name", err.Error())
	}
}

// TestDockerComposeAbsentBlock verifies that a config without a docker_compose
// block results in a nil DockerCompose field (feature disabled).
func TestDockerComposeAbsentBlock(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DockerCompose != nil {
		t.Errorf("DockerCompose = %+v, want nil", cfg.DockerCompose)
	}
}

// TestDockerComposeValidProjectName verifies that a safe project_name value
// passes validation without error.
func TestDockerComposeValidProjectName(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
  project_name: my-tests
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DockerCompose == nil {
		t.Fatal("DockerCompose is nil, want non-nil")
	}
	if cfg.DockerCompose.ProjectName != "my-tests" {
		t.Errorf("DockerCompose.ProjectName = %q, want %q", cfg.DockerCompose.ProjectName, "my-tests")
	}
}

// TestDockerComposeAbsoluteFilePath verifies that validation rejects an absolute
// path for docker_compose.file.
func TestDockerComposeAbsoluteFilePath(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: /etc/passwd
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for absolute docker_compose.file, got nil")
	}
	if !strings.Contains(err.Error(), "docker_compose.file") {
		t.Errorf("error %q does not mention docker_compose.file", err.Error())
	}
}

// --- ResolveProjectName tests ---

func TestResolveProjectName_NoPrefix(t *testing.T) {
	got := ResolveProjectName("", 42)
	if got != "godark-42" {
		t.Errorf("ResolveProjectName(%q, 42) = %q, want %q", "", got, "godark-42")
	}
}

func TestResolveProjectName_WithPrefix(t *testing.T) {
	got := ResolveProjectName("myapp", 42)
	if got != "myapp-42" {
		t.Errorf("ResolveProjectName(%q, 42) = %q, want %q", "myapp", got, "myapp-42")
	}
}

func TestResolveProjectName_PrefixWithSpaces(t *testing.T) {
	got := ResolveProjectName("My App", 42)
	// "My App" → lowercase "my app" → "my-app" → "my-app-42"
	if got != "my-app-42" {
		t.Errorf("ResolveProjectName(%q, 42) = %q, want %q", "My App", got, "my-app-42")
	}
}

func TestResolveProjectName_NameValidity(t *testing.T) {
	tests := []struct {
		prefix string
		issue  int
	}{
		{"", 42},
		{"myapp", 99},
		{"My App", 1},
		{"my_project", 543},
	}
	validChars := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	for _, tc := range tests {
		got := ResolveProjectName(tc.prefix, tc.issue)
		if !validChars.MatchString(got) {
			t.Errorf("ResolveProjectName(%q, %d) = %q: contains invalid characters", tc.prefix, tc.issue, got)
		}
	}
}

func TestResolveProjectName_DistinctPerIssue(t *testing.T) {
	prefix := "myapp"
	names := make(map[string]bool)
	for _, issue := range []int{1, 2, 42, 100, 543} {
		name := ResolveProjectName(prefix, issue)
		if names[name] {
			t.Errorf("ResolveProjectName(%q, %d) produced duplicate name %q", prefix, issue, name)
		}
		names[name] = true
	}
}

func TestResolveProjectName_EmptyPrefixDistinctPerIssue(t *testing.T) {
	names := make(map[string]bool)
	for _, issue := range []int{1, 2, 42, 100, 543} {
		name := ResolveProjectName("", issue)
		if names[name] {
			t.Errorf("ResolveProjectName(%q, %d) produced duplicate name %q", "", issue, name)
		}
		names[name] = true
	}
}

// TestDockerComposeTraversalFilePath verifies that validation rejects a file path
// containing ".." path traversal components.
func TestDockerComposeTraversalFilePath(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: "../../etc/shadow"
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for path traversal in docker_compose.file, got nil")
	}
	if !strings.Contains(err.Error(), "docker_compose.file") {
		t.Errorf("error %q does not mention docker_compose.file", err.Error())
	}
}

// TestDockerComposeServicesParsesTwoServices verifies that a docker_compose
// block with two services is parsed and both entries are populated.
func TestDockerComposeServicesParsesTwoServices(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
  services:
    - name: postgres
      description: "PostgreSQL on localhost:5432"
    - name: redis
      description: "Redis on localhost:6379"
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DockerCompose == nil {
		t.Fatal("DockerCompose is nil, want non-nil")
	}
	if len(cfg.DockerCompose.Services) != 2 {
		t.Fatalf("len(Services) = %d, want 2", len(cfg.DockerCompose.Services))
	}
	if cfg.DockerCompose.Services[0].Name != "postgres" {
		t.Errorf("Services[0].Name = %q, want %q", cfg.DockerCompose.Services[0].Name, "postgres")
	}
	if cfg.DockerCompose.Services[0].Description != "PostgreSQL on localhost:5432" {
		t.Errorf("Services[0].Description = %q, want %q", cfg.DockerCompose.Services[0].Description, "PostgreSQL on localhost:5432")
	}
	if cfg.DockerCompose.Services[1].Name != "redis" {
		t.Errorf("Services[1].Name = %q, want %q", cfg.DockerCompose.Services[1].Name, "redis")
	}
	if cfg.DockerCompose.Services[1].Description != "Redis on localhost:6379" {
		t.Errorf("Services[1].Description = %q, want %q", cfg.DockerCompose.Services[1].Description, "Redis on localhost:6379")
	}
}

// TestDockerComposeServicesEmptyNameRejected verifies that validation rejects
// a service entry with no name field.
func TestDockerComposeServicesEmptyNameRejected(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
  services:
    - description: "no name provided"
`)

	_, err := Load(path, CLIFlags{})
	if err == nil {
		t.Fatal("expected error for empty service name, got nil")
	}
	if !strings.Contains(err.Error(), "docker_compose.services[0]") {
		t.Errorf("error %q does not mention docker_compose.services[0]", err.Error())
	}
}

// TestDockerComposeServicesDescriptionOptional verifies that a service entry
// with a name but no description passes validation.
func TestDockerComposeServicesDescriptionOptional(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
  services:
    - name: postgres
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.DockerCompose.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(cfg.DockerCompose.Services))
	}
	if cfg.DockerCompose.Services[0].Name != "postgres" {
		t.Errorf("Services[0].Name = %q, want %q", cfg.DockerCompose.Services[0].Name, "postgres")
	}
	if cfg.DockerCompose.Services[0].Description != "" {
		t.Errorf("Services[0].Description = %q, want empty", cfg.DockerCompose.Services[0].Description)
	}
}

// TestDockerComposeServicesAbsent verifies that a docker_compose block without
// a services array results in a nil/empty Services slice.
func TestDockerComposeServicesAbsent(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, `
repo: owner/repo
docker_compose:
  file: docker-compose.test.yml
`)

	cfg, err := Load(path, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DockerCompose == nil {
		t.Fatal("DockerCompose is nil, want non-nil")
	}
	if len(cfg.DockerCompose.Services) != 0 {
		t.Errorf("len(Services) = %d, want 0 when services absent", len(cfg.DockerCompose.Services))
	}
}
