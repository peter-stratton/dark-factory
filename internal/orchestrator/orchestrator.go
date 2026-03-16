package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/deps"
	"github.com/phs/dark-factory/internal/detect"
	"github.com/phs/dark-factory/internal/dialogue"
	gexec "github.com/phs/dark-factory/internal/exec"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/label"
	"github.com/phs/dark-factory/internal/lock"
	"github.com/phs/dark-factory/internal/notify"
	"github.com/phs/dark-factory/internal/progress"
	"github.com/phs/dark-factory/internal/punchlist"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/sandbox"
)

// Run is the main entry point for the orchestration loop.
// It fetches issues, resolves dependencies, and either prints the execution
// plan (dry-run) or iterates through processable issues. After each successful
// merge, dependencies are re-resolved so newly unblocked issues can be
// processed in the same run.
//
// force bypasses an existing run lock (useful to clear stale locks left by
// crashed instances). punchlistPath is the optional file path to write the
// punchlist to (always printed to stdout as well).
//
// logFactory is called to create the run-directory logger once the RunDataWriter
// has established its directory. Pass logging.NewLogger for text/pipe mode and
// logging.NewLoggerFileOnly for TUI mode (where the TUI owns stdout).
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, reporter progress.ProgressReporter, logFactory func(string) (*slog.Logger, error), milestone string, issue int, dryRun bool, force bool, punchlistPath string) error {
	// Preflight: fail fast if working tree is dirty (skip in dry-run mode).
	if !dryRun {
		if err := CheckWorkingTree(); err != nil {
			return err
		}
	}

	// Initialize notifiers from config. Construction failures are logged but
	// never abort the run — notifications are best-effort.
	notifiers, err := notify.NewFromConfig(cfg.Notify)
	if err != nil {
		logger.Warn("failed to initialize notifiers, continuing without notifications", "error", err)
		notifiers = nil
	}

	// Auto-detect project type when no runtime/commands are explicitly configured.
	detect.ApplyToConfig(cfg, ".", logger)

	// Step 1: Fetch open issues for the milestone.
	issues, err := github.FetchMilestoneIssues(cfg.Repo, milestone)
	if err != nil {
		return fmt.Errorf("fetching milestone issues: %w", err)
	}

	if len(issues) == 0 {
		logger.Info("no issues found", "milestone", milestone)
		fmt.Println("No issues found in milestone.")
		return nil
	}

	// Step 2: Fetch closed issues for dependency resolution.
	closedNumbers, err := github.FetchClosedIssueNumbers(cfg.Repo)
	if err != nil {
		return fmt.Errorf("fetching closed issues: %w", err)
	}
	closedSet := deps.ClosedSet(closedNumbers)

	// Step 3: Categorize issues into blocked and processable.
	processable, blocked := categorizeIssues(issues, closedSet)

	// Single-issue mode: filter processable to the requested issue.
	if issue != 0 {
		processable, err = filterSingleIssue(processable, blocked, issue)
		if err != nil {
			return err
		}
	}

	// Step 4: Switch logger to run directory before any orchestration logging.
	// Creating the RunDataWriter here (with actual issue numbers) ensures that
	// all subsequent log entries — including "starting orchestration" — are
	// written to <run-dir>/debug.log rather than the bootstrap temp directory.
	var writer *rundata.Writer
	if !dryRun {
		issueNums := issueNumbers(issues)
		var writerErr error
		writer, writerErr = newRunDataWriterFn(cfg.Repo, milestone, issueNums, cfg.BaseBranch, rundata.AutoMerge{Feature: string(cfg.AutoMerge.Feature), Rollup: string(cfg.AutoMerge.Rollup)})
		if writerErr != nil {
			logger.Warn("failed to create run data writer, run data will not be recorded", "error", writerErr)
		} else if runLogger, logErr := logFactory(writer.Dir()); logErr == nil {
			logger = runLogger
		} else {
			logger.Warn("failed to create run-dir logger, continuing with bootstrap logger", "error", logErr)
		}
	}

	logger.Info("starting orchestration",
		"repo", cfg.Repo,
		"milestone", milestone,
		"dry_run", dryRun,
	)
	logger.Info("fetched issues", "count", len(issues))
	logger.Info("dependency resolution complete",
		"total", len(issues),
		"blocked", len(blocked),
		"processable", len(processable),
	)

	// Notify the reporter that the run has started. The timestamp comes from
	// the run data directory name (e.g. "20260314-142305"); empty if no writer.
	var runTimestamp string
	if writer != nil {
		runTimestamp = filepath.Base(writer.Dir())
	}
	issueSummaries := make([]progress.IssueSummary, len(issues))
	for i, iss := range issues {
		issueSummaries[i] = progress.IssueSummary{Number: iss.Number, Title: iss.Title}
	}
	reporter.RunStarted(cfg.Repo, milestone, runTimestamp, cfg.BaseBranch,
		string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup), issueSummaries)

	// Step 5: Print or process.
	if dryRun {
		printDryRun(processable, blocked, len(issues))
		return nil
	}

	return processIssues(ctx, issues, closedSet, cfg, logger, reporter, writer, force, punchlistPath, milestone, notifiers)
}

// categorizeIssues splits issues into processable and blocked based on the
// closed set. Returns (processable, blocked).
func categorizeIssues(issues []github.Issue, closedSet map[int]bool) ([]github.Issue, []blockedIssue) {
	var blocked []blockedIssue
	var processable []github.Issue

	for _, issue := range issues {
		issueDeps := deps.ParseDeps(issue.Body)
		openDeps := openDependencies(issueDeps, closedSet)
		if len(openDeps) > 0 {
			blocked = append(blocked, blockedIssue{Issue: issue, BlockedBy: openDeps})
		} else {
			processable = append(processable, issue)
		}
	}
	return processable, blocked
}

// filterSingleIssue extracts a single issue from processable, returning an
// error if it's not found or is blocked.
func filterSingleIssue(processable []github.Issue, blocked []blockedIssue, issueNum int) ([]github.Issue, error) {
	for _, issue := range processable {
		if issue.Number == issueNum {
			return []github.Issue{issue}, nil
		}
	}
	for _, bi := range blocked {
		if bi.Issue.Number == issueNum {
			return nil, fmt.Errorf("issue #%d is blocked by %s", issueNum, formatIssueRefs(bi.BlockedBy))
		}
	}
	return nil, fmt.Errorf("issue #%d not found in milestone", issueNum)
}

// blockedIssue pairs an issue with its open (unresolved) dependency numbers.
type blockedIssue struct {
	Issue     github.Issue
	BlockedBy []int
}

// openDependencies returns the subset of dep numbers that are not in closedSet.
func openDependencies(depNumbers []int, closedSet map[int]bool) []int {
	var open []int
	for _, d := range depNumbers {
		if !closedSet[d] {
			open = append(open, d)
		}
	}
	return open
}

// printDryRun outputs the execution plan without taking any action.
func printDryRun(processable []github.Issue, blocked []blockedIssue, total int) {
	fmt.Println("=== Execution Plan (dry-run) ===")
	fmt.Println()

	if len(processable) > 0 {
		fmt.Println("Processable issues:")
		for _, issue := range processable {
			pri := issue.Priority
			if pri == "" {
				pri = "none"
			}
			fmt.Printf("  #%d %s [priority: %s]\n", issue.Number, issue.Title, pri)
		}
		fmt.Println()
	}

	if len(blocked) > 0 {
		fmt.Println("Blocked issues:")
		for _, bi := range blocked {
			fmt.Printf("  #%d %s (blocked by: %s)\n", bi.Issue.Number, bi.Issue.Title, formatIssueRefs(bi.BlockedBy))
		}
		fmt.Println()
	}

	printSummary(total, len(blocked), len(processable))
}

// processIssues runs processable issues through the agent loop with
// re-resolution after each merge. When an issue is successfully merged,
// the closed set is refreshed and dependencies re-resolved so that newly
// unblocked issues can be processed in the same run.
func processIssues(ctx context.Context, allIssues []github.Issue, closedSet map[int]bool, cfg *config.Config, logger *slog.Logger, reporter progress.ProgressReporter, writer *rundata.Writer, force bool, punchlistPath string, milestone string, notifiers []notify.Notifier) error {
	// Open stats DB early; nil on failure (errors logged, never fatal).
	statsDB := OpenStatsDB(logger)
	if statsDB != nil {
		defer func() {
			if err := statsDB.Close(); err != nil {
				logger.Warn("stats: failed to close database", "error", err)
			}
		}()
	}

	// Initial categorization.
	processable, blocked := categorizeIssues(allIssues, closedSet)

	// Write the dependency graph to run metadata so the dashboard can derive
	// pending vs blocked status for issues without a final outcome.
	if writer != nil {
		issueDeps := buildIssueDepsForRundata(allIssues, closedSet)
		if err := writer.WriteIssueDeps(issueDeps); err != nil {
			logger.Warn("failed to write issue deps to run metadata", "error", err)
		}

		issueTitles := make(map[string]string, len(allIssues))
		for _, iss := range allIssues {
			issueTitles[strconv.Itoa(iss.Number)] = iss.Title
		}
		if err := writer.WriteIssueTitles(issueTitles); err != nil {
			logger.Warn("failed to write issue titles to run metadata", "error", err)
		}
	}

	if len(processable) == 0 {
		reporter.AllBlocked(len(allIssues), len(blocked))
		return nil
	}

	// Collect auth tokens once at the start.
	authEnv, err := sandbox.CollectAuthEnv(logger, cfg.AuthPreference, cfg.RequiredEnv)
	if err != nil {
		return fmt.Errorf("collecting auth: %w", err)
	}

	// Load prompt templates once.
	prompts, err := agent.LoadPrompts(cfg)
	if err != nil {
		return fmt.Errorf("loading prompts: %w", err)
	}

	// Ensure all PR lifecycle labels exist in the repo at startup.
	for _, spec := range label.Specs {
		if err := github.EnsureLabel(cfg.Repo, spec.Name, spec.Color, spec.Description); err != nil {
			logger.Warn("failed to ensure lifecycle label", "label", spec.Name, "error", err)
		}
	}

	// Build Docker image once if using sandbox mode.
	if !cfg.NoSandbox {
		dc := sandbox.DockerConfigFromConfig(cfg.Docker, cfg.Runtime, cfg.SandboxEnv)
		tag, err := sandbox.BuildImage(ctx, dc, logger)
		if err != nil {
			return fmt.Errorf("building Docker image: %w", err)
		}
		cfg.Docker.Image = tag
	}

	// Wire up the RunDataHook from the pre-created writer (may be nil).
	var hook agent.RunDataHook
	if writer != nil {
		hook = writer
	}

	baseBranch := cfg.EffectiveBaseBranch()

	// Track run counters across all waves.
	var runStats struct {
		implemented      int
		readyToMerge     int
		needsHumanReview int
		failed           int
		blocked          int
		abortReason      string
	}

	// implementedIssues collects issues successfully merged into the base
	// branch, used to build the rollup PR body.
	var implementedIssues []github.Issue

	// Punchlist entries are enriched in the background as each issue completes.
	var plMu sync.Mutex
	var plEntries []punchlist.Entry
	var plWg sync.WaitGroup

	// Locking: create a locker and track all locked issue numbers across waves.
	locker := lock.New(cfg.Repo, logger)
	var allLockedNums []int
	defer func() {
		if len(allLockedNums) > 0 {
			if err := locker.Release(allLockedNums); err != nil {
				logger.Warn("failed to release run lock", "error", err)
			}
		}
	}()

	// Track which issues have been seen (processed or failed) to avoid reprocessing.
	seen := make(map[int]bool)
	wave := 0

	for {
		wave++

		// Filter out already-seen issues from the current processable batch.
		var batch []github.Issue
		for _, issue := range processable {
			if !seen[issue.Number] {
				batch = append(batch, issue)
			}
		}

		if len(batch) == 0 {
			if wave == 1 {
				reporter.AllBlocked(len(allIssues), len(blocked))
			}
			break
		}

		if wave > 1 {
			logger.Info("re-resolving dependencies",
				"wave", wave,
				"newly_unblocked", len(batch),
			)
			reporter.WaveStarted(wave, len(batch))
		}

		// Lock this wave's issues.
		batchNums := issueNumbers(batch)
		if wave == 1 {
			// First wave: full acquire (checks for existing locks).
			if err := locker.Acquire(batchNums, force); err != nil {
				return fmt.Errorf("acquiring run lock: %w", err)
			}
		} else {
			// Subsequent waves: we already hold the lock, just label new issues.
			for _, n := range batchNums {
				if err := github.AddIssueLabel(cfg.Repo, n, label.InProgress); err != nil {
					logger.Warn("failed to apply lock label to newly unblocked issue", "issue", n, "error", err)
				}
			}
		}
		allLockedNums = append(allLockedNums, batchNums...)

		// Process each issue in the batch.
		merged := false
		for _, issue := range batch {
			if ctx.Err() != nil {
				logger.Warn("context cancelled, stopping", "error", ctx.Err())
				goto done
			}

			seen[issue.Number] = true
			reporter.IssueStarted(issue.Number, issue.Title)
			outcome := processIssueFn(ctx, issue, cfg, prompts, authEnv, logger, hook, reporter)

			// Write dialogue after processing if we have a PR and a writer.
			if writer != nil && outcome.PRNumber > 0 {
				bodies, fetchErr := fetchPRCommentBodiesFn(cfg.Repo, outcome.PRNumber)
				if fetchErr != nil {
					logger.Warn("failed to fetch PR comment bodies for dialogue",
						"issue_number", issue.Number, "error", fetchErr)
				} else {
					entries := BuildDialogueEntries(bodies)
					if len(entries) > 0 {
						if err := writer.WriteDialogue(issue.Number, entries); err != nil {
							logger.Warn("failed to write dialogue",
								"issue_number", issue.Number, "error", err)
						}
					}
				}
			}

			// Compute per-issue cost from recorded step result files. Gracefully
			// degrades to 0.0 when the writer is nil or step files have no cost data.
			var issueCost float64
			if writer != nil {
				issueCost = rundata.IssueCostUSD(writer.IssueDir(issue.Number))
			}

			switch outcome.Status {
			case agent.StatusImplemented:
				runStats.implemented++
				implementedIssues = append(implementedIssues, issue)
				reporter.IssueCompleted(issue.Number, issue.Title, "implemented", outcome.PRNumber, outcome.Retries, "", issueCost)
				if err := PullAfterMerge(baseBranch, logger); err != nil {
					logger.Warn("stopping loop: could not sync local repo after merge", "error", err)
					runStats.abortReason = fmt.Sprintf("could not sync after merge: %v", err)
					goto done
				}
				merged = true
			case agent.StatusReadyToMerge:
				runStats.readyToMerge++
				reporter.IssueCompleted(issue.Number, issue.Title, "ready-to-merge", outcome.PRNumber, outcome.Retries, "", issueCost)
			case agent.StatusNeedsHumanReview:
				runStats.needsHumanReview++
				reporter.IssueCompleted(issue.Number, issue.Title, "needs-human-review", outcome.PRNumber, 0, "", issueCost)
			default:
				runStats.failed++
				errMsg := ""
				if outcome.Err != nil {
					errMsg = outcome.Err.Error()
				}
				reporter.IssueCompleted(issue.Number, issue.Title, "failed", 0, 0, errMsg, issueCost)
			}

			logger.Info("issue outcome",
				"issue_number", outcome.IssueNumber,
				"status", outcome.Status,
				"pr_number", outcome.PRNumber,
				"retries", outcome.Retries,
				"error", outcome.Err,
			)

			if outcome.PRNumber > 0 {
				// Enrich punchlist in the background so it's available as
				// soon as possible without blocking the next issue.
				plWg.Add(1)
				go func(iss github.Issue, oc agent.IssueOutcome) {
					defer plWg.Done()
					entry := buildPunchlistEntry(ctx, iss, oc, cfg, prompts, authEnv, logger)
					plMu.Lock()
					plEntries = append(plEntries, entry)
					plMu.Unlock()

					// Write run data immediately.
					if writer != nil {
						status := punchlistEnrichmentStatus(prompts, entry.AcceptanceTests)
						plData := rundata.PunchlistData{
							VerificationSteps: entry.ExtractVerificationSteps(),
							ScenarioCases:     entry.ExtractScenarioCases(),
							AcceptanceTests:   entry.AcceptanceTests,
							ChangedFiles:      entry.ChangedFiles,
							EnrichmentStatus:  status,
						}
						if err := writer.WritePunchlist(entry.IssueNumber, plData); err != nil {
							logger.Warn("failed to write punchlist data",
								"issue_number", entry.IssueNumber, "error", err)
						}
					}

					// Print this entry's punchlist to stdout immediately.
					text := punchlist.Generate([]punchlist.Entry{entry})
					if text != "" {
						reporter.PunchlistText(text)
					}
					logger.Info("punchlist acceptance tests generated",
						"issue_number", iss.Number,
						"count", len(entry.AcceptanceTests),
					)
				}(issue, outcome)
			}

			// After a merge, break out of the inner loop to re-resolve.
			if outcome.Status == "implemented" {
				break
			}
		}

		if !merged {
			// No merges in this wave — no point re-resolving.
			break
		}

		// Re-fetch closed issues and re-categorize for the next wave.
		closedNumbers, err := github.FetchClosedIssueNumbers(cfg.Repo)
		if err != nil {
			logger.Warn("failed to re-fetch closed issues, stopping re-resolution", "error", err)
			break
		}
		closedSet = deps.ClosedSet(closedNumbers)
		processable, blocked = categorizeIssues(allIssues, closedSet)
	}

done:
	runStats.blocked = len(blocked)
	reporter.RunFinished(runStats.implemented, runStats.readyToMerge, runStats.needsHumanReview, runStats.failed, runStats.blocked)

	// Rollup PR: create (and optionally merge) a PR from the base branch back
	// to the default branch when the run completed cleanly, used a non-default
	// base branch, and at least one feature PR was merged.
	defaultBranch := cfg.EffectiveDefaultBranch(cfg.Repo)
	var rollupPRNumber int
	var rollupPRURL string
	if runStats.abortReason == "" &&
		cfg.AutoMerge.Rollup != config.RollupNone &&
		cfg.BaseBranch != "" && cfg.BaseBranch != defaultBranch &&
		runStats.implemented > 0 {
		prNum, prURL, err := handleRollupPR(ctx, cfg, implementedIssues, defaultBranch, logger, reporter, writer, prompts, authEnv)
		if err != nil {
			logger.Warn("rollup PR handling failed", "error", err)
			fmt.Printf("Rollup PR warning: %v\n", err)
		} else {
			rollupPRNumber = prNum
			rollupPRURL = prURL
		}
	}

	// Fire abort notification if the run stopped early.
	if runStats.abortReason != "" {
		notify.Fire(ctx, notifiers, notify.Event{
			Type:    "abort",
			Repo:    cfg.Repo,
			Message: fmt.Sprintf("Run aborted: %s", runStats.abortReason),
		}, logger)
	}

	// Finalize run data.
	if writer != nil {
		summary := rundata.RunSummary{
			Total:          runStats.implemented + runStats.readyToMerge + runStats.needsHumanReview + runStats.failed,
			Implemented:    runStats.implemented,
			Failed:         runStats.failed,
			AbortReason:    runStats.abortReason,
			RollupPRNumber: rollupPRNumber,
			RollupPRURL:    rollupPRURL,
		}
		if err := writer.FinalizeRun(summary); err != nil {
			logger.Warn("failed to finalize run data", "error", err)
		}
		WriteRunStats(ctx, statsDB, cfg, writer, summary, logger)
	}

	// Fire run_complete notification after finalization — only when not aborted.
	if runStats.abortReason == "" {
		notify.Fire(ctx, notifiers, notify.Event{
			Type: "run_complete",
			Repo: cfg.Repo,
			Message: fmt.Sprintf("%d implemented, %d failed, %d blocked",
				runStats.implemented, runStats.failed, runStats.blocked),
		}, logger)
	}

	// Wait for all background punchlist enrichments to finish.
	plWg.Wait()

	// Write consolidated punchlist to file if a path was given.
	if len(plEntries) > 0 && punchlistPath != "" {
		text := punchlist.Generate(plEntries)
		if err := punchlist.Write(text, punchlistPath); err != nil {
			logger.Warn("failed to write punchlist", "error", err)
		}
	}

	return nil
}

// printSummary outputs the final count line.
func printSummary(total, blocked, processable int) {
	fmt.Printf("Summary: %d total, %d blocked, %d processable\n", total, blocked, processable)
}

// processIssueFn is the function called to process each issue.
// Replaceable for testing.
var processIssueFn = agent.ProcessIssue

// fetchPRCommentBodiesFn fetches PR comment bodies for dialogue extraction.
// Replaceable for testing.
var fetchPRCommentBodiesFn = github.FetchPRCommentBodies

// BuildDialogueEntries parses PR comment bodies in their original chronological
// order and returns a slice of DialogueEntry values suitable for persisting.
// Round numbers within each role are sequential (implementer round 1, 2, …)
// regardless of interleaving with other roles.
func BuildDialogueEntries(bodies []string) []rundata.DialogueEntry {
	parsed := dialogue.ParseCommentsInOrder(bodies)
	entries := make([]rundata.DialogueEntry, len(parsed))
	for i, e := range parsed {
		entries[i] = rundata.DialogueEntry{
			Role:    e.Role,
			Round:   e.Round,
			Body:    e.Body,
			Outcome: dialogueOutcome(e),
		}
	}
	return entries
}

// dialogueOutcome returns "accepted" or "changes_requested" for reviewer
// entries based on whether the structured comment contains requested changes.
// Returns "" for non-reviewer roles.
func dialogueOutcome(e dialogue.Entry) string {
	switch e.Role {
	case "quality_reviewer":
		if n := dialogue.ParseQualityReviewNotes(e.Body); n != nil && n.ChangesRequested != "" {
			return "changes_requested"
		}
		return "accepted"
	case "reviewer":
		if n := dialogue.ParseReviewNotes(e.Body); n != nil && n.ChangesRequested != "" {
			return "changes_requested"
		}
		return "accepted"
	default:
		return ""
	}
}

// newRunDataWriterFn creates a new RunDataWriter. Replaceable for testing.
var newRunDataWriterFn = func(repo, milestone string, issueNumbers []int, baseBranch string, autoMerge rundata.AutoMerge) (*rundata.Writer, error) {
	return rundata.New(repo, milestone, issueNumbers, baseBranch, autoMerge)
}

// upsertRollupPRFn creates or updates the rollup PR and returns its number and URL.
// Replaceable for testing.
var upsertRollupPRFn = github.UpsertRollupPR

// mergeRollupPRFn merges the rollup PR by number.
// Replaceable for testing.
var mergeRollupPRFn = github.MergeRollupPR

// runRollupVerifyFn runs the verify pipeline on the rollup branch.
// Replaceable for testing.
var runRollupVerifyFn = agent.RunRollupVerify

// handleRollupPR creates (and optionally merges) a PR from cfg.BaseBranch
// into defaultBranch. It is called when rollup is "manual" or "auto" and at
// least one issue was implemented into the base branch during the run.
// Returns the PR number, URL, and any error.
func handleRollupPR(ctx context.Context, cfg *config.Config, issues []github.Issue, defaultBranch string, logger *slog.Logger, reporter progress.ProgressReporter, writer *rundata.Writer, prompts *agent.Prompts, authEnv map[string]string) (int, string, error) {
	title := fmt.Sprintf("chore: merge %s into %s", cfg.BaseBranch, defaultBranch)
	body := buildRollupBody(issues)

	logger.Info("upserting rollup PR",
		"base_branch", cfg.BaseBranch,
		"default_branch", defaultBranch,
		"rollup_mode", cfg.AutoMerge.Rollup,
	)

	prNum, prURL, err := upsertRollupPRFn(cfg.Repo, cfg.BaseBranch, defaultBranch, title, body)
	if err != nil {
		return 0, "", fmt.Errorf("upserting rollup PR: %w", err)
	}

	logger.Info("rollup PR upserted", "pr_number", prNum, "pr_url", prURL)

	// Run the verify pipeline on the rollup branch before merge or human review.
	var writeResult func(rundata.VerifyStepResult) error
	if writer != nil {
		writeResult = writer.WriteRollupVerifyResult
	}
	verifyPassed, err := runRollupVerifyFn(ctx, prNum, cfg.BaseBranch, cfg, prompts, authEnv, logger, writeResult)
	if err != nil {
		return 0, "", fmt.Errorf("rollup verify: %w", err)
	}
	if !verifyPassed {
		logger.Warn("rollup verify failed after all fix attempts — leaving PR open for human intervention",
			"pr_number", prNum, "pr_url", prURL)
		reporter.RollupCreated(prNum, prURL, false)
		return prNum, prURL, nil
	}

	if cfg.AutoMerge.Rollup == config.RollupAuto {
		// Wait for CI checks before merging if configured.
		if cfg.WaitForChecks != nil {
			timeout, err := time.ParseDuration(cfg.WaitForChecks.Timeout)
			if err != nil {
				return 0, "", fmt.Errorf("parsing wait_for_checks timeout: %w", err)
			}
			failures, err := agent.WaitForChecks(ctx, cfg.Repo, prNum, cfg.WaitForChecks.Required, timeout, logger)
			if err != nil {
				return 0, "", fmt.Errorf("waiting for rollup PR checks: %w", err)
			}
			if len(failures) > 0 {
				names := make([]string, len(failures))
				for i, f := range failures {
					names[i] = f.Name
				}
				return 0, "", fmt.Errorf("rollup PR checks failed: %s", strings.Join(names, ", "))
			}
		}

		if err := mergeRollupPRFn(cfg.Repo, prNum); err != nil {
			return 0, "", fmt.Errorf("merging rollup PR: %w", err)
		}
		logger.Info("rollup PR merged", "pr_number", prNum)
		reporter.RollupCreated(prNum, prURL, true)
	} else {
		logger.Info("rollup PR left open for human review", "pr_number", prNum, "pr_url", prURL)
		reporter.RollupCreated(prNum, prURL, false)
	}

	return prNum, prURL, nil
}

// buildRollupBody composes the markdown body for the rollup PR from the list
// of issues that were implemented during the run.
func buildRollupBody(issues []github.Issue) string {
	var sb strings.Builder
	sb.WriteString("## Implemented issues\n\n")
	for _, iss := range issues {
		fmt.Fprintf(&sb, "- #%d %s\n", iss.Number, iss.Title)
	}
	return sb.String()
}

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner gexec.CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// CheckWorkingTree returns an error if the git working tree has uncommitted
// changes. The error message includes the list of dirty files from
// `git status --porcelain`.
func CheckWorkingTree() error {
	out, err := CommandRunner("git", "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("checking working tree: %w", err)
	}
	if dirty := strings.TrimSpace(string(out)); dirty != "" {
		return fmt.Errorf("working tree is dirty — commit or stash your changes before running:\n%s", dirty)
	}
	return nil
}

// PullAfterMerge syncs the local repo with the remote after a successful merge.
// If the working tree is dirty, it logs an actionable message and returns an
// error so the orchestration loop can stop gracefully.
func PullAfterMerge(branch string, logger *slog.Logger) error {
	_, err := CommandRunner("git", "pull", "--rebase", "origin", branch)
	if err == nil {
		logger.Info("pulled latest changes after merge")
		return nil
	}

	// Check if the repo is dirty.
	out, statusErr := CommandRunner("git", "status", "--porcelain")
	if statusErr != nil {
		logger.Warn("failed to pull after merge and could not check repo status",
			"pull_error", err,
			"status_error", statusErr,
		)
		return fmt.Errorf("pull after merge failed: %w", err)
	}

	if dirty := strings.TrimSpace(string(out)); dirty != "" {
		logger.Warn("local repo has uncommitted changes — commit your changes then pull",
			"branch", branch,
			"dirty_files", dirty,
		)
		return fmt.Errorf("local repo is dirty, cannot pull after merge")
	}

	logger.Warn("failed to pull after merge", "error", err)
	return fmt.Errorf("pull after merge failed: %w", err)
}

// EnsureBaseBranch checks whether the configured base branch exists on the
// remote. If it does not exist, it creates it from HEAD. This is a no-op when
// branch is empty (the repo default branch is used by the agent) or when the
// branch already exists.
func EnsureBaseBranch(branch string, logger *slog.Logger) error {
	if branch == "" {
		return nil
	}

	out, err := CommandRunner("git", "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return fmt.Errorf("checking remote branch %q: %w", branch, err)
	}

	if strings.TrimSpace(string(out)) != "" {
		logger.Info("base branch already exists on remote", "branch", branch)
		return nil
	}

	logger.Info("base branch does not exist on remote, creating from HEAD", "branch", branch)
	_, err = CommandRunner("git", "push", "origin", fmt.Sprintf("HEAD:refs/heads/%s", branch))
	if err != nil {
		return fmt.Errorf("creating remote branch %q: %w", branch, err)
	}
	logger.Info("created base branch on remote", "branch", branch)
	return nil
}

// buildPunchlistEntry creates and enriches a single punchlist entry for an issue.
func buildPunchlistEntry(ctx context.Context, issue github.Issue, outcome agent.IssueOutcome, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) punchlist.Entry {
	files, err := punchlist.FetchChangedFiles(cfg.Repo, outcome.PRNumber)
	if err != nil {
		logger.Warn("failed to fetch changed files for punchlist",
			"pr_number", outcome.PRNumber, "error", err)
	}
	spec, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issue.Number)
	if err != nil {
		logger.Warn("failed to read scenario spec for punchlist",
			"issue_number", issue.Number, "error", err)
	}
	entry := punchlist.Entry{
		IssueNumber:  issue.Number,
		IssueTitle:   issue.Title,
		IssueBody:    issue.Body,
		PRNumber:     outcome.PRNumber,
		Repo:         cfg.Repo,
		ScenarioSpec: spec,
		ChangedFiles: files,
	}

	// Enrich with LLM-generated acceptance tests (single entry).
	// EnrichPunchlistEntries modifies slice elements in place, so pass
	// a slice containing a pointer-like reference via index.
	entries := []punchlist.Entry{entry}
	agent.EnrichPunchlistEntries(ctx, entries, prompts, cfg, authEnv, logger)
	return entries[0]
}

// punchlistEnrichmentStatus derives the enrichment status string for a single
// punchlist entry. Returns "skipped" when no prompt was configured, "success"
// when tests were generated, and "failed" when the prompt was present but no
// tests could be parsed from the LLM output.
func punchlistEnrichmentStatus(prompts *agent.Prompts, acceptanceTests []string) string {
	if prompts.Punchlist == "" {
		return "skipped"
	}
	if acceptanceTests != nil {
		return "success"
	}
	return "failed"
}

// issueNumbers extracts the issue numbers from a slice of issues.
func issueNumbers(issues []github.Issue) []int {
	nums := make([]int, len(issues))
	for i, iss := range issues {
		nums[i] = iss.Number
	}
	return nums
}

// buildIssueDepsForRundata extracts the dependency graph for all issues so it
// can be persisted to run.json. Only issues with at least one open (intra-run)
// dependency are included. Deps that are already in closedSet are excluded
// because computeBlockedBy resolves blockers against the current run's issue
// list only — a closed dep is already resolved and must not be counted as a
// blocker.
func buildIssueDepsForRundata(issues []github.Issue, closedSet map[int]bool) []rundata.IssueDep {
	var issueDeps []rundata.IssueDep
	for _, issue := range issues {
		allDepNums := deps.ParseDeps(issue.Body)
		var openDepNums []int
		for _, n := range allDepNums {
			if !closedSet[n] {
				openDepNums = append(openDepNums, n)
			}
		}
		if len(openDepNums) > 0 {
			issueDeps = append(issueDeps, rundata.IssueDep{
				IssueNumber: issue.Number,
				DependsOn:   openDepNums,
			})
		}
	}
	return issueDeps
}

// formatIssueRefs formats a slice of issue numbers as "#1, #2, #3".
func formatIssueRefs(numbers []int) string {
	s := ""
	for i, n := range numbers {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("#%d", n)
	}
	return s
}
