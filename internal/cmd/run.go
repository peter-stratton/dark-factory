package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/orchestrator"
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

		flags := config.CLIFlags{Config: configPath}

		if cmd.Flags().Changed("repo") {
			v, _ := cmd.Flags().GetString("repo")
			flags.Repo = &v
		}
		if cmd.Flags().Changed("milestone") {
			v, _ := cmd.Flags().GetString("milestone")
			flags.Milestone = &v
		}
		if cmd.Flags().Changed("issue") {
			v, _ := cmd.Flags().GetInt("issue")
			flags.Issue = &v
		}
		if cmd.Flags().Changed("max-retries") {
			v, _ := cmd.Flags().GetInt("max-retries")
			flags.MaxRetries = &v
		}
		if cmd.Flags().Changed("no-sandbox") {
			v, _ := cmd.Flags().GetBool("no-sandbox")
			flags.NoSandbox = &v
		}

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if cfg.NoSandbox {
			fmt.Fprintln(os.Stderr, "WARNING: running without sandbox — agent execution is not containerized")
		}

		logger, err := logging.NewLogger(cfg.LogDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return orchestrator.Run(ctx, cfg, logger, dryRun)
	},
}

func init() {
	f := runCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to process")
	f.Int("issue", 0, "Single issue number to process (instead of milestone)")
	f.Int("max-retries", 2, "Maximum review/fix retry cycles per issue")
	f.Bool("dry-run", false, "Print execution plan without taking action")
	f.Bool("no-sandbox", false, "Run agents on host instead of in Docker")
	f.String("config", "godark.yaml", "Path to configuration file")

	rootCmd.AddCommand(runCmd)
}
