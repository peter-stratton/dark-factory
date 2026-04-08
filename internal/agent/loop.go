// Package agent runs the per-issue lifecycle: implement → review → merge.
//
// # Host-isolation invariant
//
// Code reachable from ProcessIssueWithMode (and therefore ProcessIssue and
// MergeApprovedPR) must not mutate the host repository's working tree, index,
// or HEAD. Every git operation in this package is one of:
//
//   - A read-only local query (e.g. `git rev-parse HEAD` to capture baseSHA
//     for drift detection).
//   - A remote operation against GitHub via `gh` (e.g. `gh pr merge`,
//     `gh pr update-branch`, `gh pr comment`).
//   - An operation that runs inside a sandbox container, against that
//     container's own clone of the repo at /workspace.
//
// This invariant is what makes bounded-concurrency wave dispatch
// (`internal/orchestrator`, Phase 14) safe without an explicit host-side
// mutex around per-issue work. Multiple ProcessIssueWithMode goroutines run
// in parallel and never touch each other's state because they don't touch
// the host tree at all. The orchestrator still serializes the actual merge
// step (MergeApprovedPR) post-wave, but only to avoid GitHub-side
// branch-protection contention, not to protect host state.
//
// If you add a `git checkout`, `git reset`, `git pull`, `git rebase`,
// `git merge`, `git commit`, or any other host-mutating shell call to a
// function reachable from ProcessIssueWithMode, the wave dispatcher will
// silently corrupt local state when workers > 1. The regression test
// TestProcessIssue_HostIsolation in loop_test.go locks this in by recording
// every GuardRunner command and asserting that none of them mutate the
// host. Read it before relaxing this rule.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/label"
	"github.com/peter-stratton/dark-factory/internal/progress"
	"github.com/peter-stratton/dark-factory/internal/punchlist"
	"github.com/peter-stratton/dark-factory/internal/quality"
	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/specdelta"
)

// OutcomeStatus represents the result of processing a single issue.
type OutcomeStatus string

const (
	StatusImplemented      OutcomeStatus = "implemented"
	StatusReadyToMerge     OutcomeStatus = "ready-to-merge"
	StatusNeedsHumanReview OutcomeStatus = "needs-human-review"
	StatusFailed           OutcomeStatus = "failed"
	// StatusApprovedReadyForMerge is a transient status used only when
	// ProcessIssueWithMode is called with deferMerge=true. It signals that
	// the reviewer approved the PR and the merge gate (auto_merge.feature)
	// said "merge it", so the orchestrator can perform the actual merge
	// serially after the wave's parallel agents have all finished. The
	// orchestrator must call MergeApprovedPR for any outcome carrying this
	// status; the status is rewritten in place to its terminal value
	// (StatusImplemented / StatusNeedsHumanReview / StatusFailed) before
	// being reported to the user.
	StatusApprovedReadyForMerge OutcomeStatus = "approved-ready-for-merge"
)

// generateTraceID returns a new UUID string for correlating all artifacts
// produced during a single ProcessIssue invocation.
var generateTraceID = func() string { return uuid.New().String() }

// IssueOutcome records the result of processing a single issue.
type IssueOutcome struct {
	IssueNumber  int
	Status       OutcomeStatus
	PRNumber     int
	Retries      int
	Err          error
	TraceID      string
	CloneSHA     string    // HEAD SHA captured inside the container after clone
	UsageLimited bool      // true when the implementer hit a Claude usage limit
	ResetsAt     time.Time // when the usage limit resets; zero if unknown
	// MergeArgs is non-nil only when Status == StatusApprovedReadyForMerge.
	// The orchestrator passes it to MergeApprovedPR to finish the merge
	// serially after the wave's parallel agents have all returned.
	MergeArgs *MergeApprovedPRArgs
}

// MergeApprovedPRArgs bundles the per-issue state captured by ProcessIssue at
// the moment the reviewer approved the PR and the merge gate said to proceed.
// It exists so MergeApprovedPR can be called either inline (deferMerge=false,
// the historical path) or by the orchestrator post-wave (deferMerge=true) with
// no closure dependencies — every field is a value, safe to ferry across a
// goroutine boundary.
type MergeApprovedPRArgs struct {
	PRNumber              int
	Branch                string
	BaseSHA               string
	SessionID             string
	FixCycles             int
	TraceID               string
	Attempt               int
	MaxAttempts           int
	CurrentLifecycleLabel string
	HasSpec               bool
}

// ProcessIssue runs the full per-issue lifecycle:
// implement → find PR → guard rails → review/retry loop → merge or label.
// hook is optional; if non-nil, it is called after each agent step to record
// run data. Hook errors are logged as warnings and do not abort processing.
//
// This is the historical entry point, equivalent to
// ProcessIssueWithMode(..., deferMerge=false). It performs the entire
// lifecycle including the post-approval merge phase inline. Use it from
// serial code paths (the daemon re-resolve loop, the `godark resolve`
// command, every existing test). Concurrent callers in the orchestrator's
// wave dispatcher use ProcessIssueWithMode with deferMerge=true so the
// merge phase can be hoisted out and run serially after the wave.
func ProcessIssue(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger, hook RunDataHook, reporter progress.ProgressReporter, integration bool) IssueOutcome {
	return ProcessIssueWithMode(ctx, issue, cfg, prompts, authEnv, logger, hook, reporter, integration, false)
}

// ProcessIssueWithMode is the deferMerge-aware variant of ProcessIssue. When
// deferMerge is false it behaves identically to ProcessIssue. When true it
// stops at the moment the reviewer approves the PR and the merge gate
// (cfg.AutoMerge.Feature) authorizes the merge, and returns
// IssueOutcome{Status: StatusApprovedReadyForMerge, MergeArgs: ...}. The
// caller (always the orchestrator's wave dispatcher) is then responsible
// for calling MergeApprovedPR with outcome.MergeArgs serially, after every
// goroutine in the wave has returned, so the actual `gh pr merge` calls and
// their pre-merge rebase contention happen one at a time.
//
// See the host-isolation invariant comment at the top of this file: every
// function reachable from this entry point must avoid mutating the host
// working tree. The orchestrator can rely on that invariant to run multiple
// ProcessIssueWithMode goroutines in parallel without an explicit lock.
func ProcessIssueWithMode(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger, hook RunDataHook, reporter progress.ProgressReporter, integration bool, deferMerge bool) IssueOutcome {
	outcome := IssueOutcome{IssueNumber: issue.Number}
	traceID := generateTraceID()
	outcome.TraceID = traceID

	// Write outcome data on every return path.
	defer func() {
		if hook != nil {
			o := rundata.Outcome{
				IssueNumber: outcome.IssueNumber,
				Title:       issue.Title,
				Description: issue.Body,
				Status:      string(outcome.Status),
				PRNumber:    outcome.PRNumber,
				CloneSHA:    outcome.CloneSHA,
			}
			o.TraceID = traceID
			if outcome.Err != nil {
				o.Error = outcome.Err.Error()
			}
			if err := hook.WriteOutcome(o); err != nil {
				logger.Warn("failed to write outcome", "error", err)
			}
			// Update status.json to reflect the final state so it doesn't
			// stay stale at "implementing" or "in_review".
			if outcome.Status != "" {
				if err := hook.WriteIssueStatus(outcome.IssueNumber, rundata.IssueStatus{
					Status: string(outcome.Status),
				}); err != nil {
					logger.Warn("failed to update final issue status", "error", err)
				}
			}
		}
	}()

	slug := Slugify(issue.Title)
	branch := BranchName(issue.Number, slug)

	logger.Info("processing issue", "issue_number", issue.Number, "title", issue.Title)

	// Record base SHA for drift detection.
	baseSHAOut, err := GuardRunner("git", "rev-parse", "HEAD")
	if err != nil {
		outcome.Status = StatusFailed
		outcome.Err = fmt.Errorf("getting base SHA: %w", err)
		return outcome
	}
	baseSHA := trimOutput(baseSHAOut)

	// Step 0: Generate scenario spec if missing.
	specGenerated := false
	if prompts.SpecGenerator != "" && !HasScenarioSpec(cfg.ScenarioDir, issue.Number) {
		logger.Info("no scenario spec found, generating", "issue_number", issue.Number)
		specResult, err := GenerateSpec(ctx, issue, integration, cfg, prompts, authEnv, logger)
		if specResult != nil {
			handleJudgeIntervention(issue.Number, "spec_generator", specResult, hook, reporter, logger)
		}
		var specWriteHook func(rundata.StepResult) error
		if hook != nil {
			specWriteHook = func(step rundata.StepResult) error {
				step.TraceID = traceID
				return hook.WriteSpecGeneratorResult(issue.Number, step)
			}
		}
		specText := handleNonBlockingResult(specResult, err, "spec-generator", logger, specWriteHook)
		if specText != "" {
			// The spec was committed to the remote branch inside the container.
			// The file isn't on the host, but the reviewer container will see it
			// when it checks out the PR branch. After merge and pull, the spec
			// lands on main permanently.
			specGenerated = true
			logger.Info("spec generated on remote branch", "branch", branch)
		}
	}

	// Step 1: Implement.
	if hook != nil {
		if err := hook.WriteIssueStatus(issue.Number, rundata.IssueStatus{Status: "implementing"}); err != nil {
			logger.Warn("failed to write issue status", "error", err)
		}
	}

	// Optional recon step: gather context before implementation.
	reconBrief := ""
	if prompts.Recon != "" {
		if reporter != nil {
			reporter.IssueStageChanged(issue.Number, "recon")
		}
		reconResult, reconErr := Recon(ctx, issue, integration, cfg, prompts, authEnv, logger)
		if reconResult != nil {
			if handleJudgeIntervention(issue.Number, "recon", reconResult, hook, reporter, logger) {
				runPostMortem(issue.Number, reconResult, hook, logger)
			}
		}
		var reconWriteHook func(rundata.StepResult) error
		if hook != nil {
			reconWriteHook = func(step rundata.StepResult) error {
				step.TraceID = traceID
				return hook.WriteReconResult(issue.Number, step)
			}
		}
		reconBrief = handleNonBlockingResult(reconResult, reconErr, "recon agent", logger, reconWriteHook)
	}

	// Optional planner step: produce a structured plan before implementation.
	plannerBrief := ""
	if prompts.Planner != "" {
		if reporter != nil {
			reporter.IssueStageChanged(issue.Number, "plan")
		}
		planResult, planErr := Plan(ctx, issue, integration, cfg, prompts, authEnv, logger, reconBrief)
		if planResult != nil {
			if handleJudgeIntervention(issue.Number, "plan", planResult, hook, reporter, logger) {
				runPostMortem(issue.Number, planResult, hook, logger)
			}
		}
		var planWriteHook func(rundata.StepResult) error
		if hook != nil {
			planWriteHook = func(step rundata.StepResult) error {
				step.TraceID = traceID
				return hook.WritePlannerResult(issue.Number, step)
			}
		}
		plannerBrief = handleNonBlockingResult(planResult, planErr, "planner agent", logger, planWriteHook)
	}

	if reporter != nil {
		reporter.IssueStageChanged(issue.Number, "implement")
	}
	implResult, err := Implement(ctx, issue, integration, cfg, prompts, authEnv, logger, reconBrief, plannerBrief)
	if err != nil {
		outcome.Status = StatusFailed
		outcome.Err = fmt.Errorf("implementer agent: %w", err)
		return outcome
	}
	if implResult.UsageLimited {
		outcome.UsageLimited = true
		outcome.ResetsAt = implResult.ResetsAt
		// Do NOT set StatusFailed — the orchestrator will retry.
		return outcome
	}
	if handleJudgeIntervention(issue.Number, "implement", implResult, hook, reporter, logger) {
		runPostMortem(issue.Number, implResult, hook, logger)
		outcome.Status = StatusFailed
		outcome.Err = fmt.Errorf("implementer agent killed by judge: %s", implResult.JudgeIntervention.Rule)
		return outcome
	}
	if implResult.TimedOut {
		runPostMortem(issue.Number, implResult, hook, logger)
		outcome.Status = StatusFailed
		outcome.Err = fmt.Errorf("implementer agent timed out")
		return outcome
	}
	outcome.CloneSHA = implResult.CloneSHA

	if hook != nil {
		implStep := ResultToStep(implResult)
		implStep.TraceID = traceID
		if err := hook.WriteImplementResult(issue.Number, implStep); err != nil {
			logger.Warn("failed to write implement result", "error", err)
		}
	}

	// Capture session ID so retries can resume the agent's context.
	sessionID := implResult.SessionID

	// Step 2: Find PR.
	prNum, err := FindPR(cfg.Repo, branch)
	if err != nil {
		outcome.Status = StatusFailed
		outcome.Err = fmt.Errorf("finding PR: %w", err)
		return outcome
	}
	if prNum == 0 {
		runPostMortem(issue.Number, implResult, hook, logger)
		outcome.Status = StatusFailed
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
		outcome.Status = StatusFailed
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
	if reporter != nil {
		reporter.IssueStageChanged(issue.Number, "verify")
	}
	if verifyErr := runVerifyPhase(ctx, issue, prNum, branch, baseSHA, cfg, prompts, authEnv, logger, hook, reporter, &sessionID, &fixCycles, traceID, integration); verifyErr != nil {
		outcome.Status = StatusFailed
		outcome.Err = verifyErr
		return outcome
	}

	// Step 4: Quality review gate (if prompt is configured).
	if prompts.QualityReviewer != "" {
		if reporter != nil {
			reporter.IssueStageChanged(issue.Number, "review")
		}
		passed, err := runQualityReviewCycle(ctx, issue, prNum, branch, baseSHA, cfg, prompts, authEnv, logger, hook, &sessionID, &fixCycles, reporter, traceID, integration)
		if err != nil {
			outcome.Status = StatusFailed
			outcome.Err = err
			return outcome
		}
		if !passed {
			qualityMaxAttempts := cfg.MaxRetries + 1
			if err := LabelPR(cfg.Repo, prNum, "needs-human-review"); err != nil {
				logger.Warn("failed to label PR", "error", err)
			}
			applyLifecycleLabel(label.AwaitingHumanReview)
			comment := fmt.Sprintf("Exhausted %d quality review/retry cycles. Labeling for human review.", qualityMaxAttempts)
			if _, err := GuardRunner("gh", "pr", "comment", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--body", comment); err != nil {
				logger.Warn("failed to comment on PR", "error", err)
			}
			outcome.Status = StatusNeedsHumanReview
			outcome.Retries = qualityMaxAttempts - 1
			return outcome
		}
	}

	// Step 5: Review/retry loop.
	if reporter != nil {
		reporter.IssueStageChanged(issue.Number, "review")
	}
	var mergeArgs *MergeApprovedPRArgs
	outcome.Status, _, outcome.Retries, outcome.Err = runFunctionalReviewCycle(
		ctx, issue, prNum, branch, baseSHA, cfg, prompts, authEnv, logger, hook, &sessionID, &fixCycles, hasSpec, reporter, traceID, integration, deferMerge, &mergeArgs,
	)
	if outcome.Status == StatusApprovedReadyForMerge {
		outcome.MergeArgs = mergeArgs
	}
	return outcome
}

// runVerifyPhase executes the verify step and optional verify-fix cycles.
// It handles both module-based (cfg.Modules != nil) and single-module verify paths.
// Returns nil on pass or non-blocking failure (logs a warning in that case).
// Returns a non-nil error on blocking failure, context cancellation, or agent error.
// sessionID and fixCycles are updated in-place when verify-fix agents run.
func runVerifyPhase(
	ctx context.Context,
	issue github.Issue,
	prNum int,
	branch string,
	baseSHA string,
	cfg *config.Config,
	prompts *Prompts,
	authEnv map[string]string,
	logger *slog.Logger,
	hook RunDataHook,
	reporter progress.ProgressReporter,
	sessionID *string,
	fixCycles *int,
	traceID string,
	integration bool,
) error {
	if cfg.Modules != nil {
		// Per-module verification in dependency order.
		sortedMods, err := topologicalSortModules(cfg.Modules)
		if err != nil {
			return fmt.Errorf("sorting modules for verify: %w", err)
		}

		verifyRunner := sandboxCommandRunner(cfg.Docker.Image, cfg.Repo, branch, authEnv, integration, logger)

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
			verifyResult := RunVerify(ctx, moduleChecks, verifyRunner, cfg.Truncation)
			if hook != nil {
				vsr := verifyToRundata(verifyResult, 0, false)
				vsr.TraceID = traceID
				if err := hook.WriteVerifyResult(issue.Number, vsr); err != nil {
					logger.Warn("failed to write verify result", "error", err)
				}
			}

			if !verifyResult.AllPassed && prompts.VerifyFix != "" && cfg.Verify.MaxFixAttempts > 0 {
				for fixAttempt := 0; fixAttempt < cfg.Verify.MaxFixAttempts; fixAttempt++ {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					*fixCycles++

					verifyErrors := fmt.Sprintf("Module: %s\n\n%s", modName, formatVerifyErrors(verifyResult))
					logger.Info("running verify-fix attempt",
						"issue_number", issue.Number,
						"module", modName,
						"attempt", fixAttempt+1,
						"max_attempts", cfg.Verify.MaxFixAttempts,
					)

					fixResult, err := VerifyFix(ctx, issue, prNum, verifyErrors, *sessionID, integration, cfg, prompts, authEnv, logger)
					if err != nil {
						return fmt.Errorf("verify-fix agent: %w", err)
					}
					if handleJudgeIntervention(issue.Number, "verify_fix", fixResult, hook, reporter, logger) {
						return fmt.Errorf("verify-fix agent killed by judge: %s", fixResult.JudgeIntervention.Rule)
					}
					if fixResult.TimedOut {
						return fmt.Errorf("verify-fix agent timed out")
					}
					*sessionID = fixResult.SessionID

					if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
						return driftErr
					}

					verifyResult = RunVerify(ctx, moduleChecks, verifyRunner, cfg.Truncation)
					if hook != nil {
						vsr := verifyToRundata(verifyResult, fixAttempt+1, true)
						vsr.TraceID = traceID
						if err := hook.WriteVerifyResult(issue.Number, vsr); err != nil {
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
				return fmt.Errorf("verify failed for module %s", failedModName)
			}
			logger.Warn("module verify failed (non-blocking), proceeding to review",
				"issue_number", issue.Number,
				"module", failedModName,
			)
		}
	} else if verifyChecks := buildVerifyChecks(cfg); len(verifyChecks) > 0 {
		verifyRunner := sandboxCommandRunner(cfg.Docker.Image, cfg.Repo, branch, authEnv, integration, logger)
		logger.Info("running verify step", "issue_number", issue.Number, "check_count", len(verifyChecks))
		verifyResult := RunVerify(ctx, verifyChecks, verifyRunner, cfg.Truncation)
		if hook != nil {
			vsr := verifyToRundata(verifyResult, 0, false)
			vsr.TraceID = traceID
			if err := hook.WriteVerifyResult(issue.Number, vsr); err != nil {
				logger.Warn("failed to write verify result", "error", err)
			}
		}

		if !verifyResult.AllPassed && prompts.VerifyFix != "" && cfg.Verify.MaxFixAttempts > 0 {
			for fixAttempt := 0; fixAttempt < cfg.Verify.MaxFixAttempts; fixAttempt++ {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				*fixCycles++

				verifyErrors := formatVerifyErrors(verifyResult)
				logger.Info("running verify-fix attempt",
					"issue_number", issue.Number,
					"attempt", fixAttempt+1,
					"max_attempts", cfg.Verify.MaxFixAttempts,
				)

				fixResult, err := VerifyFix(ctx, issue, prNum, verifyErrors, *sessionID, integration, cfg, prompts, authEnv, logger)
				if err != nil {
					return fmt.Errorf("verify-fix agent: %w", err)
				}
				if handleJudgeIntervention(issue.Number, "verify_fix", fixResult, hook, reporter, logger) {
					return fmt.Errorf("verify-fix agent killed by judge: %s", fixResult.JudgeIntervention.Rule)
				}
				if fixResult.TimedOut {
					return fmt.Errorf("verify-fix agent timed out")
				}
				*sessionID = fixResult.SessionID

				// Re-check drift after each fix attempt.
				if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
					return driftErr
				}

				// Re-run verify.
				verifyResult = RunVerify(ctx, verifyChecks, verifyRunner, cfg.Truncation)
				if hook != nil {
					vsr := verifyToRundata(verifyResult, fixAttempt+1, true)
					vsr.TraceID = traceID
					if err := hook.WriteVerifyResult(issue.Number, vsr); err != nil {
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
				return fmt.Errorf("verify failed: %s", strings.Join(failedNames, ", "))
			}
			logger.Warn("verify step failed (non-blocking), proceeding to review",
				"issue_number", issue.Number,
				"failed_checks", failedNames,
			)
		}
	}
	return nil
}

// handleNonBlockingResult handles the 3-way error/timeout/success dispatch
// common to non-blocking agent steps (spec-gen, recon). It logs warnings for
// failure and timeout, calls writeHook in all three cases (if non-nil), and
// returns the agent's ResultText on success or empty string otherwise.
// result may be nil when err is non-nil.
func handleNonBlockingResult(
	result *Result,
	err error,
	agentName string,
	logger *slog.Logger,
	writeHook func(rundata.StepResult) error,
) (resultText string) {
	if err != nil {
		logger.Warn(agentName+" failed, continuing", "error", err)
		if writeHook != nil {
			step := rundata.StepResult{Error: err.Error()}
			if writeErr := writeHook(step); writeErr != nil {
				logger.Warn("failed to write "+agentName+" result", "error", writeErr)
			}
		}
		return ""
	}
	if result.TimedOut {
		logger.Warn(agentName + " timed out, continuing")
		if writeHook != nil {
			step := ResultToStep(result)
			step.Error = "timed out"
			if writeErr := writeHook(step); writeErr != nil {
				logger.Warn("failed to write "+agentName+" result", "error", writeErr)
			}
		}
		return ""
	}
	if writeHook != nil {
		if writeErr := writeHook(ResultToStep(result)); writeErr != nil {
			logger.Warn("failed to write "+agentName+" result", "error", writeErr)
		}
	}
	return result.ResultText
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

// shouldHandoff returns true when the current attempt count has reached or
// exceeded the maximum number of resume retries, indicating the agent should
// include full handoff context for the next run.
func shouldHandoff(attempt int, maxResumeRetries int) bool {
	return attempt >= maxResumeRetries
}

// runQualityReviewCycle runs the quality review loop for a single issue.
// It calls QualityReview up to cfg.MaxRetries+1 times, invoking Retry and
// runVerifyPhase on each CHANGES_REQUESTED verdict. Returns (true, nil) when
// the quality reviewer approves, (false, nil) when all attempts are exhausted
// without approval, and (false, err) on any hard failure (agent error, drift,
// context cancellation). sessionID and fixCycles are updated in-place.
func runQualityReviewCycle(
	ctx context.Context,
	issue github.Issue,
	prNum int,
	branch string,
	baseSHA string,
	cfg *config.Config,
	prompts *Prompts,
	authEnv map[string]string,
	logger *slog.Logger,
	hook RunDataHook,
	sessionID *string,
	fixCycles *int,
	reporter progress.ProgressReporter,
	traceID string,
	integration bool,
) (bool, error) {
	qualityMaxAttempts := cfg.MaxRetries + 1
	for qAttempt := 0; qAttempt < qualityMaxAttempts; qAttempt++ {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		qResult, err := QualityReview(ctx, issue, prNum, integration, cfg, prompts, authEnv, logger, qAttempt)
		if err != nil {
			return false, fmt.Errorf("quality reviewer agent: %w", err)
		}
		// Quality reviewer is exempt from CheckReviewTestExecution.
		qFlags := computeReviewFlags(qResult.AgentResult, cfg, false, false, false)
		qRDFlags := logAndRecordFlags(qFlags, logger, issue.Number)
		if hook != nil {
			qStep := ResultToStep(qResult.AgentResult)
			qStep.Flags = qRDFlags
			qStep.TraceID = traceID
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
			return true, nil
		case "CHANGES_REQUESTED":
			retriesLeft := qualityMaxAttempts - qAttempt - 1
			if retriesLeft <= 0 {
				return false, nil // exhausted — caller handles labeling
			}

			logger.Info("quality review requested changes, retrying implementation",
				"issue_number", issue.Number,
				"attempt", qAttempt+1,
				"retries_left", retriesLeft,
			)

			var qualityHandoff string
			if shouldHandoff(qAttempt, cfg.MaxResumeRetries) {
				qualityHandoff = assembleHandoffContext(cfg.Repo, prNum, logger)
			}

			if reporter != nil {
				reporter.IssueStageChanged(issue.Number, "implement")
			}
			retryResult, err := Retry(ctx, issue, prNum, "", "", qualityHandoff, integration, cfg, prompts, authEnv, logger)
			if err != nil {
				return false, fmt.Errorf("retry agent (quality): %w", err)
			}
			if retryResult.TimedOut {
				return false, fmt.Errorf("retry agent (quality) timed out")
			}
			if hook != nil {
				qRetryStep := ResultToStep(retryResult)
				qRetryStep.TraceID = traceID
				if err := hook.WriteRetryResult(issue.Number, qAttempt, qRetryStep); err != nil {
					logger.Warn("failed to write retry result", "error", err)
				}
			}

			*sessionID = retryResult.SessionID

			if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
				return false, driftErr
			}

			// Re-run verify after quality-gate retry so that changes introduced
			// by the retry agent are caught before the next quality review cycle.
			if verifyErr := runVerifyPhase(ctx, issue, prNum, branch, baseSHA, cfg, prompts, authEnv, logger, hook, reporter, sessionID, fixCycles, traceID, integration); verifyErr != nil {
				return false, fmt.Errorf("verify after quality-gate retry: %w", verifyErr)
			}

		default:
			return false, fmt.Errorf("quality reviewer agent did not produce a verdict")
		}
	}
	return false, nil
}

// runFunctionalReviewCycle runs the functional review/retry loop for a single issue.
// It calls Review up to cfg.MaxRetries+1 times, invoking Retry on each
// CHANGES_REQUESTED verdict. On approval, it optionally merges the PR based on
// cfg.AutoMerge settings. Returns the final status, whether the PR was merged,
// the number of retries used, and any hard error. sessionID and fixCycles are
// updated in-place.
func runFunctionalReviewCycle(
	ctx context.Context,
	issue github.Issue,
	prNum int,
	branch string,
	baseSHA string,
	cfg *config.Config,
	prompts *Prompts,
	authEnv map[string]string,
	logger *slog.Logger,
	hook RunDataHook,
	sessionID *string,
	fixCycles *int,
	hasSpec bool,
	reporter progress.ProgressReporter,
	traceID string,
	integration bool,
	deferMerge bool,
	mergeArgsOut **MergeApprovedPRArgs,
) (status OutcomeStatus, prMerged bool, retries int, err error) {
	if hook != nil {
		if err := hook.WriteIssueStatus(issue.Number, rundata.IssueStatus{Status: "in_review"}); err != nil {
			logger.Warn("failed to write issue status", "error", err)
		}
	}

	// Re-declare the lifecycle label closure from scratch. At this call site,
	// the label state is "" (the quality gate only transitions labels on failure
	// paths that return early; the approval path that reaches here does not).
	currentLifecycleLabel := ""
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

	maxAttempts := cfg.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return StatusFailed, false, 0, ctx.Err()
		}

		reviewerPrompt := selectReviewerPrompt(cfg, prompts, attempt)
		usedSemiformal := reviewerPrompt == prompts.ReviewerSemiformal && prompts.ReviewerSemiformal != ""
		reviewResult, err := Review(ctx, issue, prNum, integration, cfg, reviewerPrompt, authEnv, logger, hasSpec)
		if err != nil {
			return StatusFailed, false, 0, fmt.Errorf("reviewer agent: %w", err)
		}
		// Functional reviewer is subject to all quality checks including CheckReviewTestExecution.
		fFlags := computeReviewFlags(reviewResult.AgentResult, cfg, true, hasSpec, usedSemiformal)
		fRDFlags := logAndRecordFlags(fFlags, logger, issue.Number)
		fStep := ResultToStep(reviewResult.AgentResult)
		fStep.Flags = fRDFlags
		fStep.TraceID = traceID

		// Pre-merge guard: if the reviewer approved without writing tests to the
		// review dir, re-run the reviewer. The reviewer is the agent responsible
		// for writing tests to ReviewDir — retrying the implementer would be
		// pointless since the implementer is forbidden from touching ReviewDir.
		if reviewResult.Verdict == "APPROVED" && hasQualityFlag(fFlags, "no_review_tests_written") {
			logger.Warn("functional review approved without writing tests — re-running reviewer",
				"issue_number", issue.Number,
				"attempt", attempt+1,
			)
			// Delete the stale review comment so the dialogue doesn't show a phantom round.
			if err := github.DeleteLastPRCommentWithHeader(cfg.Repo, prNum, "## Review Notes"); err != nil {
				logger.Warn("failed to delete stale review comment", "error", err)
			}
			continue
		}

		// Consistency guard: if the semiformal review verdict contradicts the
		// stated traces, re-run the reviewer.
		if hasQualityFlag(fFlags, "semiformal_inconsistency") {
			logger.Warn("semiformal review inconsistency detected — re-running reviewer",
				"issue_number", issue.Number,
				"attempt", attempt+1,
			)
			if err := github.DeleteLastPRCommentWithHeader(cfg.Repo, prNum, "## Review Notes"); err != nil {
				logger.Warn("failed to delete stale review comment", "error", err)
			}
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
			switch cfg.AutoMerge.Feature {
			case config.MergeAll:
				logger.Info("PR approved, will merge (auto_merge.feature=all)", "pr_number", prNum)
				shouldMerge = true
			case config.MergeLowRisk:
				additions, deletions, fileCount, statsErr := github.FetchPRStats(cfg.Repo, prNum)
				if statsErr != nil {
					logger.Warn("failed to fetch PR stats for risk classification, labeling for human review",
						"pr_number", prNum, "error", statsErr)
					applyLifecycleLabel(label.AwaitingHumanReview)
					return StatusReadyToMerge, false, attempt, nil
				}
				changedFiles, filesErr := github.FetchPRChangedFiles(cfg.Repo, prNum)
				if filesErr != nil {
					logger.Warn("failed to fetch PR changed files for risk classification, labeling for human review",
						"pr_number", prNum, "error", filesErr)
					applyLifecycleLabel(label.AwaitingHumanReview)
					return StatusReadyToMerge, false, attempt, nil
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
					FixCycles:      *fixCycles,
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
					return StatusReadyToMerge, false, attempt, nil
				}
				shouldMerge = true
			default:
				// "none" or any unexpected value — skip merge safely.
				logger.Info("PR approved, skipping merge", "pr_number", prNum, "auto_merge_feature", cfg.AutoMerge.Feature)
				applyLifecycleLabel(label.AwaitingHumanReview)
				return StatusReadyToMerge, false, attempt, nil
			}

			if !shouldMerge {
				// Defensive: should not be reachable, but fail safe.
				logger.Warn("merge decision fell through unexpectedly, skipping merge", "auto_merge_feature", cfg.AutoMerge.Feature)
				applyLifecycleLabel(label.AwaitingHumanReview)
				return StatusReadyToMerge, false, attempt, nil
			}

			// Steps 5.45 / 5.5 / 5.6 — pre-merge rebase, CI gate, merge,
			// post-merge cleanup. Hoisted into MergeApprovedPR so the
			// orchestrator can run them serially after a concurrent wave's
			// agents have all finished. In deferMerge mode we package the
			// state needed and return; in serial (legacy) mode we call
			// MergeApprovedPR inline so behavior is bit-identical for the
			// daemon re-resolve path and all existing tests.
			margs := MergeApprovedPRArgs{
				PRNumber:              prNum,
				Branch:                branch,
				BaseSHA:               baseSHA,
				SessionID:             *sessionID,
				FixCycles:             *fixCycles,
				TraceID:               traceID,
				Attempt:               attempt,
				MaxAttempts:           maxAttempts,
				CurrentLifecycleLabel: currentLifecycleLabel,
				HasSpec:               hasSpec,
			}
			if deferMerge {
				if mergeArgsOut != nil {
					*mergeArgsOut = &margs
				}
				return StatusApprovedReadyForMerge, false, attempt, nil
			}
			return MergeApprovedPR(ctx, issue, cfg, prompts, authEnv, logger, hook, reporter, integration, margs)

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

			var handoff string
			if shouldHandoff(attempt, cfg.MaxResumeRetries) {
				handoff = assembleHandoffContext(cfg.Repo, prNum, logger)
			}

			if reporter != nil {
				reporter.IssueStageChanged(issue.Number, "implement")
			}
			retryResult, err := Retry(ctx, issue, prNum, *sessionID, "", handoff, integration, cfg, prompts, authEnv, logger)
			if err != nil {
				return StatusFailed, false, 0, fmt.Errorf("retry agent: %w", err)
			}
			if handleJudgeIntervention(issue.Number, "retry", retryResult, hook, reporter, logger) {
				return StatusFailed, false, 0, fmt.Errorf("retry agent killed by judge: %s", retryResult.JudgeIntervention.Rule)
			}
			if retryResult.TimedOut {
				return StatusFailed, false, 0, fmt.Errorf("retry agent timed out")
			}
			if hook != nil {
				fRetryStep := ResultToStep(retryResult)
				fRetryStep.TraceID = traceID
				if err := hook.WriteRetryResult(issue.Number, attempt, fRetryStep); err != nil {
					logger.Warn("failed to write retry result", "error", err)
				}
			}

			// Update session ID so the next retry can resume this session's context.
			*sessionID = retryResult.SessionID

			// Re-check drift after retry.
			if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
				return StatusFailed, false, 0, driftErr
			}

		default:
			// No verdict found — write to top-level and treat as failure.
			if hook != nil {
				if err := hook.WriteReviewResult(issue.Number, "functional", fStep); err != nil {
					logger.Warn("failed to write functional review result", "error", err)
				}
			}
			return StatusFailed, false, 0, fmt.Errorf("reviewer agent did not produce a verdict")
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
	return StatusNeedsHumanReview, false, maxAttempts - 1, nil
}

// MergeApprovedPR runs the post-approval phase for a single PR: pre-merge
// rebase, optional CI check gate with fix-cycle retries, the actual squash
// merge, post-merge spec-delta capture, issue close, and lifecycle-label
// cleanup.
//
// It is invoked in two situations:
//
//  1. Inline from runFunctionalReviewCycle when deferMerge=false (the
//     historical serial path used by `godark resolve`, the daemon
//     re-resolve loop, and every existing test).
//  2. Serially by the orchestrator after a concurrent wave's parallel
//     ProcessIssueWithMode goroutines have all returned. In that mode,
//     ProcessIssue stops at the cut point and returns
//     StatusApprovedReadyForMerge plus a populated MergeApprovedPRArgs
//     so the orchestrator can ferry the per-issue state across the
//     goroutine boundary and call this function from a single thread.
//
// All inputs are values (not closures), so the function makes no
// assumptions about which goroutine it runs on. It must remain free of
// host-working-tree mutations: every git operation here is either a
// remote `gh` call against GitHub or runs inside a sandbox container.
// See the host-isolation invariant at the top of this file.
func MergeApprovedPR(
	ctx context.Context,
	issue github.Issue,
	cfg *config.Config,
	prompts *Prompts,
	authEnv map[string]string,
	logger *slog.Logger,
	hook RunDataHook,
	reporter progress.ProgressReporter,
	integration bool,
	args MergeApprovedPRArgs,
) (status OutcomeStatus, prMerged bool, retries int, err error) {
	prNum := args.PRNumber
	branch := args.Branch
	baseSHA := args.BaseSHA
	traceID := args.TraceID
	attempt := args.Attempt
	sessionID := args.SessionID
	fixCycles := args.FixCycles

	// Re-construct the lifecycle-label closure rooted at whatever state
	// ProcessIssue left it in. The orchestrator-driven path doesn't share
	// memory with ProcessIssue, so we have to thread the current label
	// through args and rebuild the closure here.
	currentLifecycleLabel := args.CurrentLifecycleLabel
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

	// Step 5.45: Pre-merge rebase check. Verifies the PR is still
	// mergeable after approval; if not, attempts automatic rebase or
	// triggers a conflict-fix cycle.
	if needsHR, rebaseErr := runPreMergeRebasePhase(
		ctx, issue, prNum, branch, baseSHA, cfg, prompts, authEnv, logger, hook, reporter, &sessionID, &fixCycles, traceID, integration,
	); rebaseErr != nil {
		return StatusFailed, false, 0, rebaseErr
	} else if needsHR {
		if err := LabelPR(cfg.Repo, prNum, "needs-human-review"); err != nil {
			logger.Warn("failed to label PR", "error", err)
		}
		applyLifecycleLabel(label.AwaitingHumanReview)
		comment := fmt.Sprintf("PR has unresolvable conflicts after %d rebase attempt(s). Labeling for human review.", cfg.MaxRebaseAttempts)
		if _, err := GuardRunner("gh", "pr", "comment", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--body", comment); err != nil {
			logger.Warn("failed to comment on PR", "error", err)
		}
		return StatusNeedsHumanReview, false, attempt, nil
	}

	// Step 5.5: Wait for CI checks if configured.
	if cfg.WaitForChecks != nil {
		ciTimeout, _ := time.ParseDuration(cfg.WaitForChecks.Timeout) // already validated by config.Load
		ciGrace := ParseGraceOrDefault(cfg.WaitForChecks.StartupGrace)
		ciFailures, err := WaitForChecks(ctx, cfg.Repo, prNum, cfg.WaitForChecks.Required, ciTimeout, ciGrace, logger)
		if err != nil {
			return StatusFailed, false, 0, fmt.Errorf("waiting for CI checks: %w", err)
		}
		for ciAttempt := 0; len(ciFailures) > 0; ciAttempt++ {
			if ctx.Err() != nil {
				return StatusFailed, false, 0, ctx.Err()
			}
			if ciAttempt >= cfg.Verify.MaxFixAttempts || prompts.VerifyFix == "" {
				var names []string
				for _, f := range ciFailures {
					names = append(names, f.Name)
				}
				if err := LabelPR(cfg.Repo, prNum, "needs-human-review"); err != nil {
					logger.Warn("failed to label PR", "error", err)
				}
				applyLifecycleLabel(label.AwaitingHumanReview)
				comment := fmt.Sprintf("CI checks failed after %d fix attempt(s): %s. Labeling for human review.",
					ciAttempt, strings.Join(names, ", "))
				if _, err := GuardRunner("gh", "pr", "comment", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--body", comment); err != nil {
					logger.Warn("failed to comment on PR", "error", err)
				}
				return StatusNeedsHumanReview, false, 0, fmt.Errorf("CI checks failed: %s", strings.Join(names, ", "))
			}

			logger.Info("CI check failed, triggering fix cycle",
				"issue_number", issue.Number,
				"attempt", ciAttempt+1,
				"max_attempts", cfg.Verify.MaxFixAttempts,
			)

			ciErrors := formatCheckFailures(ciFailures)
			fixResult, err := VerifyFix(ctx, issue, prNum, ciErrors, sessionID, integration, cfg, prompts, authEnv, logger)
			if err != nil {
				return StatusFailed, false, 0, fmt.Errorf("CI fix agent: %w", err)
			}
			if handleJudgeIntervention(issue.Number, "ci_fix", fixResult, hook, reporter, logger) {
				return StatusFailed, false, 0, fmt.Errorf("CI fix agent killed by judge: %s", fixResult.JudgeIntervention.Rule)
			}
			if fixResult.TimedOut {
				return StatusFailed, false, 0, fmt.Errorf("CI fix agent timed out")
			}
			sessionID = fixResult.SessionID

			if driftErr := checkDriftAndClose(baseSHA, cfg, prNum, logger); driftErr != nil {
				return StatusFailed, false, 0, driftErr
			}

			ciFailures, err = WaitForChecks(ctx, cfg.Repo, prNum, cfg.WaitForChecks.Required, ciTimeout, ciGrace, logger)
			if err != nil {
				return StatusFailed, false, 0, fmt.Errorf("waiting for CI checks: %w", err)
			}
		}
	}

	// Capture pre-merge spec content for delta computation.
	specBefore, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issue.Number)
	if err != nil {
		logger.Warn("failed to read pre-merge scenario spec", "error", err)
	}

	// Merge the PR. Retry with exponential backoff because GitHub
	// may need a moment to recalculate mergeable status after a
	// rebase or merge coordinator push.
	if err := mergeWithRetry(cfg.Repo, prNum, logger); err != nil {
		return StatusFailed, false, 0, err
	}

	// Capture post-merge spec content and compute delta.
	specAfter, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issue.Number)
	if err != nil {
		logger.Warn("failed to read post-merge scenario spec", "error", err)
	}
	delta := specdelta.Diff(specBefore, specAfter)
	logger.Info("spec delta computed", "empty", specdelta.IsEmpty(delta), "added", len(delta.AddedCases), "removed", len(delta.RemovedCases), "changed", len(delta.ChangedCases))
	if specdelta.IsEmpty(delta) && (specBefore != "" || specAfter != "") {
		logger.Warn("spec delta is empty despite spec file existing — local checkout may not reflect merged changes")
	}
	if !specdelta.IsEmpty(delta) {
		comment := "## Spec Delta\n\n" + specdelta.Format(delta)
		if _, err := GuardRunner("gh", "pr", "comment", fmt.Sprintf("%d", prNum), "--repo", cfg.Repo, "--body", comment); err != nil {
			logger.Warn("failed to comment spec delta on PR", "error", err)
		}
	}
	if hook != nil {
		deltaData := rundata.SpecDeltaData{
			Before:       specBefore,
			After:        specAfter,
			AddedCases:   delta.AddedCases,
			RemovedCases: delta.RemovedCases,
			ChangedCases: len(delta.ChangedCases),
			SetupChanged: delta.SetupChanged,
		}
		if err := hook.WriteSpecDelta(issue.Number, deltaData); err != nil {
			logger.Warn("failed to write spec delta", "error", err)
		}
	}

	// Explicitly close the issue after merge. GitHub's "Closes #N"
	// keyword only auto-closes on merge to the default branch, so
	// when the base branch differs we must close it ourselves.
	if err := github.CloseIssue(cfg.Repo, issue.Number); err != nil {
		logger.Warn("failed to close issue after merge", "issue", issue.Number, "error", err)
	}
	// Remove all lifecycle labels after merge (best-effort). This cleans up
	// labels applied in a previous run or manually, not just the current run.
	for _, lbl := range cfg.Labels().All() {
		_ = github.RemoveIssueLabel(cfg.Repo, prNum, lbl)
	}
	return StatusImplemented, true, attempt, nil
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
// to the root-level config commands. Commands are used exactly as written —
// they must include any directory changes needed (e.g. "cd app && flutter test").
// Order: format → generate → build → lint → test.
func buildModuleVerifyChecks(modName string, mod config.Module, cfg *config.Config) []Check {
	format := mod.FormatCommand
	if format == "" {
		format = cfg.FormatCommand
	}
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

	var checks []Check
	if format != "" {
		checks = append(checks, Check{Name: "format", Command: format})
	}
	if generate != "" {
		checks = append(checks, Check{Name: "generate", Command: generate})
	}
	if build != "" {
		checks = append(checks, Check{Name: "build", Command: build})
	}
	if lint != "" {
		checks = append(checks, Check{Name: "lint", Command: lint})
	}
	if test != "" {
		checks = append(checks, Check{Name: "test", Command: test})
	}
	return checks
}

// buildVerifyChecks constructs the ordered list of verify checks from non-empty
// config commands. Empty commands are omitted.
// Order: format → generate → build → lint → test.
func buildVerifyChecks(cfg *config.Config) []Check {
	var checks []Check
	if cfg.FormatCommand != "" {
		checks = append(checks, Check{Name: "format", Command: cfg.FormatCommand})
	}
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
// If isSemiformal is true, CheckSemiformalConsistency is called on the result output.
func computeReviewFlags(result *Result, cfg *config.Config, checkTestExecution bool, hasScenarioSpec bool, isSemiformal bool) []quality.Flag {
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

	if isSemiformal {
		if f := quality.CheckSemiformalConsistency(result.ResultText); f != nil {
			flags = append(flags, *f)
		}
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

// handleJudgeIntervention writes a judge intervention to run data and logs it.
// Returns true if the judge killed the container with no usable result (caller
// should treat as terminal for this attempt). Returns false if there was no
// intervention, or if the kill produced a usable result (caller should proceed
// normally).
func handleJudgeIntervention(issueNum int, step string, result *Result, hook RunDataHook, reporter progress.ProgressReporter, logger *slog.Logger) (terminalKill bool) {
	if result == nil || !result.JudgeKilled || result.JudgeIntervention == nil {
		return false
	}

	iv := result.JudgeIntervention
	logger.Warn("judge intervention",
		"issue_number", issueNum,
		"step", step,
		"rule", iv.Rule,
		"judgment", string(iv.Judgment),
		"detail", iv.Detail,
	)

	if reporter != nil {
		reporter.JudgeIntervention(issueNum, iv.Rule, string(iv.Judgment), iv.Detail, step)
	}

	if hook != nil {
		ji := rundata.JudgeIntervention{
			Rule:       iv.Rule,
			Judgment:   string(iv.Judgment),
			Detail:     iv.Detail,
			Counts:     iv.Counts,
			DetectedAt: iv.DetectedAt,
			Step:       step,
		}
		if err := hook.WriteJudgeIntervention(issueNum, ji); err != nil {
			logger.Warn("failed to write judge intervention", "issue_number", issueNum, "error", err)
		}
	}

	// Kill with usable result: the agent completed work before going idle.
	// Proceed normally — the kill was benign.
	if result.ResultText != "" {
		logger.Info("judge killed container but result is usable, proceeding",
			"issue_number", issueNum, "step", step)
		return false
	}

	// Kill with no result: the agent stalled before producing output.
	// Terminal for this attempt.
	logger.Warn("judge killed container with no usable result",
		"issue_number", issueNum, "step", step, "rule", iv.Rule)
	return true
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

// assembleHandoffContext fetches PR comment bodies and extracts
// ## Implementation Notes, ## Review Notes, and ## Quality Review Notes
// sections in chronological order. The result is a multi-section string
// suitable for injection into the implementer_retry prompt as HandoffContext.
// Returns an empty string on any error (best-effort) so that the caller can
// always proceed — worst case the retry runs without structured handoff.
func assembleHandoffContext(repo string, prNum int, logger *slog.Logger) string {
	out, err := GuardRunner("gh", "pr", "view",
		fmt.Sprintf("%d", prNum),
		"--repo", repo,
		"--json", "body,comments",
	)
	if err != nil {
		logger.Warn("assembleHandoffContext: gh pr view failed", "pr_number", prNum, "error", err)
		return ""
	}

	var pr struct {
		Body     string `json:"body"`
		Comments []struct {
			Body string `json:"body"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		logger.Warn("assembleHandoffContext: failed to parse pr view output", "pr_number", prNum, "error", err)
		return ""
	}

	// The headings we look for in comment bodies.
	headings := []string{
		"## Implementation Notes",
		"## Review Notes",
		"## Quality Review Notes",
	}

	// Collect all bodies in order: PR body first, then comments.
	allBodies := make([]string, 0, 1+len(pr.Comments))
	if pr.Body != "" {
		allBodies = append(allBodies, pr.Body)
	}
	for _, c := range pr.Comments {
		if c.Body != "" {
			allBodies = append(allBodies, c.Body)
		}
	}

	var sections []string
	for _, body := range allBodies {
		for _, heading := range headings {
			if strings.Contains(body, heading) {
				extracted := extractSection(body, heading)
				if extracted != "" {
					sections = append(sections, extracted)
				}
				break // only extract one heading per comment body
			}
		}
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n---\n\n")
}

// extractSection finds the named markdown section in body and returns
// its content (from the heading line through to the next same-level heading
// or the end of the body). Returns empty string when the heading is absent.
func extractSection(body, heading string) string {
	idx := strings.Index(body, heading)
	if idx < 0 {
		return ""
	}
	// Find the start of the next top-level or same-level heading (## or higher)
	// that appears after the current heading.
	rest := body[idx:]
	// Find next "## " occurrence after the first line.
	firstNL := strings.Index(rest, "\n")
	if firstNL < 0 {
		// Only one line — return the whole heading.
		return strings.TrimSpace(rest)
	}
	nextHeading := strings.Index(rest[firstNL:], "\n## ")
	var section string
	if nextHeading < 0 {
		section = rest
	} else {
		section = rest[:firstNL+nextHeading]
	}
	return strings.TrimSpace(section)
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

// mergeWithRetry attempts to merge a PR with exponential backoff. Before the
// first attempt it waits for GitHub to resolve the mergeable status from
// UNKNOWN, which avoids wasting retries on a status GitHub hasn't computed yet.
// Retries up to 4 times with delays of 2s, 4s, 8s, 16s (30s total max).
func mergeWithRetry(repo string, prNum int, logger *slog.Logger) error {
	// Wait for GitHub to resolve mergeable status before attempting merge.
	// This avoids burning retries when the status is simply not yet computed.
	status, waitErr := github.WaitForMergeable(repo, prNum, 30*time.Second)
	if waitErr != nil {
		logger.Warn("failed to poll mergeable status before merge, proceeding",
			"pr_number", prNum, "error", waitErr)
	} else if status == "CONFLICTING" {
		return fmt.Errorf("merging PR: PR has merge conflicts")
	} else if status == "UNKNOWN" {
		logger.Warn("mergeable status still UNKNOWN after 30s, attempting merge anyway",
			"pr_number", prNum)
	}

	const maxAttempts = 5
	delay := 2 * time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		out, err := GuardRunner("gh", "pr", "merge", fmt.Sprintf("%d", prNum), "--repo", repo, "--squash", "--delete-branch")
		if err == nil {
			return nil
		}
		if attempt == maxAttempts-1 {
			return fmt.Errorf("merging PR: %s: %w", strings.TrimSpace(string(out)), err)
		}
		logger.Warn("merge attempt failed, retrying",
			"pr_number", prNum,
			"attempt", attempt+1,
			"delay", delay,
			"error", err,
			"output", strings.TrimSpace(string(out)),
		)
		time.Sleep(delay)
		delay *= 2
	}
	return nil // unreachable
}

// runPreMergeRebasePhase checks whether the PR has conflicts before merging.
// If the PR is CONFLICTING, it attempts to resolve them via automatic rebase
// (gh pr update-branch) or by invoking the merge coordinator agent.
// After any successful resolution, the verify pipeline is re-run since branch
// content has changed. The cycle repeats up to cfg.MaxRebaseAttempts times.
//
// Returns (true, nil) when all attempts are exhausted and the caller should
// label the PR for human review. Returns (false, nil) when the PR is ready to
// merge. Returns (false, err) on hard errors (context cancel, verify failure).
//
// If cfg.MaxRebaseAttempts is 0 the function is a no-op and returns (false, nil).
func runPreMergeRebasePhase(
	ctx context.Context,
	issue github.Issue,
	prNum int,
	branch string,
	baseSHA string,
	cfg *config.Config,
	prompts *Prompts,
	authEnv map[string]string,
	logger *slog.Logger,
	hook RunDataHook,
	reporter progress.ProgressReporter,
	sessionID *string,
	fixCycles *int,
	traceID string,
	integration bool,
) (needsHumanReview bool, err error) {
	if cfg.MaxRebaseAttempts <= 0 {
		return false, nil
	}

	for attempt := 0; attempt < cfg.MaxRebaseAttempts; attempt++ {
		mergeable, checkErr := github.CheckMergeable(cfg.Repo, prNum)
		if checkErr != nil {
			// Best-effort: log and proceed; the merge call will fail naturally
			// if the PR is actually conflicting.
			logger.Warn("failed to check mergeable status, proceeding to merge",
				"pr_number", prNum, "error", checkErr)
			return false, nil
		}

		// GitHub may return UNKNOWN while computing mergeability (e.g. after
		// a push or rebase). Poll until it resolves so we don't attempt a
		// merge that will be rejected.
		if mergeable == "UNKNOWN" {
			logger.Info("mergeable status is UNKNOWN, waiting for GitHub to resolve",
				"pr_number", prNum)
			mergeable, checkErr = github.WaitForMergeable(cfg.Repo, prNum, 60*time.Second)
			if checkErr != nil {
				logger.Warn("failed to poll mergeable status, proceeding to merge",
					"pr_number", prNum, "error", checkErr)
				return false, nil
			}
			if mergeable == "UNKNOWN" {
				logger.Warn("mergeable status still UNKNOWN after polling, proceeding to merge",
					"pr_number", prNum)
			}
		}

		if mergeable != "CONFLICTING" {
			return false, nil
		}

		logger.Info("PR is conflicting, attempting automatic rebase",
			"pr_number", prNum,
			"attempt", attempt+1,
			"max_attempts", cfg.MaxRebaseAttempts,
		)

		updateErr := github.UpdateBranch(cfg.Repo, prNum)
		if updateErr != nil {
			// Automatic rebase failed — invoke the merge coordinator to
			// resolve conflicts independently of the implementer session.
			logger.Info("automatic rebase failed, invoking merge coordinator",
				"pr_number", prNum,
				"attempt", attempt+1,
				"error", updateErr,
			)
			conflictInfo := fmt.Sprintf(
				"PR #%d has merge conflicts with the base branch that could not be automatically resolved.\n\nError: %v\n\nPlease resolve the merge conflicts, push the changes, and ensure the branch is up to date with the base branch.",
				prNum, updateErr,
			)
			if reporter != nil {
				reporter.IssueStageChanged(issue.Number, "merge-coordinate")
			}
			mcResult, mcErr := MergeCoordinate(ctx, issue, prNum, conflictInfo, integration, cfg, prompts, authEnv, logger)
			writeMCResult := func(step rundata.StepResult) {
				if hook != nil {
					step.TraceID = traceID
					if writeErr := hook.WriteMergeCoordinatorResult(issue.Number, step); writeErr != nil {
						logger.Warn("failed to write merge coordinator result", "error", writeErr)
					}
				}
			}
			if mcErr != nil {
				writeMCResult(rundata.StepResult{Error: mcErr.Error()})
				logger.Warn("merge coordinator failed",
					"pr_number", prNum,
					"attempt", attempt+1,
					"error", mcErr,
				)
				continue
			}
			mcStep := ResultToStep(mcResult)
			writeMCResult(mcStep)
			if mcResult.ExitCode != 0 {
				logger.Warn("merge coordinator exited non-zero",
					"pr_number", prNum,
					"attempt", attempt+1,
					"exit_code", mcResult.ExitCode,
				)
				continue
			}
		}

		// Re-run verify since the branch content has changed (either by
		// automatic rebase or by the merge coordinator's conflict resolution).
		if verifyErr := runVerifyPhase(ctx, issue, prNum, branch, baseSHA, cfg, prompts, authEnv, logger, hook, reporter, sessionID, fixCycles, traceID, integration); verifyErr != nil {
			return false, fmt.Errorf("verify after rebase attempt %d: %w", attempt+1, verifyErr)
		}
	}

	// All attempts used; check one final time, waiting for UNKNOWN to resolve.
	mergeable, checkErr := github.WaitForMergeable(cfg.Repo, prNum, 60*time.Second)
	if checkErr != nil {
		logger.Warn("failed to check mergeable status after rebase attempts, proceeding",
			"pr_number", prNum, "error", checkErr)
		return false, nil
	}
	if mergeable == "CONFLICTING" {
		logger.Warn("PR still conflicting after all rebase attempts, labeling for human review",
			"pr_number", prNum,
			"max_attempts", cfg.MaxRebaseAttempts,
		)
		return true, nil
	}
	return false, nil
}
