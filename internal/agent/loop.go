package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/label"
	"github.com/phs/dark-factory/internal/quality"
	"github.com/phs/dark-factory/internal/rundata"
)

// IssueOutcome records the result of processing a single issue.
type IssueOutcome struct {
	IssueNumber int
	Status      string // "implemented", "ready-to-merge", "failed", "needs-human-review"
	PRNumber    int
	Retries     int
	Err         error
}

// ProcessIssue runs the full per-issue lifecycle:
// implement → find PR → guard rails → review/retry loop → merge or label.
// hook is optional; if non-nil, it is called after each agent step to record
// run data. Hook errors are logged as warnings and do not abort processing.
func ProcessIssue(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger, hook RunDataHook) IssueOutcome {
	outcome := IssueOutcome{IssueNumber: issue.Number}

	// Write outcome data on every return path.
	defer func() {
		if hook != nil {
			if err := hook.WriteOutcome(rundata.Outcome{
				IssueNumber: outcome.IssueNumber,
				Title:       issue.Title,
				Description: issue.Body,
				Status:      outcome.Status,
				PRNumber:    outcome.PRNumber,
			}); err != nil {
				logger.Warn("failed to write outcome", "error", err)
			}
		}
	}()

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
	specGenerated := false
	if prompts.SpecGenerator != "" && !HasScenarioSpec(cfg.ScenarioDir, issue.Number) {
		logger.Info("no scenario spec found, generating", "issue_number", issue.Number)
		specResult, err := GenerateSpec(ctx, issue, cfg, prompts, authEnv, logger)
		if err != nil {
			logger.Warn("spec generation failed, continuing without spec", "error", err)
			if hook != nil {
				step := rundata.StepResult{Error: err.Error()}
				if writeErr := hook.WriteSpecGeneratorResult(issue.Number, step); writeErr != nil {
					logger.Warn("failed to write spec generator result", "error", writeErr)
				}
			}
		} else if specResult.TimedOut {
			logger.Warn("spec generation timed out, continuing without spec")
			if hook != nil {
				step := ResultToStep(specResult)
				step.Error = "timed out"
				if writeErr := hook.WriteSpecGeneratorResult(issue.Number, step); writeErr != nil {
					logger.Warn("failed to write spec generator result", "error", writeErr)
				}
			}
		} else {
			// The spec was committed to the remote branch inside the container.
			// The file isn't on the host, but the reviewer container will see it
			// when it checks out the PR branch. After merge and pull, the spec
			// lands on main permanently.
			specGenerated = true
			logger.Info("spec generated on remote branch", "branch", branch)
			if hook != nil {
				if writeErr := hook.WriteSpecGeneratorResult(issue.Number, ResultToStep(specResult)); writeErr != nil {
					logger.Warn("failed to write spec generator result", "error", writeErr)
				}
			}
		}
	}

	// Step 1: Implement.
	if hook != nil {
		if err := hook.WriteIssueStatus(issue.Number, rundata.IssueStatus{Status: "implementing"}); err != nil {
			logger.Warn("failed to write issue status", "error", err)
		}
	}
	implResult, err := Implement(ctx, issue, cfg, prompts, authEnv, logger)
	if err != nil {
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("implementer agent: %w", err)
		return outcome
	}
	if implResult.TimedOut {
		runPostMortem(issue.Number, implResult, hook, logger)
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("implementer agent timed out")
		return outcome
	}
	if hook != nil {
		if err := hook.WriteImplementResult(issue.Number, ResultToStep(implResult)); err != nil {
			logger.Warn("failed to write implement result", "error", err)
		}
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
		runPostMortem(issue.Number, implResult, hook, logger)
		outcome.Status = "failed"
		outcome.Err = fmt.Errorf("implementer agent did not create a PR")
		return outcome
	}
	outcome.PRNumber = prNum

	// Track the current PR lifecycle label for state machine enforcement.
	// Initialized to "" (no label). Updated by applyLifecycleLabel on success.
	currentLifecycleLabel := ""

	// applyLifecycleLabel applies lbl to the PR, first asserting the
	// label.Transition state machine. Logs a warning (but does not abort) on
	// illegal transitions or API errors.
	applyLifecycleLabel := func(lbl string) {
		if !label.Transition(currentLifecycleLabel, lbl) {
			logger.Warn("invalid lifecycle label transition",
				"from", currentLifecycleLabel, "to", lbl, "pr_number", prNum)
		}
		if err := github.AddIssueLabel(cfg.Repo, prNum, lbl); err != nil {
			logger.Warn("failed to apply lifecycle label", "label", lbl, "pr_number", prNum, "error", err)
			return
		}
		currentLifecycleLabel = lbl
	}

	// Step 3: Guard rails.
	if err := EnsureClosesRef(cfg.Repo, prNum, issue.Number); err != nil {
		logger.Warn("failed to ensure Closes ref", "error", err)
	}

	if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
		outcome.Status = "failed"
		outcome.Err = driftErr
		return outcome
	}

	if !specGenerated {
		_ = WarnMissingScenario(cfg.Repo, prNum, issue.Number, cfg.ScenarioDir, logger)
	}

	// Determine whether a scenario spec exists for this issue. Used by the
	// functional reviewer's quality check: tests are only expected when a spec exists.
	hasSpec := specGenerated || HasScenarioSpec(cfg.ScenarioDir, issue.Number)

	// fixCycles counts the total number of verify-fix attempts used across all
	// modules. Used by the risk classifier to assess PR safety.
	fixCycles := 0

	// Step 3.5: Verify. Runs between guard rails and quality review.
	if cfg.Modules != nil {
		// Per-module verification in dependency order.
		sortedMods, err := topologicalSortModules(cfg.Modules)
		if err != nil {
			outcome.Status = "failed"
			outcome.Err = fmt.Errorf("sorting modules for verify: %w", err)
			return outcome
		}

		var verifyRunner CommandRunner
		if cfg.NoSandbox {
			verifyRunner = newHostRunner()
		} else {
			verifyRunner = sandboxCommandRunner(cfg.Docker.Image, cfg.Repo, branch, authEnv, logger)
		}

		moduleFailed := false
		var failedModName string

		for _, modName := range sortedMods {
			mod := cfg.Modules[modName]
			moduleChecks := buildModuleVerifyChecks(modName, mod, cfg)
			if len(moduleChecks) == 0 {
				continue
			}

			logger.Info("running verify step for module",
				"issue_number", issue.Number,
				"module", modName,
				"check_count", len(moduleChecks),
			)
			verifyResult := RunVerify(ctx, moduleChecks, verifyRunner)
			if hook != nil {
				if err := hook.WriteVerifyResult(issue.Number, verifyToRundata(verifyResult, 0, false)); err != nil {
					logger.Warn("failed to write verify result", "error", err)
				}
			}

			if !verifyResult.AllPassed && prompts.VerifyFix != "" && cfg.Verify.MaxFixAttempts > 0 {
				for fixAttempt := 0; fixAttempt < cfg.Verify.MaxFixAttempts; fixAttempt++ {
					if ctx.Err() != nil {
						outcome.Status = "failed"
						outcome.Err = ctx.Err()
						return outcome
					}
					fixCycles++

					verifyErrors := fmt.Sprintf("Module: %s\n\n%s", modName, formatVerifyErrors(verifyResult))
					logger.Info("running verify-fix attempt",
						"issue_number", issue.Number,
						"module", modName,
						"attempt", fixAttempt+1,
						"max_attempts", cfg.Verify.MaxFixAttempts,
					)

					fixResult, err := VerifyFix(ctx, issue, prNum, verifyErrors, sessionID, cfg, prompts, authEnv, logger)
					if err != nil {
						outcome.Status = "failed"
						outcome.Err = fmt.Errorf("verify-fix agent: %w", err)
						return outcome
					}
					if fixResult.TimedOut {
						outcome.Status = "failed"
						outcome.Err = fmt.Errorf("verify-fix agent timed out")
						return outcome
					}
					sessionID = fixResult.SessionID

					if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
						outcome.Status = "failed"
						outcome.Err = driftErr
						return outcome
					}

					verifyResult = RunVerify(ctx, moduleChecks, verifyRunner)
					if hook != nil {
						if err := hook.WriteVerifyResult(issue.Number, verifyToRundata(verifyResult, fixAttempt+1, true)); err != nil {
							logger.Warn("failed to write verify result", "error", err)
						}
					}
					if verifyResult.AllPassed {
						break
					}
				}
			}

			if verifyResult.AllPassed {
				logger.Info("verify step passed for module", "issue_number", issue.Number, "module", modName)
			} else {
				var failedNames []string
				for _, cr := range verifyResult.Checks {
					if !cr.Passed {
						failedNames = append(failedNames, cr.Name)
					}
				}
				logger.Warn("verify step failed for module",
					"issue_number", issue.Number,
					"module", modName,
					"failed_checks", failedNames,
				)
				moduleFailed = true
				failedModName = modName
				break // Stop processing dependent modules.
			}
		}

		if moduleFailed {
			if cfg.Verify.Blocking {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("verify failed for module %s", failedModName)
				return outcome
			}
			logger.Warn("module verify failed (non-blocking), proceeding to review",
				"issue_number", issue.Number,
				"module", failedModName,
			)
		}
	} else if verifyChecks := buildVerifyChecks(cfg); len(verifyChecks) > 0 {
		var verifyRunner CommandRunner
		if cfg.NoSandbox {
			verifyRunner = newHostRunner()
		} else {
			verifyRunner = sandboxCommandRunner(cfg.Docker.Image, cfg.Repo, branch, authEnv, logger)
		}
		logger.Info("running verify step", "issue_number", issue.Number, "check_count", len(verifyChecks))
		verifyResult := RunVerify(ctx, verifyChecks, verifyRunner)
		if hook != nil {
			if err := hook.WriteVerifyResult(issue.Number, verifyToRundata(verifyResult, 0, false)); err != nil {
				logger.Warn("failed to write verify result", "error", err)
			}
		}

		if !verifyResult.AllPassed && prompts.VerifyFix != "" && cfg.Verify.MaxFixAttempts > 0 {
			for fixAttempt := 0; fixAttempt < cfg.Verify.MaxFixAttempts; fixAttempt++ {
				if ctx.Err() != nil {
					outcome.Status = "failed"
					outcome.Err = ctx.Err()
					return outcome
				}
				fixCycles++

				verifyErrors := formatVerifyErrors(verifyResult)
				logger.Info("running verify-fix attempt",
					"issue_number", issue.Number,
					"attempt", fixAttempt+1,
					"max_attempts", cfg.Verify.MaxFixAttempts,
				)

				fixResult, err := VerifyFix(ctx, issue, prNum, verifyErrors, sessionID, cfg, prompts, authEnv, logger)
				if err != nil {
					outcome.Status = "failed"
					outcome.Err = fmt.Errorf("verify-fix agent: %w", err)
					return outcome
				}
				if fixResult.TimedOut {
					outcome.Status = "failed"
					outcome.Err = fmt.Errorf("verify-fix agent timed out")
					return outcome
				}
				sessionID = fixResult.SessionID

				// Re-check drift after each fix attempt.
				if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
					outcome.Status = "failed"
					outcome.Err = driftErr
					return outcome
				}

				// Re-run verify.
				verifyResult = RunVerify(ctx, verifyChecks, verifyRunner)
				if hook != nil {
					if err := hook.WriteVerifyResult(issue.Number, verifyToRundata(verifyResult, fixAttempt+1, true)); err != nil {
						logger.Warn("failed to write verify result", "error", err)
					}
				}
				if verifyResult.AllPassed {
					break
				}
			}
		}

		if verifyResult.AllPassed {
			logger.Info("verify step passed", "issue_number", issue.Number)
		} else {
			var failedNames []string
			for _, cr := range verifyResult.Checks {
				if !cr.Passed {
					failedNames = append(failedNames, cr.Name)
				}
			}
			if cfg.Verify.Blocking {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("verify failed: %s", strings.Join(failedNames, ", "))
				return outcome
			}
			logger.Warn("verify step failed (non-blocking), proceeding to review",
				"issue_number", issue.Number,
				"failed_checks", failedNames,
			)
		}
	}

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

			qResult, err := QualityReview(ctx, issue, prNum, cfg, prompts, authEnv, logger, qAttempt)
			if err != nil {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("quality reviewer agent: %w", err)
				return outcome
			}
			// Quality reviewer is exempt from CheckReviewTestExecution.
			qFlags := computeReviewFlags(qResult.AgentResult, cfg, false, false)
			qRDFlags := logAndRecordFlags(qFlags, logger, issue.Number)
			if hook != nil {
				qStep := ResultToStep(qResult.AgentResult)
				qStep.Flags = qRDFlags
				if qAttempt == 0 {
					if err := hook.WriteReviewResult(issue.Number, "quality", qStep); err != nil {
						logger.Warn("failed to write quality review result", "error", err)
					}
				} else {
					if err := hook.WriteRetryReviewResult(issue.Number, qAttempt-1, qStep); err != nil {
						logger.Warn("failed to write retry review result", "error", err)
					}
				}
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

				retryResult, err := Retry(ctx, issue, prNum, "", "", cfg, prompts, authEnv, logger)
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
				if hook != nil {
					if err := hook.WriteRetryResult(issue.Number, qAttempt, ResultToStep(retryResult)); err != nil {
						logger.Warn("failed to write retry result", "error", err)
					}
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
			applyLifecycleLabel(label.AwaitingHumanReview)
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
	if hook != nil {
		if err := hook.WriteIssueStatus(issue.Number, rundata.IssueStatus{Status: "in_review"}); err != nil {
			logger.Warn("failed to write issue status", "error", err)
		}
	}
	maxAttempts := cfg.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			outcome.Status = "failed"
			outcome.Err = ctx.Err()
			return outcome
		}

		reviewResult, err := Review(ctx, issue, prNum, cfg, prompts, authEnv, logger, hasSpec)
		if err != nil {
			outcome.Status = "failed"
			outcome.Err = fmt.Errorf("reviewer agent: %w", err)
			return outcome
		}
		// Functional reviewer is subject to all quality checks including CheckReviewTestExecution.
		fFlags := computeReviewFlags(reviewResult.AgentResult, cfg, true, hasSpec)
		fRDFlags := logAndRecordFlags(fFlags, logger, issue.Number)
		fStep := ResultToStep(reviewResult.AgentResult)
		fStep.Flags = fRDFlags

		// Pre-merge guard: if the reviewer approved without writing tests to the
		// review dir, re-run the reviewer. The reviewer is the agent responsible
		// for writing tests to ReviewDir — retrying the implementer would be
		// pointless since the implementer is forbidden from touching ReviewDir.
		if reviewResult.Verdict == "APPROVED" && hasQualityFlag(fFlags, "no_review_tests_written") {
			logger.Warn("functional review approved without writing tests — re-running reviewer",
				"issue_number", issue.Number,
				"attempt", attempt+1,
			)
			continue
		}

		switch reviewResult.Verdict {
		case "APPROVED":
			// Final result — write to top-level functional-review.json.
			if hook != nil {
				if err := hook.WriteReviewResult(issue.Number, "functional", fStep); err != nil {
					logger.Warn("failed to write functional review result", "error", err)
				}
			}
			// Merge decision: explicit opt-in only. Merge requires
			// auto_merge="all" or "low_risk" (with passing risk check).
			// Any other value — including "none" or unexpected — skips
			// merge safely. This prevents accidental merges if the
			// config value is empty or unrecognized.
			shouldMerge := false
			switch cfg.AutoMerge {
			case "all":
				logger.Info("PR approved, will merge (auto_merge=all)", "pr_number", prNum)
				shouldMerge = true
			case "low_risk":
				additions, deletions, fileCount, statsErr := github.FetchPRStats(cfg.Repo, prNum)
				if statsErr != nil {
					logger.Warn("failed to fetch PR stats for risk classification, labeling for human review",
						"pr_number", prNum, "error", statsErr)
					applyLifecycleLabel(label.AwaitingHumanReview)
					outcome.Status = "ready-to-merge"
					outcome.Retries = attempt
					return outcome
				}
				changedFiles, filesErr := github.FetchPRChangedFiles(cfg.Repo, prNum)
				if filesErr != nil {
					logger.Warn("failed to fetch PR changed files for risk classification, labeling for human review",
						"pr_number", prNum, "error", filesErr)
					applyLifecycleLabel(label.AwaitingHumanReview)
					outcome.Status = "ready-to-merge"
					outcome.Retries = attempt
					return outcome
				}

				var maxLines, maxFiles int
				if cfg.RiskThresholds != nil {
					maxLines = cfg.RiskThresholds.MaxLines
					maxFiles = cfg.RiskThresholds.MaxFiles
				}
				riskInput := quality.RiskInput{
					LinesChanged:   additions + deletions,
					FilesChanged:   fileCount,
					ChangedFiles:   changedFiles,
					ProtectedPaths: cfg.ProtectedPaths,
					FixCycles:      fixCycles,
					QualityFlags:   fFlags,
				}
				assessment := quality.ClassifyRisk(riskInput, maxLines, maxFiles)

				if hook != nil {
					if err := hook.WriteRiskAssessment(issue.Number, toRundataRiskAssessment(assessment)); err != nil {
						logger.Warn("failed to write risk assessment", "error", err)
					}
				}

				logger.Info("risk classification result",
					"issue_number", issue.Number,
					"pr_number", prNum,
					"is_low_risk", assessment.IsLowRisk,
				)

				if !assessment.IsLowRisk {
					logger.Info("PR is not low-risk, labeling for human review", "pr_number", prNum)
					applyLifecycleLabel(label.AwaitingHumanReview)
					outcome.Status = "ready-to-merge"
					outcome.Retries = attempt
					return outcome
				}
				shouldMerge = true
			default:
				// "none" or any unexpected value — skip merge safely.
				logger.Info("PR approved, skipping merge", "pr_number", prNum, "auto_merge", cfg.AutoMerge)
				applyLifecycleLabel(label.AwaitingHumanReview)
				outcome.Status = "ready-to-merge"
				outcome.Retries = attempt
				return outcome
			}

			if !shouldMerge {
				// Defensive: should not be reachable, but fail safe.
				logger.Warn("merge decision fell through unexpectedly, skipping merge", "auto_merge", cfg.AutoMerge)
				applyLifecycleLabel(label.AwaitingHumanReview)
				outcome.Status = "ready-to-merge"
				outcome.Retries = attempt
				return outcome
			}

			// Step 5.5: Wait for CI checks if configured.
			if cfg.WaitForChecks != nil {
				ciTimeout, _ := time.ParseDuration(cfg.WaitForChecks.Timeout) // already validated by config.Load
				ciFailures, err := WaitForChecks(ctx, cfg.Repo, prNum, cfg.WaitForChecks.Required, ciTimeout, logger)
				if err != nil {
					outcome.Status = "failed"
					outcome.Err = fmt.Errorf("waiting for CI checks: %w", err)
					return outcome
				}
				for ciAttempt := 0; len(ciFailures) > 0; ciAttempt++ {
					if ctx.Err() != nil {
						outcome.Status = "failed"
						outcome.Err = ctx.Err()
						return outcome
					}
					if ciAttempt >= cfg.Verify.MaxFixAttempts || prompts.VerifyFix == "" {
						var names []string
						for _, f := range ciFailures {
							names = append(names, f.Name)
						}
						outcome.Status = "failed"
						outcome.Err = fmt.Errorf("CI checks failed: %s", strings.Join(names, ", "))
						return outcome
					}

					logger.Info("CI check failed, triggering fix cycle",
						"issue_number", issue.Number,
						"attempt", ciAttempt+1,
						"max_attempts", cfg.Verify.MaxFixAttempts,
					)

					ciErrors := formatCheckFailures(ciFailures)
					fixResult, err := VerifyFix(ctx, issue, prNum, ciErrors, sessionID, cfg, prompts, authEnv, logger)
					if err != nil {
						outcome.Status = "failed"
						outcome.Err = fmt.Errorf("CI fix agent: %w", err)
						return outcome
					}
					if fixResult.TimedOut {
						outcome.Status = "failed"
						outcome.Err = fmt.Errorf("CI fix agent timed out")
						return outcome
					}
					sessionID = fixResult.SessionID

					if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
						outcome.Status = "failed"
						outcome.Err = driftErr
						return outcome
					}

					ciFailures, err = WaitForChecks(ctx, cfg.Repo, prNum, cfg.WaitForChecks.Required, ciTimeout, logger)
					if err != nil {
						outcome.Status = "failed"
						outcome.Err = fmt.Errorf("waiting for CI checks: %w", err)
						return outcome
					}
				}
			}
			// Merge the PR.
			if _, err := GuardRunner("gh", "pr", "merge", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--squash", "--delete-branch"); err != nil {
				outcome.Status = "failed"
				outcome.Err = fmt.Errorf("merging PR: %w", err)
				return outcome
			}
			// Remove all lifecycle labels after merge (best-effort). This cleans up
			// labels applied in a previous run or manually, not just the current run.
			for _, lbl := range label.All() {
				_ = github.RemoveIssueLabel(cfg.Repo, prNum, lbl)
			}
			outcome.Status = "implemented"
			outcome.Retries = attempt
			return outcome

		case "CHANGES_REQUESTED":
			retriesLeft := maxAttempts - attempt - 1
			if retriesLeft <= 0 {
				// Exhausted retries — write to top-level functional-review.json.
				if hook != nil {
					if err := hook.WriteReviewResult(issue.Number, "functional", fStep); err != nil {
						logger.Warn("failed to write functional review result", "error", err)
					}
				}
				break // exit loop, will label needs-human-review
			}

			// More retries available — write to retry dir so the pre-retry review is preserved.
			if hook != nil {
				if err := hook.WriteRetryFunctionalReviewResult(issue.Number, attempt, fStep); err != nil {
					logger.Warn("failed to write functional review result", "error", err)
				}
			}

			logger.Info("retrying implementation",
				"issue_number", issue.Number,
				"attempt", attempt+1,
				"retries_left", retriesLeft,
			)

			retryResult, err := Retry(ctx, issue, prNum, sessionID, "", cfg, prompts, authEnv, logger)
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
			if hook != nil {
				if err := hook.WriteRetryResult(issue.Number, attempt, ResultToStep(retryResult)); err != nil {
					logger.Warn("failed to write retry result", "error", err)
				}
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
			// No verdict found — write to top-level and treat as failure.
			if hook != nil {
				if err := hook.WriteReviewResult(issue.Number, "functional", fStep); err != nil {
					logger.Warn("failed to write functional review result", "error", err)
				}
			}
			outcome.Status = "failed"
			outcome.Err = fmt.Errorf("reviewer agent did not produce a verdict")
			return outcome
		}
	}

	// Exhausted retries.
	if err := LabelPR(cfg.Repo, prNum, "needs-human-review"); err != nil {
		logger.Warn("failed to label PR", "error", err)
	}
	applyLifecycleLabel(label.AwaitingHumanReview)
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

// topologicalSortModules returns module names sorted in dependency order
// (dependencies before dependents). Uses Kahn's algorithm with alphabetical
// tie-breaking for deterministic output. Returns an error if a cycle is
// detected (though config.validateModules should have already caught this).
func topologicalSortModules(modules map[string]config.Module) ([]string, error) {
	// In-degree = number of dependencies each module has.
	inDegree := make(map[string]int, len(modules))
	for name, mod := range modules {
		inDegree[name] = len(mod.DependsOn)
	}

	// Seed queue with zero-in-degree modules (no dependencies).
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	result := make([]string, 0, len(modules))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		result = append(result, name)

		// Reduce in-degree for every module that lists name in depends_on.
		var nowReady []string
		for other, mod := range modules {
			for _, dep := range mod.DependsOn {
				if dep == name {
					inDegree[other]--
					if inDegree[other] == 0 {
						nowReady = append(nowReady, other)
					}
					break
				}
			}
		}
		sort.Strings(nowReady)
		queue = append(queue, nowReady...)
	}

	if len(result) != len(modules) {
		return nil, fmt.Errorf("cycle detected in module dependencies")
	}
	return result, nil
}

// buildModuleVerifyChecks builds the ordered list of verify checks for a
// single module. Module-level commands take precedence; empty fields fall back
// to the root-level config commands. Each command is prefixed with
// "cd <modName> && " so it runs in the module's subdirectory.
// Order: generate → build → lint → test.
func buildModuleVerifyChecks(modName string, mod config.Module, cfg *config.Config) []Check {
	generate := mod.GenerateCommand
	if generate == "" {
		generate = cfg.GenerateCommand
	}
	build := mod.BuildCommand
	if build == "" {
		build = cfg.BuildCommand
	}
	lint := mod.LintCommand
	if lint == "" {
		lint = cfg.LintCommand
	}
	test := mod.TestCommand
	if test == "" {
		test = cfg.TestCommand
	}

	prefix := "cd " + modName + " && "

	var checks []Check
	if generate != "" {
		checks = append(checks, Check{Name: "generate", Command: prefix + generate})
	}
	if build != "" {
		checks = append(checks, Check{Name: "build", Command: prefix + build})
	}
	if lint != "" {
		checks = append(checks, Check{Name: "lint", Command: prefix + lint})
	}
	if test != "" {
		checks = append(checks, Check{Name: "test", Command: prefix + test})
	}
	return checks
}

// buildVerifyChecks constructs the ordered list of verify checks from non-empty
// config commands. Empty commands are omitted.
// Order: generate → build → lint → test.
func buildVerifyChecks(cfg *config.Config) []Check {
	var checks []Check
	if cfg.GenerateCommand != "" {
		checks = append(checks, Check{Name: "generate", Command: cfg.GenerateCommand})
	}
	if cfg.BuildCommand != "" {
		checks = append(checks, Check{Name: "build", Command: cfg.BuildCommand})
	}
	if cfg.LintCommand != "" {
		checks = append(checks, Check{Name: "lint", Command: cfg.LintCommand})
	}
	if cfg.TestCommand != "" {
		checks = append(checks, Check{Name: "test", Command: cfg.TestCommand})
	}
	return checks
}

// newHostRunner returns a CommandRunner that executes commands on the host
// via GuardRunner("sh", "-c", command). Exit codes are extracted from
// exec.ExitError when available; other errors return exit code 1.
func newHostRunner() CommandRunner {
	return func(ctx context.Context, command string) ([]byte, []byte, int, error) {
		out, err := GuardRunner("sh", "-c", command)
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return out, nil, exitErr.ExitCode(), nil
			}
			return out, nil, 1, err
		}
		return out, nil, 0, nil
	}
}

func trimOutput(b []byte) string {
	s := string(b)
	if idx := len(s) - 1; idx >= 0 && s[idx] == '\n' {
		return s[:idx]
	}
	return s
}

// computeReviewFlags runs quality analysis on an agent Result and returns any flags found.
// If checkTestExecution is false, CheckReviewTestExecution is skipped (used for the
// quality reviewer, which is exempt from that check).
func computeReviewFlags(result *Result, cfg *config.Config, checkTestExecution bool, hasScenarioSpec bool) []quality.Flag {
	var flags []quality.Flag

	if f := quality.CheckCostFloor(result.CostUSD, cfg.Quality.MinReviewCostUSD); f != nil {
		flags = append(flags, *f)
	}

	duration := result.FinishedAt.Sub(result.StartedAt)
	threshold := time.Duration(cfg.Quality.MinReviewDurationSeconds) * time.Second
	if f := quality.CheckDuration(duration, threshold); f != nil {
		flags = append(flags, *f)
	}

	flags = append(flags, quality.CheckToolTrace(result.ToolTrace, cfg.TestCommand, checkTestExecution)...)

	if checkTestExecution {
		flags = append(flags, quality.CheckReviewTestExecution(result.ToolTrace, cfg.ReviewDir, cfg.TestCommand, hasScenarioSpec)...)
	}

	return flags
}

// hasQualityFlag reports whether any flag in the slice has the given code.
func hasQualityFlag(flags []quality.Flag, code string) bool {
	for _, f := range flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

// runPostMortem performs best-effort post-mortem analysis on a failed agent result
// and writes the findings to the hook. It is a no-op when hook is nil, when the
// result has no container log, or when writing fails (errors are logged as warnings).
func runPostMortem(issueNum int, result *Result, hook RunDataHook, logger *slog.Logger) {
	if hook == nil || result == nil || result.ContainerLog == "" {
		return
	}

	if err := hook.WriteContainerLog(issueNum, result.ContainerLog); err != nil {
		logger.Warn("failed to write container log", "issue_number", issueNum, "error", err)
		// best-effort; continue to write analysis even if raw log write fails
	}

	analysis := AnalyzeFailure(result.ContainerLog, result.ExitCode)
	if err := hook.WriteFailureAnalysis(issueNum, analysis); err != nil {
		logger.Warn("failed to write failure analysis", "issue_number", issueNum, "error", err)
	}
}

// logAndRecordFlags logs each flag as a warning and returns a []rundata.Flag for storage.
func logAndRecordFlags(flags []quality.Flag, logger *slog.Logger, issueNum int) []rundata.Flag {
	if len(flags) == 0 {
		return nil
	}
	rdFlags := make([]rundata.Flag, len(flags))
	for i, f := range flags {
		logger.Warn("quality flag detected",
			"issue_number", issueNum,
			"code", f.Code,
			"message", f.Message,
		)
		rdFlags[i] = rundata.Flag{Code: f.Code, Message: f.Message}
	}
	return rdFlags
}
