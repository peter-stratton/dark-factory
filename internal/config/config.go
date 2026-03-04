package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for a godark run.
type Config struct {
	Repo       string `yaml:"repo"`
	Milestone  string `yaml:"milestone"`
	Issue      int    `yaml:"issue"`
	MaxRetries int    `yaml:"max_retries"`

	AgentTimeout string       `yaml:"agent_timeout"`
	BuildCommand string       `yaml:"build_command"`
	TestCommand  string       `yaml:"test_command"`
	CrossCompile CrossCompile `yaml:"cross_compile"`

	ProtectedPaths []string `yaml:"protected_paths"`
	RoadmapPath    string   `yaml:"roadmap_path"`
	PlanningDir    string   `yaml:"planning_dir"`
	ScenarioDir    string   `yaml:"scenario_dir"`
	ReviewDir      string   `yaml:"review_dir"`
	LogDir         string   `yaml:"log_dir"`

	NoSandbox bool `yaml:"no_sandbox"`

	Docker  Docker  `yaml:"docker"`
	Prompts Prompts `yaml:"prompts"`
}

// CrossCompile holds cross-compilation environment variables.
type CrossCompile struct {
	GOOS   string `yaml:"GOOS"`
	GOARCH string `yaml:"GOARCH"`
}

// Docker holds Docker sandbox configuration.
type Docker struct {
	Image         string   `yaml:"image"`
	Dockerfile    string   `yaml:"dockerfile"`
	Mount         string   `yaml:"mount"`
	User          string   `yaml:"user"`
	GoVersion     string   `yaml:"go_version"`
	NodeVersion   string   `yaml:"node_version"`
	ExtraPackages []string `yaml:"extra_packages"`
}

// Prompts holds paths to prompt template files.
type Prompts struct {
	Implementer      string `yaml:"implementer"`
	ImplementerRetry string `yaml:"implementer_retry"`
	Reviewer         string `yaml:"reviewer"`
	SpecGenerator    string `yaml:"spec_generator"`
}

// CLIFlags holds flag values passed on the command line.
// Pointer fields distinguish "not set" (nil) from zero values.
type CLIFlags struct {
	Repo       *string
	Milestone  *string
	Issue      *int
	MaxRetries *int
	NoSandbox  *bool
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
		MaxRetries:  2,
		RoadmapPath: "docs/ROADMAP.md",
		PlanningDir: "docs/planning/",
		ScenarioDir: "tests/scenarios/",
		ReviewDir:   "tests/review/",
		LogDir:      "logs/",
	}
}

func applyFlags(cfg *Config, flags CLIFlags) {
	if flags.Repo != nil {
		cfg.Repo = *flags.Repo
	}
	if flags.Milestone != nil {
		cfg.Milestone = *flags.Milestone
	}
	if flags.Issue != nil {
		cfg.Issue = *flags.Issue
	}
	if flags.MaxRetries != nil {
		cfg.MaxRetries = *flags.MaxRetries
	}
	if flags.NoSandbox != nil {
		cfg.NoSandbox = *flags.NoSandbox
	}
}

func validate(cfg *Config) error {
	if cfg.Repo == "" {
		return fmt.Errorf("repo is required (set in config file or pass --repo)")
	}
	if cfg.Milestone == "" && cfg.Issue == 0 {
		return fmt.Errorf("milestone or issue is required (set in config file or pass --milestone / --issue)")
	}
	return nil
}
