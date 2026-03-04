package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/detect"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/orchestrator"
	"github.com/phs/dark-factory/internal/punchlist"
	"github.com/phs/dark-factory/internal/sandbox"
	"github.com/spf13/cobra"
)

var implementCmd = &cobra.Command{
	Use:   "implement <issue-number>",
	Short: "Implement a single GitHub issue",
	Long: `Fetch a GitHub issue by number and run the implement → review → merge
loop directly, without milestone or dependency resolution.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueNumber, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid issue number %q: %w", args[0], err)
		}

		configPath, _ := cmd.Flags().GetString("config")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

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
		if cmd.Flags().Changed("no-merge") {
			v, _ := cmd.Flags().GetBool("no-merge")
			flags.NoMerge = &v
		}

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		issue, err := github.FetchIssue(cfg.Repo, issueNumber)
		if err != nil {
			return fmt.Errorf("fetching issue #%d: %w", issueNumber, err)
		}

		if dryRun {
			fmt.Printf("Issue #%d: %s\n", issue.Number, issue.Title)
			fmt.Printf("Labels: %v\n", issue.Labels)
			fmt.Printf("Body:\n%s\n", issue.Body)
			return nil
		}

		if cfg.NoSandbox {
			fmt.Fprintln(os.Stderr, "WARNING: running without sandbox — agent execution is not containerized")
		}

		logger, err := logging.NewLogger(cfg.LogDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}

		// Auto-detect project type when no runtime/commands are explicitly configured.
		detect.ApplyToConfig(cfg, ".", logger)

		authEnv, err := sandbox.CollectAuthEnv(logger)
		if err != nil {
			return fmt.Errorf("collecting auth: %w", err)
		}

		prompts, err := agent.LoadPrompts(cfg)
		if err != nil {
			return fmt.Errorf("loading prompts: %w", err)
		}

		if !cfg.NoSandbox {
			dc := sandbox.DockerConfigFromConfig(cfg.Docker, cfg.Runtime, cfg.SandboxEnv)
			tag, err := sandbox.BuildImage(cmd.Context(), dc, logger)
			if err != nil {
				return fmt.Errorf("building Docker image: %w", err)
			}
			cfg.Docker.Image = tag
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		outcome := agent.ProcessIssue(ctx, issue, cfg, prompts, authEnv, logger)
		if outcome.Err != nil {
			return fmt.Errorf("issue #%d failed: %w", issueNumber, outcome.Err)
		}

		fmt.Printf("Issue #%d: %s (PR #%d, %d retries)\n",
			outcome.IssueNumber, outcome.Status, outcome.PRNumber, outcome.Retries)

		if outcome.Status == "implemented" {
			if err := orchestrator.PullAfterMerge(logger); err != nil {
				logger.Warn("could not sync local repo after merge", "error", err)
			}
		}

		if outcome.PRNumber > 0 {
			punchlistPath, _ := cmd.Flags().GetString("punchlist")
			files, err := punchlist.FetchChangedFiles(cfg.Repo, outcome.PRNumber)
			if err != nil {
				logger.Warn("failed to fetch changed files for punchlist",
					"pr_number", outcome.PRNumber, "error", err)
			}
			spec, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issueNumber)
			if err != nil {
				logger.Warn("failed to read scenario spec for punchlist",
					"issue_number", issueNumber, "error", err)
			}
			entries := []punchlist.Entry{{
				IssueNumber:  issue.Number,
				IssueTitle:   issue.Title,
				IssueBody:    issue.Body,
				PRNumber:     outcome.PRNumber,
				Repo:         cfg.Repo,
				ScenarioSpec: spec,
				ChangedFiles: files,
			}}
			text := punchlist.Generate(entries)
			fmt.Println()
			if err := punchlist.Write(text, punchlistPath); err != nil {
				logger.Warn("failed to write punchlist", "error", err)
			}
		}

		return nil
	},
}

func init() {
	f := implementCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.Int("max-retries", 3, "Maximum review/fix retry cycles")
	f.Bool("dry-run", false, "Print issue details and exit")
	f.Bool("no-sandbox", false, "Run agents on host instead of in Docker")
	f.Bool("no-merge", false, "Skip PR merge after approval (human reviews and merges manually)")
	f.String("config", "godark.yaml", "Path to configuration file")
	f.String("punchlist", "", "Write manual testing punchlist to this file (always printed to stdout)")

	rootCmd.AddCommand(implementCmd)
}
