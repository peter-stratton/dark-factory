package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// validNotifyProviders lists the recognized notify provider names.
// A provider name and its constructor in the notify package are added
// atomically — config validation and runtime dispatch stay in sync.
var validNotifyProviders = map[string]bool{
	"telegram": true,
}

// validNotifyEvents lists the recognized notify event names.
var validNotifyEvents = map[string]bool{
	"run_complete":            true,
	"implementation_complete": true,
	"abort":                   true,
}

// NotifyProviderConfig holds configuration for a single notification provider.
type NotifyProviderConfig struct {
	Provider string            `yaml:"provider"`           // "telegram", future: "slack", etc.
	Events   []string          `yaml:"events"`             // "run_complete", "implementation_complete", "abort"
	Settings map[string]string `yaml:"settings,omitempty"` // provider-specific key-value pairs
}

// safeModuleName matches module names containing only alphanumerics, hyphens,
// underscores, and dots — safe to embed in shell commands via "cd <name> && …".
var safeModuleName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validateModuleName returns an error if name is not a safe, unambiguous
// filesystem path component. Rejected: empty, the special directory "..", and
// any name containing characters outside [a-zA-Z0-9._-].
func validateModuleName(name string) error {
	if name == "" {
		return fmt.Errorf("module name must not be empty")
	}
	if name == ".." {
		return fmt.Errorf("module name %q is not a safe path component", name)
	}
	if !safeModuleName.MatchString(name) {
		return fmt.Errorf("module name %q contains unsafe characters", name)
	}
	return nil
}

// Runtime identifies the project's toolchain and optional version.
type Runtime struct {
	Name    string `yaml:"name"`    // go, flutter, node, rust, python
	Version string `yaml:"version"` // optional — auto-detected if empty
}

// Quality holds quality-gate thresholds for review steps.
// A threshold of 0 disables that check.
type Quality struct {
	MinReviewCostUSD         float64 `yaml:"min_review_cost_usd"`
	MinReviewDurationSeconds int     `yaml:"min_review_duration_seconds"`
}

// Verify holds configuration for the verify step.
type Verify struct {
	MaxFixAttempts int  `yaml:"max_fix_attempts"`
	Blocking       bool `yaml:"blocking"`
}

// WaitForChecks holds CI check gating configuration. When non-nil, godark
// polls GitHub status checks after the review cycle and only merges once all
// required checks succeed.
type WaitForChecks struct {
	Timeout  string   `yaml:"timeout"`  // duration string, e.g. "10m"
	Required []string `yaml:"required"` // check names to wait for
}

// Module holds per-module build/test/lint/generate commands and dependency
// relationships. All fields are optional.
type Module struct {
	FormatCommand   string   `yaml:"format_command"`
	BuildCommand    string   `yaml:"build_command"`
	TestCommand     string   `yaml:"test_command"`
	LintCommand     string   `yaml:"lint_command"`
	GenerateCommand string   `yaml:"generate_command"`
	DependsOn       []string `yaml:"depends_on"`
}

// Watch holds configuration for the godark watch polling settings.
// Nil means the watch command uses its hardcoded default interval ("60s").
type Watch struct {
	PollInterval string `yaml:"poll_interval"`
}

// RiskThresholds holds the thresholds used by the low_risk auto-merge
// classifier. Nil means the classifier uses its built-in defaults
// (MaxLines: 200, MaxFiles: 10).
type RiskThresholds struct {
	MaxLines int `yaml:"max_lines"`
	MaxFiles int `yaml:"max_files"`
}

// TruncationLimits holds the maximum byte counts used when truncating
// agent outputs before embedding them in prompts. Defaults are applied
// in defaults() so usage sites never need to check for zero values.
type TruncationLimits struct {
	// VerifyOutput is the maximum number of bytes kept from combined
	// stdout+stderr of a verify command. Default: 4096.
	VerifyOutput int `yaml:"verify_output"`
	// PRDiff is the maximum number of bytes included from a PR diff.
	// Default: 30000.
	PRDiff int `yaml:"pr_diff"`
}

// FeatureMergeStrategy controls whether and how the feature branch is merged
// into the base branch after a successful review cycle.
type FeatureMergeStrategy string

const (
	// MergeNone disables automatic feature-branch merging.
	MergeNone FeatureMergeStrategy = "none"
	// MergeLowRisk merges automatically only when the risk classifier deems
	// the PR low-risk.
	MergeLowRisk FeatureMergeStrategy = "low_risk"
	// MergeAll merges automatically regardless of risk.
	MergeAll FeatureMergeStrategy = "all"
)

// Valid reports whether s is a recognized FeatureMergeStrategy value.
func (s FeatureMergeStrategy) Valid() bool {
	switch s {
	case MergeNone, MergeLowRisk, MergeAll:
		return true
	}
	return false
}

// RollupMergeStrategy controls what happens to the base branch after a run
// completes (base branch → default branch rollup PR).
type RollupMergeStrategy string

const (
	// RollupNone disables rollup PR creation.
	RollupNone RollupMergeStrategy = "none"
	// RollupManual creates a rollup PR but leaves it open for human review.
	RollupManual RollupMergeStrategy = "manual"
	// RollupAuto creates and immediately merges the rollup PR.
	RollupAuto RollupMergeStrategy = "auto"
)

// Valid reports whether s is a recognized RollupMergeStrategy value.
func (s RollupMergeStrategy) Valid() bool {
	switch s {
	case RollupNone, RollupManual, RollupAuto:
		return true
	}
	return false
}

// AutoMerge holds independent merge-strategy controls for the two merge
// points introduced by configurable base branches.
type AutoMerge struct {
	// Feature controls merging of the feature branch into the base branch.
	Feature FeatureMergeStrategy `yaml:"feature"`
	// Rollup controls what happens to the base branch → default branch after
	// a run completes.
	Rollup RollupMergeStrategy `yaml:"rollup"`
}

// Config holds all configuration for a godark run.
type Config struct {
	Repo       string `yaml:"repo"`
	MaxRetries int    `yaml:"max_retries"`
	// MaxResumeRetries controls when retry agents switch from resuming their
	// prior session to starting a fresh session with structured handoff context.
	// On attempt N where N >= MaxResumeRetries, a fresh session is started.
	// A value of 0 means all retries use fresh mode. A value >= MaxRetries
	// means all retries use session resumption. Default: 2.
	MaxResumeRetries int `yaml:"max_resume_retries"`
	// MaxRebaseAttempts controls how many automatic rebase/conflict-fix cycles
	// are attempted before a conflicting PR is labeled needs-human-review.
	// Each cycle tries gh pr update-branch first; if that fails it invokes the
	// implementer to resolve conflicts. A value of 0 disables the pre-merge
	// rebase check entirely. Default: 1.
	MaxRebaseAttempts int `yaml:"max_rebase_attempts"`

	AgentTimeout  string            `yaml:"agent_timeout"`
	FormatCommand string            `yaml:"format_command"`
	BuildCommand  string            `yaml:"build_command"`
	TestCommand   string            `yaml:"test_command"`
	LintCommand   string            `yaml:"lint_command"`
	SandboxEnv    map[string]string `yaml:"sandbox_env"`
	Runtime       Runtime           `yaml:"runtime"`

	ProtectedPaths []string `yaml:"protected_paths"`
	DeniedCommands []string `yaml:"denied_commands"`
	RoadmapPath    string   `yaml:"roadmap_path"`

	GenerateCommand string   `yaml:"generate_command"`
	GeneratedPaths  []string `yaml:"generated_paths"`
	PlanningDir     string   `yaml:"planning_dir"`
	ScenarioDir     string   `yaml:"scenario_dir"`
	ReviewDir       string   `yaml:"review_dir"`

	ArchitectureDoc  string `yaml:"architecture_doc"`
	ArchitectureJSON string `yaml:"architecture_json"`
	ConventionsDoc   string `yaml:"conventions_doc"`

	NoSandbox              bool      `yaml:"no_sandbox"`
	AutoMerge              AutoMerge `yaml:"auto_merge"`
	BaseBranch             string    `yaml:"base_branch"`
	DefaultBranch          string    `yaml:"default_branch"`
	QualityStrictnessDecay bool   `yaml:"quality_strictness_decay"`
	EnforceArchitecture    bool   `yaml:"enforce_architecture"`

	// AuthPreference controls which Anthropic auth token is preferred when both
	// ANTHROPIC_API_KEY and CLAUDE_CODE_OAUTH_TOKEN are set.
	// Valid values: "oauth" (default) or "api_key".
	AuthPreference string `yaml:"auth_preference"`

	Docker      Docker           `yaml:"docker"`
	Prompts     Prompts          `yaml:"prompts"`
	Quality     Quality          `yaml:"quality"`
	Verify      Verify           `yaml:"verify"`
	Truncation  TruncationLimits `yaml:"truncation"`

	// Modules maps module names to per-module build/test/lint/generate commands
	// and dependency relationships. Nil (absent) means single-module mode.
	Modules map[string]Module `yaml:"modules"`

	// RequiredEnv lists environment variable names that must be set before a
	// run starts. Their values are forwarded to the sandbox alongside auth env
	// vars. Nil (absent) means no required env vars.
	RequiredEnv []string `yaml:"required_env"`

	// WaitForChecks configures CI check gating before merge. Nil means merge
	// immediately after the review cycle (current behavior).
	WaitForChecks *WaitForChecks `yaml:"wait_for_checks"`

	// Watch configures polling settings for the watch command. Nil means
	// the watch command uses its hardcoded default poll interval ("60s").
	Watch *Watch `yaml:"watch"`

	// RiskThresholds configures thresholds for the low_risk auto-merge
	// classifier. Nil means the classifier uses its built-in defaults.
	RiskThresholds *RiskThresholds `yaml:"risk_thresholds"`

	// Notify holds zero or more notification provider configurations.
	// An absent or empty list disables all notifications.
	Notify []NotifyProviderConfig `yaml:"notify"`
}

// Docker holds Docker sandbox configuration.
type Docker struct {
	Image           string   `yaml:"image"`
	Dockerfile      string   `yaml:"dockerfile"`
	Mount           string   `yaml:"mount"`
	User            string   `yaml:"user"`
	NodeVersion     string   `yaml:"node_version"`
	ExtraPackages   []string `yaml:"extra_packages"`
	InstallCommands []string `yaml:"install_commands"`
}

// Prompts holds paths to prompt template files.
type Prompts struct {
	Implementer      string `yaml:"implementer"`
	ImplementerRetry string `yaml:"implementer_retry"`
	Reviewer         string `yaml:"reviewer"`
	QualityReviewer  string `yaml:"quality_reviewer"`
	SpecGenerator    string `yaml:"spec_generator"`
	Punchlist        string `yaml:"punchlist"`
	VerifyFix        string `yaml:"verify_fix"`
	Recon            string `yaml:"recon"`
}

// CLIFlags holds flag values passed on the command line.
// Pointer fields distinguish "not set" (nil) from zero values.
type CLIFlags struct {
	Repo              *string
	MaxRetries        *int
	NoSandbox         *bool
	AutoMergeFeature  *string
	AutoMergeRollup   *string
	BaseBranch        *string
	DefaultBranch     *string
	Config            string
}

// EffectiveBaseBranch returns BaseBranch, defaulting to "main" when empty.
func (c *Config) EffectiveBaseBranch() string {
	if c.BaseBranch == "" {
		return "main"
	}
	return c.BaseBranch
}

// EffectiveDefaultBranch returns the default branch for the repository.
// Resolution order: explicit config/CLI value, GitHub API lookup, fallback "main".
func (c *Config) EffectiveDefaultBranch(repo string) string {
	if c.DefaultBranch != "" {
		return c.DefaultBranch
	}
	out, err := CommandRunner("gh", "repo", "view", repo, "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name")
	if err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	return "main"
}

// Load reads a YAML config file at path and merges CLI flag overrides.
// A missing config file is not an error if all required values come from flags.
func Load(path string, flags CLIFlags) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		// Missing file is OK — flags may supply required values.
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	expandNotifySettings(cfg)
	applyFlags(cfg, flags)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		MaxRetries:        3,
		MaxResumeRetries:  2,
		MaxRebaseAttempts: 1,
		AutoMerge:      AutoMerge{Feature: MergeNone, Rollup: RollupNone},
		RoadmapPath:    "docs/ROADMAP.md",
		ProtectedPaths: []string{"godark.yaml"},
		DeniedCommands: []string{
			"rm -rf",
			"git push --force",
			"git push -f",
			"git reset --hard",
			"git clean -f",
		},
		PlanningDir:            "docs/planning/",
		ScenarioDir:            "tests/scenarios/",
		ReviewDir:              "tests/review/",
		ArchitectureDoc:        "docs/architecture.md",
		ArchitectureJSON:       "docs/architecture.json",
		ConventionsDoc:         "docs/conventions.md",
		QualityStrictnessDecay: true,
		AuthPreference:         "oauth",
		Verify: Verify{
			MaxFixAttempts: 2,
			Blocking:       true,
		},
		Truncation: TruncationLimits{
			VerifyOutput: 4096,
			PRDiff:       30000,
		},
	}
}

func applyFlags(cfg *Config, flags CLIFlags) {
	if flags.Repo != nil {
		cfg.Repo = *flags.Repo
	}
	if flags.MaxRetries != nil {
		cfg.MaxRetries = *flags.MaxRetries
	}
	if flags.NoSandbox != nil {
		cfg.NoSandbox = *flags.NoSandbox
	}
	if flags.AutoMergeFeature != nil {
		cfg.AutoMerge.Feature = FeatureMergeStrategy(*flags.AutoMergeFeature)
	}
	if flags.AutoMergeRollup != nil {
		cfg.AutoMerge.Rollup = RollupMergeStrategy(*flags.AutoMergeRollup)
	}
	if flags.BaseBranch != nil {
		cfg.BaseBranch = *flags.BaseBranch
	}
	if flags.DefaultBranch != nil {
		cfg.DefaultBranch = *flags.DefaultBranch
	}
}

func validate(cfg *Config) error {
	if cfg.Repo == "" {
		return fmt.Errorf("repo is required (set in config file or pass --repo)")
	}
	switch cfg.AuthPreference {
	case "oauth", "api_key":
		// valid
	default:
		return fmt.Errorf("auth_preference must be \"oauth\" or \"api_key\", got %q", cfg.AuthPreference)
	}
	if !cfg.AutoMerge.Feature.Valid() {
		return fmt.Errorf("auto_merge.feature must be \"none\", \"low_risk\", or \"all\", got %q", cfg.AutoMerge.Feature)
	}
	if !cfg.AutoMerge.Rollup.Valid() {
		return fmt.Errorf("auto_merge.rollup must be \"none\", \"manual\", or \"auto\", got %q", cfg.AutoMerge.Rollup)
	}
	if err := validateModules(cfg.Modules); err != nil {
		return err
	}
	if err := validateWaitForChecks(cfg.WaitForChecks); err != nil {
		return err
	}
	if err := validateWatch(cfg.Watch); err != nil {
		return err
	}
	if err := validateRiskThresholds(cfg.RiskThresholds); err != nil {
		return err
	}
	if err := validateTruncationLimits(cfg.Truncation); err != nil {
		return err
	}
	if err := validateNotify(cfg.Notify); err != nil {
		return err
	}
	return nil
}

// validateWaitForChecks ensures WaitForChecks fields are valid when set.
func validateWaitForChecks(w *WaitForChecks) error {
	if w == nil {
		return nil
	}
	d, err := time.ParseDuration(w.Timeout)
	if err != nil {
		return fmt.Errorf("wait_for_checks.timeout %q is not a valid duration: %w", w.Timeout, err)
	}
	if d <= 0 {
		return fmt.Errorf("wait_for_checks.timeout must be a positive duration, got %q", w.Timeout)
	}
	if len(w.Required) == 0 {
		return fmt.Errorf("wait_for_checks.required must be non-empty when wait_for_checks is set")
	}
	return nil
}

// validateWatch ensures Watch fields are valid when set.
func validateWatch(w *Watch) error {
	if w == nil {
		return nil
	}
	if w.PollInterval != "" {
		d, err := time.ParseDuration(w.PollInterval)
		if err != nil {
			return fmt.Errorf("watch.poll_interval %q is not a valid duration: %w", w.PollInterval, err)
		}
		if d <= 0 {
			return fmt.Errorf("watch.poll_interval must be a positive duration, got %q", w.PollInterval)
		}
	}
	return nil
}

// validateRiskThresholds ensures RiskThresholds fields are positive when set.
func validateRiskThresholds(r *RiskThresholds) error {
	if r == nil {
		return nil
	}
	if r.MaxLines <= 0 {
		return fmt.Errorf("risk_thresholds.max_lines must be a positive integer, got %d", r.MaxLines)
	}
	if r.MaxFiles <= 0 {
		return fmt.Errorf("risk_thresholds.max_files must be a positive integer, got %d", r.MaxFiles)
	}
	return nil
}

// validateModules checks that module names are safe, that all depends_on
// references name existing modules (with no duplicates), and that there are no
// dependency cycles.
func validateModules(modules map[string]Module) error {
	if len(modules) == 0 {
		return nil
	}

	// Validate every module name is a safe filesystem path component.
	for name := range modules {
		if err := validateModuleName(name); err != nil {
			return err
		}
	}

	// Check all depends_on entries reference known modules and are unique.
	for name, mod := range modules {
		seen := make(map[string]bool, len(mod.DependsOn))
		for _, dep := range mod.DependsOn {
			if _, ok := modules[dep]; !ok {
				return fmt.Errorf("module %q depends_on unknown module %q", name, dep)
			}
			if seen[dep] {
				return fmt.Errorf("module %q depends_on %q more than once", name, dep)
			}
			seen[dep] = true
		}
	}

	// Detect cycles using DFS with three-color marking:
	//   0 = white (unvisited), 1 = gray (in current path), 2 = black (done)
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(modules))

	var visit func(name string) error
	visit = func(name string) error {
		if color[name] == gray {
			return fmt.Errorf("cycle detected in module dependencies involving %q", name)
		}
		if color[name] == black {
			return nil
		}
		color[name] = gray
		for _, dep := range modules[name].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		color[name] = black
		return nil
	}

	for name := range modules {
		if color[name] == white {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

// expandNotifySettings expands ${VAR} references in each provider's Settings
// map using the current process environment. Missing variables resolve to "".
// Expansion happens in-place after YAML parsing so that downstream code
// (including the notify package) always receives literal values.
func expandNotifySettings(cfg *Config) {
	for i := range cfg.Notify {
		s := cfg.Notify[i].Settings
		if len(s) == 0 {
			continue
		}
		expanded := make(map[string]string, len(s))
		for k, v := range s {
			expanded[k] = os.Expand(v, os.Getenv)
		}
		cfg.Notify[i].Settings = expanded
	}
}

// validateTruncationLimits ensures TruncationLimits fields are positive.
func validateTruncationLimits(t TruncationLimits) error {
	if t.VerifyOutput <= 0 {
		return fmt.Errorf("truncation.verify_output must be a positive integer, got %d", t.VerifyOutput)
	}
	if t.PRDiff <= 0 {
		return fmt.Errorf("truncation.pr_diff must be a positive integer, got %d", t.PRDiff)
	}
	return nil
}

// validateNotify checks that every provider name and event name in the notify
// list is recognized. Provider-specific settings are validated by the
// provider's own constructor, not here.
func validateNotify(notify []NotifyProviderConfig) error {
	for i, n := range notify {
		if len(n.Events) == 0 {
			return fmt.Errorf("notify[%d]: events list must not be empty", i)
		}
		for _, event := range n.Events {
			if !validNotifyEvents[event] {
				return fmt.Errorf("notify[%d]: unknown event %q", i, event)
			}
		}
		if !validNotifyProviders[n.Provider] {
			return fmt.Errorf("notify[%d]: unknown provider %q", i, n.Provider)
		}
	}
	return nil
}
