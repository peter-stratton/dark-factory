package watch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/label"
	"github.com/phs/dark-factory/internal/orchestrator"
	"github.com/phs/dark-factory/internal/rundata"
)

const defaultPollInterval = 60 * time.Second

// Watch polls GitHub for PRs awaiting human review and handles review events.
type Watch struct {
	cfg       *config.Config
	prompts   *agent.Prompts
	authEnv   map[string]string
	logger    *slog.Logger
	processed map[int]bool
}

// New creates a Watch instance with the given dependencies.
func New(cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) *Watch {
	return &Watch{
		cfg:       cfg,
		prompts:   prompts,
		authEnv:   authEnv,
		logger:    logger,
		processed: make(map[int]bool),
	}
}

// RunUntilDone runs the polling loop until ctx is cancelled or there are no
// more PRs labeled godark:awaiting-human-review. It is used by
// "godark run --watch" to stay alive after the first pass completes, then exit
// cleanly once all outstanding PRs are reviewed or merged.
func (w *Watch) RunUntilDone(ctx context.Context) error {
	interval, err := w.pollInterval()
	if err != nil {
		return err
	}

	w.logger.Info("starting watch (exit when empty)", "repo", w.cfg.Repo, "poll_interval", interval)

	// Poll once immediately before the first tick so results appear right away.
	if err := w.PollOnce(ctx); err != nil && ctx.Err() == nil {
		w.logger.Error("poll error", "err", err)
	}

	// Check remaining PRs after the initial poll; exit if none remain.
	prs, listErr := listPRsFn(w.cfg.Repo, label.AwaitingHumanReview)
	if listErr != nil && ctx.Err() == nil {
		w.logger.Error("listing PRs after poll", "err", listErr)
	}
	if len(prs) == 0 {
		w.logger.Info("no PRs awaiting review, watch exiting")
		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("watch stopped")
			return nil
		case <-ticker.C:
			if err := w.PollOnce(ctx); err != nil && ctx.Err() == nil {
				w.logger.Error("poll error", "err", err)
			}
			prs, listErr := listPRsFn(w.cfg.Repo, label.AwaitingHumanReview)
			if listErr != nil && ctx.Err() == nil {
				w.logger.Error("listing PRs after poll", "err", listErr)
			}
			if len(prs) == 0 {
				w.logger.Info("no PRs awaiting review, watch exiting")
				return nil
			}
		}
	}
}

// Run runs the polling loop until ctx is cancelled.
func (w *Watch) Run(ctx context.Context) error {
	interval, err := w.pollInterval()
	if err != nil {
		return err
	}

	w.logger.Info("starting watch", "repo", w.cfg.Repo, "poll_interval", interval)

	// Poll once immediately before the first tick so results appear right away.
	if err := w.PollOnce(ctx); err != nil && ctx.Err() == nil {
		w.logger.Error("poll error", "err", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("watch stopped")
			return nil
		case <-ticker.C:
			if err := w.PollOnce(ctx); err != nil && ctx.Err() == nil {
				w.logger.Error("poll error", "err", err)
			}
		}
	}
}

// PollOnce fetches PRs labeled godark:awaiting-human-review, inspects their
// reviews, and acts on new findings: CHANGES_REQUESTED invokes the implementer
// agent to address feedback; APPROVED merges the PR and closes the linked
// issue. Processed review IDs are recorded in the processed map to avoid
// repeat actions.
func (w *Watch) PollOnce(ctx context.Context) error {
	prs, err := github.ListPRsWithLabel(w.cfg.Repo, label.AwaitingHumanReview)
	if err != nil {
		return fmt.Errorf("listing PRs: %w", err)
	}

	w.logger.Info("poll", "repo", w.cfg.Repo, "prs_awaiting_review", len(prs))

	for _, pr := range prs {
		if ctx.Err() != nil {
			return nil
		}

		reviews, err := github.FetchPRReviews(w.cfg.Repo, pr.Number)
		if err != nil {
			w.logger.Error("fetching reviews", "pr", pr.Number, "err", err)
			continue
		}

		for _, review := range reviews {
			if review.State == "CHANGES_REQUESTED" {
				if w.processed[review.ID] {
					continue
				}

				w.logger.Info("CHANGES_REQUESTED review detected",
					"pr", pr.Number,
					"review_id", review.ID,
					"author", review.Author,
				)

				if err := github.AddIssueLabel(w.cfg.Repo, pr.Number, label.FixingReviewFeedback); err != nil {
					w.logger.Error("adding label", "pr", pr.Number, "label", label.FixingReviewFeedback, "err", err)
					continue
				}
				if err := github.RemoveIssueLabel(w.cfg.Repo, pr.Number, label.AwaitingHumanReview); err != nil {
					w.logger.Error("removing label", "pr", pr.Number, "label", label.AwaitingHumanReview, "err", err)
					continue
				}

				w.processed[review.ID] = true
				w.logger.Info("labels updated",
					"pr", pr.Number,
					"added", label.FixingReviewFeedback,
					"removed", label.AwaitingHumanReview,
				)

				w.HandleChangesRequested(ctx, pr, review)
			} else if review.State == "APPROVED" {
				if w.processed[review.ID] {
					continue
				}

				w.logger.Info("APPROVED review detected",
					"pr", pr.Number,
					"review_id", review.ID,
					"author", review.Author,
				)

				w.processed[review.ID] = true
				w.HandleApproved(ctx, pr, review)
				break // PR is now merged; no further review processing for this PR
			}
		}
	}

	return nil
}

// HandleChangesRequested invokes the implementer agent to address human review
// feedback. It retrieves the prior session ID, builds a feedback string from
// the review body and inline comments, calls agent.Retry, writes run data, and
// swaps the PR labels back to godark:awaiting-human-review.
func (w *Watch) HandleChangesRequested(ctx context.Context, pr github.PRInfo, review github.PRReview) {
	// Extract issue number from conventional branch name "{issueNum}-{slug}".
	issueNum := issueNumberFromBranch(pr.HeadRefName)
	if issueNum == 0 {
		w.logger.Warn("cannot extract issue number from branch, skipping agent invocation",
			"pr", pr.Number, "branch", pr.HeadRefName)
		return
	}

	issue, err := fetchIssueFn(w.cfg.Repo, issueNum)
	if err != nil {
		w.logger.Error("fetching issue", "issue_number", issueNum, "err", err)
		return
	}

	// Retrieve session ID from prior run data for session resumption.
	sessionID, err := findSessionIDFn(w.cfg.Repo, pr.Number)
	if err != nil {
		w.logger.Warn("finding session ID", "pr", pr.Number, "err", err)
		// Fall through with empty sessionID — agent will cold-start.
	}
	if sessionID != "" {
		w.logger.Info("resuming prior session", "pr", pr.Number, "session_id", sessionID)
	} else {
		w.logger.Info("no prior session found, cold-starting agent", "pr", pr.Number)
	}

	// Build feedback string: review body + inline review comments.
	feedback := buildFeedback(review.Body, fetchReviewCommentsFn, w.cfg.Repo, pr.Number, review.ID, w.logger)

	// Create a run data writer for this watch-initiated fix cycle.
	writer, writerErr := newWriterFn(w.cfg.Repo, "", []int{issueNum}, w.cfg.BaseBranch, rundata.AutoMerge{Feature: string(w.cfg.AutoMerge.Feature), Rollup: string(w.cfg.AutoMerge.Rollup)})
	if writerErr != nil {
		w.logger.Warn("failed to create run data writer", "err", writerErr)
	}

	// Open stats DB for this fix cycle; nil on failure (errors logged, never fatal).
	statsDB := orchestrator.OpenStatsDB(w.logger)
	if statsDB != nil {
		defer func() {
			if err := statsDB.Close(); err != nil {
				w.logger.Warn("stats: failed to close database", "error", err)
			}
		}()
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
			w.logger.Warn("failed to finalize run", "err", err)
		}
		orchestrator.WriteRunStats(ctx, statsDB, w.cfg, writer, summary, w.logger)
	}()

	w.logger.Info("invoking implementer retry for human review",
		"pr", pr.Number,
		"issue_number", issueNum,
		"resume_session", sessionID != "",
	)

	result, err := retryFn(ctx, issue, pr.Number, sessionID, feedback, w.cfg, w.prompts, w.authEnv, w.logger)
	if err != nil {
		w.logger.Error("implementer retry failed", "pr", pr.Number, "err", err)
		return
	}

	// Write run data for this fix attempt.
	if writer != nil {
		step := agent.ResultToStep(result)
		if writeErr := writer.WriteRetryResult(issueNum, 1, step); writeErr != nil {
			w.logger.Warn("failed to write retry result", "err", writeErr)
		}
	}

	// Swap labels: remove fixing-review-feedback, apply awaiting-human-review.
	if err := github.RemoveIssueLabel(w.cfg.Repo, pr.Number, label.FixingReviewFeedback); err != nil {
		w.logger.Error("removing label after fix", "pr", pr.Number, "label", label.FixingReviewFeedback, "err", err)
		return
	}
	if err := github.AddIssueLabel(w.cfg.Repo, pr.Number, label.AwaitingHumanReview); err != nil {
		w.logger.Error("adding label after fix", "pr", pr.Number, "label", label.AwaitingHumanReview, "err", err)
		return
	}

	succeeded = true
	w.logger.Info("fix cycle complete, awaiting human re-review",
		"pr", pr.Number,
		"issue_number", issueNum,
	)
}

// HandleApproved merges the PR (squash + delete branch), closes the linked
// issue, removes the godark:awaiting-human-review label, and writes run data.
// On merge failure the error is logged and the label is left in place for
// manual intervention.
func (w *Watch) HandleApproved(ctx context.Context, pr github.PRInfo, review github.PRReview) {
	issueNum := issueNumberFromBranch(pr.HeadRefName)
	if issueNum == 0 {
		w.logger.Warn("cannot extract issue number from branch, skipping merge",
			"pr", pr.Number, "branch", pr.HeadRefName)
		return
	}

	if err := mergePRFn(w.cfg.Repo, pr.Number); err != nil {
		w.logger.Error("merging approved PR", "pr", pr.Number, "err", err)
		return
	}

	w.logger.Info("PR merged", "pr", pr.Number, "issue_number", issueNum)

	if err := github.CloseIssue(w.cfg.Repo, issueNum); err != nil {
		w.logger.Warn("failed to close issue after merge", "issue", issueNum, "err", err)
	}

	if err := github.RemoveIssueLabel(w.cfg.Repo, pr.Number, label.AwaitingHumanReview); err != nil {
		w.logger.Warn("failed to remove label after merge", "pr", pr.Number, "label", label.AwaitingHumanReview, "err", err)
	}

	issue, err := fetchIssueFn(w.cfg.Repo, issueNum)
	if err != nil {
		w.logger.Warn("fetching issue for run data", "issue_number", issueNum, "err", err)
	}

	// Open stats DB for this approved-merge cycle; nil on failure (errors logged, never fatal).
	statsDB := orchestrator.OpenStatsDB(w.logger)
	if statsDB != nil {
		defer func() {
			if err := statsDB.Close(); err != nil {
				w.logger.Warn("stats: failed to close database", "error", err)
			}
		}()
	}

	writer, writerErr := newWriterFn(w.cfg.Repo, "", []int{issueNum}, w.cfg.BaseBranch, rundata.AutoMerge{Feature: string(w.cfg.AutoMerge.Feature), Rollup: string(w.cfg.AutoMerge.Rollup)})
	if writerErr != nil {
		w.logger.Warn("failed to create run data writer", "err", writerErr)
	}
	if writer != nil {
		if err := writer.WriteOutcome(rundata.Outcome{
			IssueNumber: issueNum,
			Title:       issue.Title,
			Status:      "implemented",
			PRNumber:    pr.Number,
		}); err != nil {
			w.logger.Warn("failed to write outcome", "err", err)
		}
		summary := rundata.RunSummary{Total: 1, Implemented: 1}
		if err := writer.FinalizeRun(summary); err != nil {
			w.logger.Warn("failed to finalize run", "err", err)
		}
		orchestrator.WriteRunStats(ctx, statsDB, w.cfg, writer, summary, w.logger)
	}

	w.logger.Info("approved PR merged and issue closed",
		"pr", pr.Number,
		"issue_number", issueNum,
		"reviewer", review.Author,
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

// pollInterval returns the configured poll interval, falling back to
// defaultPollInterval when cfg.Watch is nil or PollInterval is empty.
// config.Load already validates PollInterval so ParseDuration cannot fail here.
func (w *Watch) pollInterval() (time.Duration, error) {
	if w.cfg.Watch == nil || w.cfg.Watch.PollInterval == "" {
		return defaultPollInterval, nil
	}
	d, err := time.ParseDuration(w.cfg.Watch.PollInterval)
	if err != nil {
		return 0, fmt.Errorf("parsing watch.poll_interval: %w", err)
	}
	return d, nil
}

// Testability seams — replaced in unit tests to avoid real GitHub and agent calls.

var retryFn = func(ctx context.Context, issue github.Issue, prNum int, sessionID string, feedback string, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) (*agent.Result, error) {
	// watch-initiated retries always resume the prior session (no handoff context).
	return agent.Retry(ctx, issue, prNum, sessionID, feedback, "", cfg, prompts, authEnv, logger)
}

var findSessionIDFn = func(repo string, prNum int) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	runsDir := filepath.Join(home, ".godark", "runs")
	return rundata.FindSessionID(runsDir, repo, prNum, slog.Default())
}

var newWriterFn = rundata.New

var fetchReviewCommentsFn = github.FetchReviewComments

var fetchIssueFn = github.FetchIssue

var mergePRFn = github.MergeFeaturePR

var listPRsFn = github.ListPRsWithLabel
