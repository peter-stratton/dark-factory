package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/label"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/spf13/cobra"
)

const defaultWatchPollInterval = 60 * time.Second

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll for PRs awaiting human review and detect CHANGES_REQUESTED reviews",
	Long: `Polls GitHub for pull requests labeled godark:awaiting-human-review and
detects when a human submits a CHANGES_REQUESTED review. When detected, the
godark:fixing-review-feedback label is applied and godark:awaiting-human-review
is removed.

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
		defer os.RemoveAll(logDir)

		logger, err := logging.NewLogger(logDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		return runWatch(ctx, cfg, logger)
	},
}

// runWatch runs the polling loop until ctx is cancelled.
func runWatch(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	interval, err := watchPollInterval(cfg)
	if err != nil {
		return err
	}

	logger.Info("starting watch", "repo", cfg.Repo, "poll_interval", interval)

	processed := make(map[int]bool)

	// Poll once immediately before the first tick so results appear right away.
	if err := pollOnce(ctx, cfg.Repo, processed, logger); err != nil && ctx.Err() == nil {
		logger.Error("poll error", "err", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("watch stopped")
			return nil
		case <-ticker.C:
			if err := pollOnce(ctx, cfg.Repo, processed, logger); err != nil && ctx.Err() == nil {
				logger.Error("poll error", "err", err)
			}
		}
	}
}

// pollOnce fetches PRs labeled godark:awaiting-human-review, inspects their
// reviews for CHANGES_REQUESTED, and swaps labels on any new findings.
// Processed review IDs are recorded in the processed map to avoid repeat actions.
func pollOnce(ctx context.Context, repo string, processed map[int]bool, logger *slog.Logger) error {
	prs, err := github.ListPRsWithLabel(repo, label.AwaitingHumanReview)
	if err != nil {
		return fmt.Errorf("listing PRs: %w", err)
	}

	logger.Info("poll", "repo", repo, "prs_awaiting_review", len(prs))

	for _, pr := range prs {
		if ctx.Err() != nil {
			return nil
		}

		reviews, err := github.FetchPRReviews(repo, pr.Number)
		if err != nil {
			logger.Error("fetching reviews", "pr", pr.Number, "err", err)
			continue
		}

		for _, review := range reviews {
			if review.State != "CHANGES_REQUESTED" {
				continue
			}
			if processed[review.ID] {
				continue
			}

			logger.Info("CHANGES_REQUESTED review detected",
				"pr", pr.Number,
				"review_id", review.ID,
				"author", review.Author,
			)

			if err := github.AddIssueLabel(repo, pr.Number, label.FixingReviewFeedback); err != nil {
				logger.Error("adding label", "pr", pr.Number, "label", label.FixingReviewFeedback, "err", err)
				continue
			}
			if err := github.RemoveIssueLabel(repo, pr.Number, label.AwaitingHumanReview); err != nil {
				logger.Error("removing label", "pr", pr.Number, "label", label.AwaitingHumanReview, "err", err)
				continue
			}

			processed[review.ID] = true
			logger.Info("labels updated",
				"pr", pr.Number,
				"added", label.FixingReviewFeedback,
				"removed", label.AwaitingHumanReview,
			)
		}
	}

	return nil
}

// watchPollInterval returns the configured poll interval, falling back to
// defaultWatchPollInterval when cfg.Watch is nil or PollInterval is empty.
// config.Load already validates PollInterval so ParseDuration cannot fail here.
func watchPollInterval(cfg *config.Config) (time.Duration, error) {
	if cfg.Watch == nil || cfg.Watch.PollInterval == "" {
		return defaultWatchPollInterval, nil
	}
	d, err := time.ParseDuration(cfg.Watch.PollInterval)
	if err != nil {
		return 0, fmt.Errorf("parsing watch.poll_interval: %w", err)
	}
	return d, nil
}

func init() {
	f := watchCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("config", "godark.yaml", "Path to configuration file")

	rootCmd.AddCommand(watchCmd)
}
