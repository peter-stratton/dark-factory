package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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

// Config holds all configuration for a godark run.
type Config struct {
	Repo       string `yaml:"repo"`
	MaxRetries int    `yaml:"max_retries"`

	AgentTimeout string            `yaml:"agent_timeout"`
	BuildCommand string            `yaml:"build_command"`
	TestCommand  string            `yaml:"test_command"`
	SandboxEnv   map[string]string `yaml:"sandbox_env"`
	Runtime      Runtime           `yaml:"runtime"`

	ProtectedPaths []string `yaml:"protected_paths"`
	RoadmapPath    string   `yaml:"roadmap_path"`
	PlanningDir    string   `yaml:"planning_dir"`
	ScenarioDir    string   `yaml:"scenario_dir"`
	ReviewDir      string   `yaml:"review_dir"`

	ArchitectureDoc  string `yaml:"architecture_doc"`
	ArchitectureJSON string `yaml:"architecture_json"`
	ConventionsDoc   string `yaml:"conventions_doc"`

	NoSandbox              bool `yaml:"no_sandbox"`
	NoMerge                bool `yaml:"no_merge"`
	QualityStrictnessDecay bool `yaml:"quality_strictness_decay"`
	EnforceArchitecture    bool `yaml:"enforce_architecture"`

	// AuthPreference controls which Anthropic auth token is preferred when both
	// ANTHROPIC_API_KEY and CLAUDE_CODE_OAUTH_TOKEN are set.
	// Valid values: "oauth" (default) or "api_key".
	AuthPreference string `yaml:"auth_preference"`

	Docker  Docker  `yaml:"docker"`
	Prompts Prompts `yaml:"prompts"`
	Quality Quality `yaml:"quality"`
}

// Docker holds Docker sandbox configuration.
type Docker struct {
	Image         string   `yaml:"image"`
	Dockerfile    string   `yaml:"dockerfile"`
	Mount         string   `yaml:"mount"`
	User          string   `yaml:"user"`
	NodeVersion   string   `yaml:"node_version"`
	ExtraPackages []string `yaml:"extra_packages"`
}

// Prompts holds paths to prompt template files.
type Prompts struct {
	Implementer      string `yaml:"implementer"`
	ImplementerRetry string `yaml:"implementer_retry"`
	Reviewer         string `yaml:"reviewer"`
	QualityReviewer  string `yaml:"quality_reviewer"`
	SpecGenerator    string `yaml:"spec_generator"`
	Punchlist        string `yaml:"punchlist"`
}

// CLIFlags holds flag values passed on the command line.
// Pointer fields distinguish "not set" (nil) from zero values.
type CLIFlags struct {
	Repo       *string
	MaxRetries *int
	NoSandbox  *bool
	NoMerge    *bool
	Config     string
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

	applyFlags(cfg, flags)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		MaxRetries:             3,
		RoadmapPath:            "docs/ROADMAP.md",
		PlanningDir:            "docs/planning/",
		ScenarioDir:            "tests/scenarios/",
		ReviewDir:              "tests/review/",
		ArchitectureDoc:        "docs/architecture.md",
		ArchitectureJSON:       "docs/architecture.json",
		ConventionsDoc:         "docs/conventions.md",
		QualityStrictnessDecay: true,
		AuthPreference:         "oauth",
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
	if flags.NoMerge != nil {
		cfg.NoMerge = *flags.NoMerge
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
	return nil
}
