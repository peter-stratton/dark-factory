package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/sandbox"
	"github.com/phs/dark-factory/internal/watch"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll for PRs awaiting human review and handle CHANGES_REQUESTED or APPROVED reviews",
	Long: `Polls GitHub for pull requests labeled godark:awaiting-human-review and
detects when a human submits a review. When CHANGES_REQUESTED is detected, the
implementer agent is invoked to address the feedback using session resumption.
After the fix is pushed, the PR is re-labeled godark:awaiting-human-review.
When APPROVED is detected, the PR is merged (squash + delete branch) and the
linked issue is closed.

Runs as a long-lived foreground process. Press Ctrl+C to stop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		flags := config.CLIFlags{Config: configPath}
		if cmd.Flags().Changed("repo") {
			v, _ := cmd.Flags().GetString("repo")
			flags.Repo = &v
		}

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		logDir, err := os.MkdirTemp("", "godark-watch-*")
		if err != nil {
			return fmt.Errorf("creating temp log dir: %w", err)
		}

		logger, err := logging.NewLogger(logDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}
		logger.Info("logging to", "dir", logDir)

		prompts, err := agent.LoadPrompts(cfg)
		if err != nil {
			return fmt.Errorf("loading prompts: %w", err)
		}

		authEnv, err := sandbox.CollectAuthEnv(logger, cfg.AuthPreference, cfg.RequiredEnv)
		if err != nil {
			return fmt.Errorf("collecting auth: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return watch.New(cfg, prompts, authEnv, logger).Run(ctx)
	},
}

func init() {
	f := watchCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("config", "godark.yaml", "Path to configuration file")

	rootCmd.AddCommand(watchCmd)
}
