package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/label"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/sandbox"
	"github.com/spf13/cobra"
)

const defaultWatchPollInterval = 60 * time.Second

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll for PRs awaiting human review and detect CHANGES_REQUESTED reviews",
	Long: `Polls GitHub for pull requests labeled godark:awaiting-human-review and
detects when a human submits a CHANGES_REQUESTED review. When detected, the
implementer agent is invoked to address the feedback using session resumption.
After the fix is pushed, the PR is re-labeled godark:awaiting-human-review.

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

		return runWatch(ctx, cfg, prompts, authEnv, logger)
	},
}

// runWatch runs the polling loop until ctx is cancelled.
func runWatch(ctx context.Context, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) error {
	interval, err := watchPollInterval(cfg)
	if err != nil {
		return err
	}

	logger.Info("starting watch", "repo", cfg.Repo, "poll_interval", interval)

	processed := make(map[int]bool)

	// Poll once immediately before the first tick so results appear right away.
	if err := pollOnce(ctx, cfg, prompts, authEnv, processed, logger); err != nil && ctx.Err() == nil {
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
			if err := pollOnce(ctx, cfg, prompts, authEnv, processed, logger); err != nil && ctx.Err() == nil {
				logger.Error("poll error", "err", err)
			}
		}
	}
}

// pollOnce fetches PRs labeled godark:awaiting-human-review, inspects their
// reviews for CHANGES_REQUESTED, and invokes the implementer agent for any
// new findings. Processed review IDs are recorded in the processed map to
// avoid repeat actions.
func pollOnce(ctx context.Context, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, processed map[int]bool, logger *slog.Logger) error {
	prs, err := github.ListPRsWithLabel(cfg.Repo, label.AwaitingHumanReview)
	if err != nil {
		return fmt.Errorf("listing PRs: %w", err)
	}

	logger.Info("poll", "repo", cfg.Repo, "prs_awaiting_review", len(prs))

	for _, pr := range prs {
		if ctx.Err() != nil {
			return nil
		}

		reviews, err := github.FetchPRReviews(cfg.Repo, pr.Number)
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

			if err := github.AddIssueLabel(cfg.Repo, pr.Number, label.FixingReviewFeedback); err != nil {
				logger.Error("adding label", "pr", pr.Number, "label", label.FixingReviewFeedback, "err", err)
				continue
			}
			if err := github.RemoveIssueLabel(cfg.Repo, pr.Number, label.AwaitingHumanReview); err != nil {
				logger.Error("removing label", "pr", pr.Number, "label", label.AwaitingHumanReview, "err", err)
				continue
			}

			processed[review.ID] = true
			logger.Info("labels updated",
				"pr", pr.Number,
				"added", label.FixingReviewFeedback,
				"removed", label.AwaitingHumanReview,
			)

			handleChangesRequested(ctx, cfg, prompts, authEnv, pr, review, logger)
		}
	}

	return nil
}

// handleChangesRequested invokes the implementer agent to address human review
// feedback. It retrieves the prior session ID, builds a feedback string from
// the review body and inline comments, calls agent.Retry, writes run data, and
// swaps the PR labels back to godark:awaiting-human-review.
func handleChangesRequested(ctx context.Context, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, pr github.PRInfo, review github.PRReview, logger *slog.Logger) {
	// Extract issue number from conventional branch name "{issueNum}-{slug}".
	issueNum := issueNumberFromBranch(pr.HeadRefName)
	if issueNum == 0 {
		logger.Warn("cannot extract issue number from branch, skipping agent invocation",
			"pr", pr.Number, "branch", pr.HeadRefName)
		return
	}

	issue, err := watchFetchIssueFn(cfg.Repo, issueNum)
	if err != nil {
		logger.Error("fetching issue", "issue_number", issueNum, "err", err)
		return
	}

	// Retrieve session ID from prior run data for session resumption.
	sessionID, err := watchFindSessionIDFn(cfg.Repo, pr.Number)
	if err != nil {
		logger.Warn("finding session ID", "pr", pr.Number, "err", err)
		// Fall through with empty sessionID — agent will cold-start.
	}
	if sessionID != "" {
		logger.Info("resuming prior session", "pr", pr.Number, "session_id", sessionID)
	} else {
		logger.Info("no prior session found, cold-starting agent", "pr", pr.Number)
	}

	// Build feedback string: review body + inline review comments.
	feedback := buildFeedback(review.Body, watchFetchReviewCommentsFn, cfg.Repo, pr.Number, review.ID, logger)

	// Create a run data writer for this watch-initiated fix cycle.
	writer, writerErr := watchNewWriterFn(cfg.Repo, "", []int{issueNum}, cfg.BaseBranch, rundata.AutoMerge{Feature: cfg.AutoMerge.Feature, Rollup: cfg.AutoMerge.Rollup})
	if writerErr != nil {
		logger.Warn("failed to create run data writer", "err", writerErr)
	}

	// finalizeWriter writes a completed run.json on all exit paths after writer creation.
	succeeded := false
	defer func() {
		if writer == nil {
			return
		}
		summary := rundata.RunSummary{Total: 1, Failed: 1}
		if succeeded {
			summary = rundata.RunSummary{Total: 1, Implemented: 1}
		}
		if err := writer.FinalizeRun(summary); err != nil {
			logger.Warn("failed to finalize run", "err", err)
		}
	}()

	logger.Info("invoking implementer retry for human review",
		"pr", pr.Number,
		"issue_number", issueNum,
		"resume_session", sessionID != "",
	)

	result, err := watchRetryFn(ctx, issue, pr.Number, sessionID, feedback, cfg, prompts, authEnv, logger)
	if err != nil {
		logger.Error("implementer retry failed", "pr", pr.Number, "err", err)
		return
	}

	// Write run data for this fix attempt.
	if writer != nil {
		step := agent.ResultToStep(result)
		if writeErr := writer.WriteRetryResult(issueNum, 1, step); writeErr != nil {
			logger.Warn("failed to write retry result", "err", writeErr)
		}
	}

	// Swap labels: remove fixing-review-feedback, apply awaiting-human-review.
	if err := github.RemoveIssueLabel(cfg.Repo, pr.Number, label.FixingReviewFeedback); err != nil {
		logger.Error("removing label after fix", "pr", pr.Number, "label", label.FixingReviewFeedback, "err", err)
		return
	}
	if err := github.AddIssueLabel(cfg.Repo, pr.Number, label.AwaitingHumanReview); err != nil {
		logger.Error("adding label after fix", "pr", pr.Number, "label", label.AwaitingHumanReview, "err", err)
		return
	}

	succeeded = true
	logger.Info("fix cycle complete, awaiting human re-review",
		"pr", pr.Number,
		"issue_number", issueNum,
	)
}

// buildFeedback concatenates the review body and any inline review comments
// into a single feedback string. Network errors fetching inline comments are
// logged and non-fatal.
func buildFeedback(reviewBody string, fetchCommentsFn func(string, int, int) ([]string, error), repo string, prNum, reviewID int, logger *slog.Logger) string {
	var parts []string
	if reviewBody != "" {
		parts = append(parts, reviewBody)
	}

	comments, err := fetchCommentsFn(repo, prNum, reviewID)
	if err != nil {
		logger.Warn("failed to fetch inline review comments", "pr", prNum, "err", err)
	} else {
		parts = append(parts, comments...)
	}

	return strings.Join(parts, "\n\n")
}

// issueNumberFromBranch extracts the issue number from a conventional branch
// name of the form "{issueNum}-{slug}". Returns 0 if the format is unexpected.
func issueNumberFromBranch(headRefName string) int {
	idx := strings.Index(headRefName, "-")
	if idx <= 0 {
		return 0
	}
	n, err := strconv.Atoi(headRefName[:idx])
	if err != nil {
		return 0
	}
	return n
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

// Testability seams — replaced in unit tests to avoid real GitHub and agent calls.

var watchRetryFn = func(ctx context.Context, issue github.Issue, prNum int, sessionID string, feedback string, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) (*agent.Result, error) {
	// watch-initiated retries always resume the prior session (no handoff context).
	return agent.Retry(ctx, issue, prNum, sessionID, feedback, "", cfg, prompts, authEnv, logger)
}

var watchFindSessionIDFn = func(repo string, prNum int) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	runsDir := filepath.Join(home, ".godark", "runs")
	return rundata.FindSessionID(runsDir, repo, prNum, slog.Default())
}

var watchNewWriterFn = rundata.New

var watchFetchReviewCommentsFn = github.FetchReviewComments

var watchFetchIssueFn = github.FetchIssue

func init() {
	f := watchCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("config", "godark.yaml", "Path to configuration file")

	rootCmd.AddCommand(watchCmd)
}
