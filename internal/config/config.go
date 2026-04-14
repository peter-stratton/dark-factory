package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/peter-stratton/dark-factory/internal/label"

	darkexec "github.com/peter-stratton/dark-factory/internal/exec"
	"gopkg.in/yaml.v3"
)

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner darkexec.CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
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
	Timeout      string   `yaml:"timeout"`       // duration string, e.g. "10m"
	Required     []string `yaml:"required"`      // check names to wait for
	StartupGrace string   `yaml:"startup_grace"` // duration to wait for checks to appear before treating as N/A (default: "60s")
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

// Judge holds configuration for the container health judge.
// Absent fields get defaults applied by JudgeConfig().
type Judge struct {
	Enabled                   *bool          `yaml:"enabled"`
	IdleTimeoutByRole         map[string]int `yaml:"idle_timeout_by_role"`
	DefaultIdleTimeout        int            `yaml:"default_idle_timeout"`
	NoProgressTimeoutByRole   map[string]int `yaml:"no_progress_timeout_by_role"`
	DefaultNoProgressTimeout  int            `yaml:"default_no_progress_timeout"`
	ToolThrashThreshold       int            `yaml:"tool_thrash_threshold"`
	ToolThrashWindowSecs      int            `yaml:"tool_thrash_window_secs"`
	TransportFailureThreshold int            `yaml:"transport_failure_threshold"`
	ContainerRetryLimit       int            `yaml:"container_retry_limit"`
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

	Model          string            `yaml:"model"`
	ModelOverrides map[string]string `yaml:"model_overrides"`
	AgentTimeout   string            `yaml:"agent_timeout"`
	FormatCommand  string            `yaml:"format_command"`
	BuildCommand   string            `yaml:"build_command"`
	TestCommand    string            `yaml:"test_command"`
	LintCommand    string            `yaml:"lint_command"`
	SandboxEnv     map[string]string `yaml:"sandbox_env"`
	Runtime        Runtime           `yaml:"runtime"`

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

	AutoMerge              AutoMerge `yaml:"auto_merge"`
	BaseBranch             string    `yaml:"base_branch"`
	RollupTitle            string    `yaml:"rollup_title"`
	DefaultBranch          string    `yaml:"default_branch"`
	BranchPrefix           string    `yaml:"branch_prefix"`
	LabelPrefix            string    `yaml:"label_prefix"`
	QualityStrictnessDecay bool      `yaml:"quality_strictness_decay"`
	EnforceArchitecture    bool      `yaml:"enforce_architecture"`

	// AuthPreference controls which Anthropic auth token is preferred when both
	// ANTHROPIC_API_KEY and CLAUDE_CODE_OAUTH_TOKEN are set.
	// Valid values: "oauth" (default) or "api_key".
	AuthPreference string `yaml:"auth_preference"`

	Docker      Docker           `yaml:"docker"`
	Prompts     Prompts          `yaml:"prompts"`
	Review      Review           `yaml:"review"`
	Quality     Quality          `yaml:"quality"`
	Verify      Verify           `yaml:"verify"`
	Truncation  TruncationLimits `yaml:"truncation"`
	Judge       Judge            `yaml:"judge"`
	Concurrency Concurrency      `yaml:"concurrency"`

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

	// DockerCompose configures Docker Compose integration for test environments.
	// Nil means the feature is disabled (opt-in).
	DockerCompose *DockerCompose `yaml:"docker_compose"`

	// HostServices declares services running on the host that must be
	// reachable before processing begins. The orchestrator verifies health
	// checks but does not manage service lifecycle. Nil means no host services.
	HostServices []HostService `yaml:"host_services"`
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

// ComposeService describes a single Docker Compose service for injection
// into agent prompts.
type ComposeService struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// DockerCompose holds configuration for Docker Compose integration.
// When non-nil, godark uses the specified compose file to manage test
// environment containers alongside the sandbox.
type DockerCompose struct {
	// File is the path to the Docker Compose file (e.g. "docker-compose.test.yml").
	// Required when the docker_compose block is present.
	File string `yaml:"file"`
	// ProjectName is an optional prefix for the compose project name.
	// When empty, Docker Compose auto-generates a project name.
	ProjectName string `yaml:"project_name"`
	// Services describes the compose services available in the test environment.
	// Optional — when absent, agents receive no compose service context.
	Services []ComposeService `yaml:"services"`
}

// HostServiceHealthCheck configures health checking for a host service.
type HostServiceHealthCheck struct {
	// Command is the shell command to run (via "sh -c"). Required.
	Command string `yaml:"command"`
	// Timeout is the maximum duration for each health check attempt.
	// Default: "5s".
	Timeout string `yaml:"timeout"`
	// Retries is the number of attempts before declaring the service
	// unreachable. Default: 3.
	Retries int `yaml:"retries"`
}

// HostService describes a service running on the host machine that agents
// need to know about. The orchestrator verifies reachability before
// processing issues and injects service descriptions into agent prompts.
type HostService struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	HealthCheck *HostServiceHealthCheck `yaml:"health_check"`
}

// Prompts holds paths to prompt template files.
type Prompts struct {
	Implementer        string `yaml:"implementer"`
	ImplementerRetry   string `yaml:"implementer_retry"`
	Reviewer           string `yaml:"reviewer"`
	ReviewerSemiformal string `yaml:"reviewer_semiformal"`
	QualityReviewer    string `yaml:"quality_reviewer"`
	SpecGenerator      string `yaml:"spec_generator"`
	Punchlist          string `yaml:"punchlist"`
	VerifyFix          string `yaml:"verify_fix"`
	Recon              string `yaml:"recon"`
	Planner            string `yaml:"planner"`
	MergeCoordinator   string `yaml:"merge_coordinator"`
}

// Concurrency holds concurrency settings for the run orchestrator.
type Concurrency struct {
	MaxWorkers int `yaml:"max_workers"`
}

// Review holds configuration for the functional review step.
type Review struct {
	Semiformal        bool `yaml:"semiformal"`
	SemiformalOnRetry bool `yaml:"semiformal_on_retry"`
}

// CLIFlags holds flag values passed on the command line.
// Pointer fields distinguish "not set" (nil) from zero values.
type CLIFlags struct {
	Repo             *string
	MaxRetries       *int
	AutoMergeFeature *string
	AutoMergeRollup  *string
	BaseBranch       *string
	RollupTitle      *string
	DefaultBranch    *string
	NoJudge          *bool
	Model            *string
	Workers          *int
	Integration      *bool
	Config           string
}

// phasePattern matches "Phase N" at the start of a milestone title (case-insensitive).
var phasePattern = regexp.MustCompile(`(?i)^phase\s+(\d+)`)

// invalidBranchChars matches any character that is not a lowercase letter,
// digit, or hyphen — all are forbidden in git ref names (git check-ref-format).
var invalidBranchChars = regexp.MustCompile(`[^a-z0-9-]+`)

// repeatedHyphens collapses two or more consecutive hyphens into one.
var repeatedHyphens = regexp.MustCompile(`-{2,}`)

// milestoneSlug converts a milestone title to a git-safe slug.
// "Phase 23: Watch & Daemon Mode" → "phase-23"
// Falls back to a lowercase-hyphenated slug when no "Phase N" prefix is found,
// stripping any characters that git check-ref-format would reject.
func milestoneSlug(milestone string) string {
	if m := phasePattern.FindStringSubmatch(milestone); m != nil {
		return "phase-" + m[1]
	}
	// Lower-case and replace common separators with hyphens first.
	s := strings.NewReplacer(" ", "-", "_", "-").Replace(strings.ToLower(milestone))
	// Strip any remaining characters invalid in git ref names.
	s = invalidBranchChars.ReplaceAllString(s, "-")
	// Collapse repeated hyphens and trim leading/trailing hyphens.
	s = repeatedHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// ResolveBranch returns the effective base branch name for a run.
// When BaseBranch is explicitly set (non-empty) it is returned as-is.
// When BaseBranch is empty, auto-generation is attempted: a milestone produces
// a branch like "godark/phase-23", and issueNums produces "godark/issue-42"
// (single issue) or "godark/issues-42-43" (multiple issues).
// To opt out of auto-generation and merge directly to the default branch,
// set base_branch to the repo's default branch name (e.g. "main") explicitly.
// Returns "" when BaseBranch is empty and nothing can be auto-generated.
func (c *Config) ResolveBranch(milestone string, issueNums []int) string {
	if c.BaseBranch != "" {
		return c.BaseBranch
	}
	prefix := c.effectiveBranchPrefix()
	if milestone != "" {
		return prefix + "/" + milestoneSlug(milestone)
	}
	if len(issueNums) == 1 {
		return fmt.Sprintf("%s/issue-%d", prefix, issueNums[0])
	}
	if len(issueNums) > 1 {
		parts := make([]string, len(issueNums))
		for i, n := range issueNums {
			parts[i] = fmt.Sprintf("%d", n)
		}
		return prefix + "/issues-" + strings.Join(parts, "-")
	}
	return ""
}

// ResolveProjectName returns the docker compose project name for a given issue.
// When prefix is empty it returns "godark-<issueNumber>".
// When prefix is non-empty it is sanitized (lowercased, separators replaced with
// hyphens, non-alphanumeric/hyphen characters stripped, repeated hyphens collapsed,
// leading/trailing hyphens removed) and returns "<sanitized-prefix>-<issueNumber>".
// The result is always a valid docker compose -p argument.
func ResolveProjectName(prefix string, issueNumber int) string {
	if prefix == "" {
		return fmt.Sprintf("godark-%d", issueNumber)
	}
	s := strings.NewReplacer(" ", "-", "_", "-").Replace(strings.ToLower(prefix))
	s = invalidBranchChars.ReplaceAllString(s, "-")
	s = repeatedHyphens.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return fmt.Sprintf("godark-%d", issueNumber)
	}
	return fmt.Sprintf("%s-%d", s, issueNumber)
}

// ResolveRollupTitle returns the rollup PR title. When RollupTitle is set, it
// is used as a template with {{.BaseBranch}} and {{.DefaultBranch}} expanded.
// When RollupTitle is empty, the default "chore: merge <base> into <default>"
// format is returned.
func (c *Config) ResolveRollupTitle(baseBranch, defaultBranch string) string {
	if c.RollupTitle == "" {
		return fmt.Sprintf("chore: merge %s into %s", baseBranch, defaultBranch)
	}
	r := strings.NewReplacer(
		"{{.BaseBranch}}", baseBranch,
		"{{.DefaultBranch}}", defaultBranch,
	)
	return r.Replace(c.RollupTitle)
}

// EffectiveBaseBranch returns BaseBranch, defaulting to "main" when empty.
func (c *Config) EffectiveBaseBranch() string {
	if c.BaseBranch == "" {
		return "main"
	}
	return c.BaseBranch
}

// JudgeConfig returns the judge configuration with defaults applied for any
// absent (zero/nil) fields.
func (c *Config) JudgeConfig() Judge {
	j := c.Judge
	if j.Enabled == nil {
		t := true
		j.Enabled = &t
	}
	if j.DefaultIdleTimeout == 0 {
		j.DefaultIdleTimeout = 300
	}
	if j.ToolThrashThreshold == 0 {
		j.ToolThrashThreshold = 3
	}
	if j.ToolThrashWindowSecs == 0 {
		j.ToolThrashWindowSecs = 60
	}
	if j.TransportFailureThreshold == 0 {
		j.TransportFailureThreshold = 10
	}
	if j.ContainerRetryLimit == 0 {
		j.ContainerRetryLimit = 2
	}
	return j
}

func (c *Config) effectiveBranchPrefix() string {
	if c.BranchPrefix == "" {
		return "godark"
	}
	return c.BranchPrefix
}

func (c *Config) effectiveLabelPrefix() string {
	if c.LabelPrefix == "" {
		return "godark"
	}
	return c.LabelPrefix
}

// Labels returns a Labels instance derived from the configured label prefix.
func (c *Config) Labels() *label.Labels {
	return label.New(c.effectiveLabelPrefix())
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
		AutoMerge:         AutoMerge{Feature: MergeNone, Rollup: RollupManual},
		RoadmapPath:       "docs/roadmap/",
		ProtectedPaths:    []string{"godark.yaml"},
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
		Concurrency: Concurrency{MaxWorkers: 1},
	}
}

func applyFlags(cfg *Config, flags CLIFlags) {
	if flags.Repo != nil {
		cfg.Repo = *flags.Repo
	}
	if flags.MaxRetries != nil {
		cfg.MaxRetries = *flags.MaxRetries
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
	if flags.RollupTitle != nil {
		cfg.RollupTitle = *flags.RollupTitle
	}
	if flags.DefaultBranch != nil {
		cfg.DefaultBranch = *flags.DefaultBranch
	}
	if flags.NoJudge != nil && *flags.NoJudge {
		disabled := false
		if cfg.Judge.Enabled == nil || *cfg.Judge.Enabled {
			cfg.Judge.Enabled = &disabled
		}
	}
	if flags.Model != nil {
		cfg.Model = *flags.Model
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
	switch cfg.Model {
	case "", "sonnet", "opus":
		// valid (empty = CLI default, which is sonnet)
	default:
		return fmt.Errorf("model must be \"sonnet\" or \"opus\", got %q", cfg.Model)
	}
	if err := validateModelOverrides(cfg.ModelOverrides); err != nil {
		return err
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
	if err := validateDockerCompose(cfg.DockerCompose); err != nil {
		return err
	}
	if err := validateHostServices(cfg.HostServices); err != nil {
		return err
	}
	if err := validateConcurrency(cfg.Concurrency); err != nil {
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
	if w.StartupGrace != "" {
		gd, err := time.ParseDuration(w.StartupGrace)
		if err != nil {
			return fmt.Errorf("wait_for_checks.startup_grace %q is not a valid duration: %w", w.StartupGrace, err)
		}
		if gd < 0 {
			return fmt.Errorf("wait_for_checks.startup_grace must be non-negative, got %q", w.StartupGrace)
		}
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
func validateModelOverrides(overrides map[string]string) error {
	for role, model := range overrides {
		switch model {
		case "sonnet", "opus":
			// valid
		default:
			return fmt.Errorf("model_overrides.%s must be \"sonnet\" or \"opus\", got %q", role, model)
		}
	}
	return nil
}

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

// validateConcurrency ensures Concurrency fields are valid.
func validateConcurrency(c Concurrency) error {
	if c.MaxWorkers < 1 {
		return fmt.Errorf("concurrency.max_workers must be >= 1, got %d", c.MaxWorkers)
	}
	return nil
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

// validateDockerCompose ensures DockerCompose fields are valid when set.
// file is required and must be a safe relative path; project_name, if set,
// must match the safe name pattern.
func validateDockerCompose(dc *DockerCompose) error {
	if dc == nil {
		return nil
	}
	if dc.File == "" {
		return fmt.Errorf("docker_compose.file must not be empty when docker_compose block is present")
	}
	if strings.HasPrefix(dc.File, "/") {
		return fmt.Errorf("docker_compose.file %q must be a relative path", dc.File)
	}
	for _, component := range strings.Split(dc.File, "/") {
		if component == ".." {
			return fmt.Errorf("docker_compose.file %q must not contain path traversal components", dc.File)
		}
	}
	if dc.ProjectName == ".." {
		return fmt.Errorf("docker_compose.project_name %q is not a safe path component", dc.ProjectName)
	}
	if dc.ProjectName != "" && !safeModuleName.MatchString(dc.ProjectName) {
		return fmt.Errorf("docker_compose.project_name %q contains unsafe characters", dc.ProjectName)
	}
	for i, svc := range dc.Services {
		if svc.Name == "" {
			return fmt.Errorf("docker_compose.services[%d]: name must not be empty", i)
		}
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

// validateHostServices ensures HostService entries are valid when present.
func validateHostServices(services []HostService) error {
	for i, svc := range services {
		if svc.Name == "" {
			return fmt.Errorf("host_services[%d]: name must not be empty", i)
		}
		if svc.HealthCheck != nil {
			if svc.HealthCheck.Command == "" {
				return fmt.Errorf("host_services[%d] (%s): health_check.command must not be empty", i, svc.Name)
			}
			if svc.HealthCheck.Timeout != "" {
				d, err := time.ParseDuration(svc.HealthCheck.Timeout)
				if err != nil {
					return fmt.Errorf("host_services[%d] (%s): health_check.timeout %q is not a valid duration: %w", i, svc.Name, svc.HealthCheck.Timeout, err)
				}
				if d <= 0 {
					return fmt.Errorf("host_services[%d] (%s): health_check.timeout must be positive, got %q", i, svc.Name, svc.HealthCheck.Timeout)
				}
			}
			if svc.HealthCheck.Retries < 0 {
				return fmt.Errorf("host_services[%d] (%s): health_check.retries must not be negative, got %d", i, svc.Name, svc.HealthCheck.Retries)
			}
		}
	}
	return nil
}
