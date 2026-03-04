package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// IssueOutcome records the result of processing a single issue.
type IssueOutcome struct {
	IssueNumber int
	Status      string // "implemented", "failed", "needs-human-review"
	PRNumber    int
	Retries     int
	Err         error
}

// ProcessIssue runs the full per-issue lifecycle:
// implement → find PR → guard rails → review/retry loop → merge or label.
func ProcessIssue(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) IssueOutcome {
	outcome := IssueOutcome{IssueNumber: issue.Number}

	slug := Slugify(issue.Title)
	branch := BranchName(issue.Number, slug)

	logger.Info("processing issue", "issue_number", issue.Number, "title", issue.Title)

	// Record base SHA for drift detection.
	baseSHAOut, err := GuardRunner("git", "rev-parse", "HEAD")
	if err != nil {
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("getting base SHA: %w", err)
		return outcome
	}
	baseSHA := trimOutput(baseSHAOut)

	// Step 0: Generate scenario spec if missing.
	if prompts.SpecGenerator != "" && !HasScenarioSpec(cfg.ScenarioDir, issue.Number) {
		logger.Info("no scenario spec found, generating", "issue_number", issue.Number)
		specResult, err := GenerateSpec(ctx, issue, cfg, prompts, authEnv, logger)
		if err != nil {
			logger.Warn("spec generation failed, continuing without spec", "error", err)
		} else if specResult.TimedOut {
			logger.Warn("spec generation timed out, continuing without spec")
		}
	}

	// Step 1: Implement.
	implResult, err := Implement(ctx, issue, cfg, prompts, authEnv, logger)
	if err != nil {
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("implementer agent: %w", err)
		return outcome
	}
	if implResult.TimedOut {
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("implementer agent timed out")
		return outcome
	}

	// Capture session ID so retries can resume the agent's context.
	sessionID := implResult.SessionID

	// Step 2: Find PR.
	prNum, err := FindPR(cfg.Repo, branch)
	if err != nil {
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("finding PR: %w", err)
		return outcome
	}
	if prNum == 0 {
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("implementer agent did not create a PR")
		return outcome
	}
	outcome.PRNumber = prNum

	// Step 3: Guard rails.
	if err := EnsureClosesRef(cfg.Repo, prNum, issue.Number); err != nil {
		logger.Warn("failed to ensure Closes ref", "error", err)
	}

	if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
		outcome.Status = "failed"
		outcome.Err = driftErr
		return outcome
	}

	_ = WarnMissingScenario(cfg.Repo, prNum, issue.Number, cfg.ScenarioDir, logger)

	// Step 4: Quality review gate (if prompt is configured).
	if prompts.QualityReviewer != "" {
		qualityMaxAttempts := cfg.MaxRetries + 1
		qualityPassed := false
		for qAttempt := 0; qAttempt < qualityMaxAttempts; qAttempt++ {
			if ctx.Err() != nil {
				outcome.Status = "failed"
				outcome.Err = ctx.Err()
				return outcome
			}

			qResult, err := QualityReview(ctx, issue, prNum, cfg, prompts, authEnv, logger)
			if err != nil {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("quality reviewer agent: %w", err)
				return outcome
			}

			switch qResult.Verdict {
			case "APPROVED":
				qualityPassed = true
			case "CHANGES_REQUESTED":
				retriesLeft := qualityMaxAttempts - qAttempt - 1
				if retriesLeft <= 0 {
					break // exit loop, will label needs-human-review
				}

				logger.Info("quality review requested changes, retrying implementation",
					"issue_number", issue.Number,
					"attempt", qAttempt+1,
					"retries_left", retriesLeft,
				)

				retryResult, err := Retry(ctx, issue, prNum, sessionID, cfg, prompts, authEnv, logger)
				if err != nil {
					outcome.Status = "failed"
					outcome.Err = fmt.Errorf("retry agent (quality): %w", err)
					return outcome
				}
				if retryResult.TimedOut {
					outcome.Status = "failed"
					outcome.Err = fmt.Errorf("retry agent (quality) timed out")
					return outcome
				}

				sessionID = retryResult.SessionID

				if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
					outcome.Status = "failed"
					outcome.Err = driftErr
					return outcome
				}

			default:
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("quality reviewer agent did not produce a verdict")
				return outcome
			}

			if qualityPassed {
				break
			}
		}

		if !qualityPassed {
			if err := LabelPR(cfg.Repo, prNum, "needs-human-review"); err != nil {
				logger.Warn("failed to label PR", "error", err)
			}
			comment := fmt.Sprintf("Exhausted %d quality review/retry cycles. Labeling for human review.", qualityMaxAttempts)
			if _, err := GuardRunner("gh", "pr", "comment", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--body", comment); err != nil {
				logger.Warn("failed to comment on PR", "error", err)
			}
			outcome.Status = "needs-human-review"
			outcome.Retries = qualityMaxAttempts - 1
			return outcome
		}
	}

	// Step 5: Review/retry loop.
	maxAttempts := cfg.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			outcome.Status = "failed"
			outcome.Err = ctx.Err()
			return outcome
		}

		reviewResult, err := Review(ctx, issue, prNum, cfg, prompts, authEnv, logger)
		if err != nil {
			outcome.Status = "failed"
			outcome.Err = fmt.Errorf("reviewer agent: %w", err)
			return outcome
		}

		switch reviewResult.Verdict {
		case "APPROVED":
			// Merge the PR.
			if _, err := GuardRunner("gh", "pr", "merge", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--squash", "--delete-branch"); err != nil {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("merging PR: %w", err)
				return outcome
			}
			outcome.Status = "implemented"
			outcome.Retries = attempt
			return outcome

		case "CHANGES_REQUESTED":
			retriesLeft := maxAttempts - attempt - 1
			if retriesLeft <= 0 {
				break // exit loop, will label needs-human-review
			}

			logger.Info("retrying implementation",
				"issue_number", issue.Number,
				"attempt", attempt+1,
				"retries_left", retriesLeft,
			)

			retryResult, err := Retry(ctx, issue, prNum, sessionID, cfg, prompts, authEnv, logger)
			if err != nil {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("retry agent: %w", err)
				return outcome
			}
			if retryResult.TimedOut {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("retry agent timed out")
				return outcome
			}

			// Update session ID so the next retry can resume this session's context.
			sessionID = retryResult.SessionID

			// Re-check drift after retry.
			if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
				outcome.Status = "failed"
				outcome.Err = driftErr
				return outcome
			}

		default:
			// No verdict found — treat as failure.
			outcome.Status = "failed"
			outcome.Err = fmt.Errorf("reviewer agent did not produce a verdict")
			return outcome
		}
	}

	// Exhausted retries.
	if err := LabelPR(cfg.Repo, prNum, "needs-human-review"); err != nil {
		logger.Warn("failed to label PR", "error", err)
	}
	comment := fmt.Sprintf("Exhausted %d review/retry cycles. Labeling for human review.", maxAttempts)
	if _, err := GuardRunner("gh", "pr", "comment", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--body", comment); err != nil {
		logger.Warn("failed to comment on PR", "error", err)
	}

	outcome.Status = "needs-human-review"
	outcome.Retries = maxAttempts - 1
	return outcome
}

// checkDriftAndClose checks for protected path modifications and closes the
// PR if any are found. Returns a non-nil error only when drift is detected.
func checkDriftAndClose(baseSHA string, cfg *config.Config, prNum int, logger *slog.Logger) error {
	touched, err := CheckProtectedDrift(baseSHA, cfg.ProtectedPaths)
	if err != nil {
		logger.Warn("failed to check protected drift", "error", err)
		return nil
	}
	if len(touched) == 0 {
		return nil
	}
	reason := fmt.Sprintf("Closing: agent modified protected paths: %v", touched)
	if closeErr := ClosePR(cfg.Repo, prNum, reason); closeErr != nil {
		logger.Warn("failed to close PR", "error", closeErr)
	}
	return fmt.Errorf("protected path drift: %v", touched)
}

func trimOutput(b []byte) string {
	s := string(b)
	if idx := len(s) - 1; idx >= 0 && s[idx] == '\n' {
		return s[:idx]
	}
	return s
}
