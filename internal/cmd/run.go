package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/orchestrator"
	"github.com/phs/dark-factory/internal/pypi"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the development loop for a milestone or single issue",
	Long: `Fetch issues from a GitHub milestone, resolve dependencies, and process
each unblocked issue through the implement → review → merge loop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")

		flags := config.CLIFlags{Config: configPath}

		if cmd.Flags().Changed("repo") {
			v, _ := cmd.Flags().GetString("repo")
			flags.Repo = &v
		}
		if cmd.Flags().Changed("max-retries") {
			v, _ := cmd.Flags().GetInt("max-retries")
			flags.MaxRetries = &v
		}
		if cmd.Flags().Changed("no-sandbox") {
			v, _ := cmd.Flags().GetBool("no-sandbox")
			flags.NoSandbox = &v
		}
		if cmd.Flags().Changed("auto-merge-feature") {
			v, _ := cmd.Flags().GetString("auto-merge-feature")
			flags.AutoMergeFeature = &v
		}
		if cmd.Flags().Changed("auto-merge-rollup") {
			v, _ := cmd.Flags().GetString("auto-merge-rollup")
			flags.AutoMergeRollup = &v
		}
		if cmd.Flags().Changed("base-branch") {
			v, _ := cmd.Flags().GetString("base-branch")
			flags.BaseBranch = &v
		}
		if cmd.Flags().Changed("default-branch") {
			v, _ := cmd.Flags().GetString("default-branch")
			flags.DefaultBranch = &v
		}

		// Parse milestone/issue locally — these are per-run params, not config.
		var milestone string
		var issue int

		if cmd.Flags().Changed("tag") && cmd.Flags().Changed("milestone") {
			return fmt.Errorf("--milestone and --tag are mutually exclusive")
		}
		if cmd.Flags().Changed("tag") {
			tag, _ := cmd.Flags().GetString("tag")
			// Need repo to resolve tag — check flag first, then fall back to config.
			repo := flags.Repo
			if repo == nil {
				cfgOnly, err := config.Load(configPath, config.CLIFlags{Config: configPath})
				if err != nil {
					return fmt.Errorf("loading config to resolve --tag: %w", err)
				}
				if cfgOnly.Repo != "" {
					r := cfgOnly.Repo
					repo = &r
				}
			}
			if repo == nil || *repo == "" {
				return fmt.Errorf("--repo is required when using --tag")
			}
			resolved, err := github.ResolveMilestoneByTag(*repo, tag)
			if err != nil {
				return err
			}
			milestone = resolved
		} else if cmd.Flags().Changed("milestone") {
			milestone, _ = cmd.Flags().GetString("milestone")
		}
		if cmd.Flags().Changed("issue") {
			issue, _ = cmd.Flags().GetInt("issue")
		}

		if milestone == "" && issue == 0 {
			return fmt.Errorf("milestone or issue is required (pass --milestone, --tag, or --issue)")
		}

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if err := config.ValidateRequiredEnv(cfg.RequiredEnv); err != nil {
			return err
		}

		if cfg.NoSandbox {
			fmt.Fprintln(os.Stderr, "WARNING: running without sandbox — agent execution is not containerized")
		}

		// Use a private temp directory for bootstrap logging. The orchestrator will
		// create a logger in the run directory once the RunDataWriter is set up.
		// Always remove the temp dir on exit so bootstrap logs don't accumulate.
		logDir, err := os.MkdirTemp("", "godark-log-*")
		if err != nil {
			return fmt.Errorf("creating temp log dir: %w", err)
		}
		defer os.RemoveAll(logDir)
		logger, err := logging.NewLogger(logDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}

		pypi.WarnIfSDKOutdated(os.Stderr, logger)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// Ensure base branch exists on the remote before starting the run (no-op if empty or dry-run).
		if !dryRun {
			if err := orchestrator.EnsureBaseBranch(cfg.BaseBranch, logger); err != nil {
				return fmt.Errorf("ensuring base branch: %w", err)
			}
		}

		punchlistPath, _ := cmd.Flags().GetString("punchlist")
		return orchestrator.Run(ctx, cfg, logger, milestone, issue, dryRun, force, punchlistPath)
	},
}

func init() {
	f := runCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to process (exact title)")
	f.String("tag", "", "Milestone tag (e.g., phase-3) — resolved to full milestone title")
	f.Int("issue", 0, "Single issue number to process (instead of milestone)")
	f.Int("max-retries", 3, "Maximum review/fix retry cycles per issue")
	f.Bool("dry-run", false, "Print execution plan without taking action")
	f.Bool("force", false, "Clear any existing run lock before starting (override stale lock)")
	f.Bool("no-sandbox", false, "Run agents on host instead of in Docker")
	f.String("auto-merge-feature", "none", "Feature branch merge strategy after approval: none (human merges), low_risk (auto-merge small/safe PRs), all (auto-merge everything)")
	f.String("auto-merge-rollup", "none", "Rollup merge strategy after run: none (no rollup PR), manual (create PR but don't merge), auto (create and merge rollup PR)")
	f.String("config", "godark.yaml", "Path to configuration file")
	f.String("punchlist", "", "Write manual testing punchlist to this file (always printed to stdout)")
	f.String("base-branch", "", "Base branch for PRs (overrides repo default branch)")
	f.String("default-branch", "", "Default branch of the repository (auto-detected if omitted)")

	rootCmd.AddCommand(runCmd)
}
