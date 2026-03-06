package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/deps"
	"github.com/phs/dark-factory/internal/detect"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/lock"
	"github.com/phs/dark-factory/internal/logging"
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
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, milestone string, issue int, dryRun bool, force bool, punchlistPath string) error {
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
		writer, writerErr = newRunDataWriterFn(cfg.Repo, milestone, issueNums)
		if writerErr != nil {
			logger.Warn("failed to create run data writer, run data will not be recorded", "error", writerErr)
		} else if runLogger, logErr := logging.NewLogger(writer.Dir()); logErr == nil {
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

	// Step 5: Print or process.
	if dryRun {
		printDryRun(processable, blocked, len(issues))
		return nil
	}

	return processIssues(ctx, issues, closedSet, cfg, logger, writer, force, punchlistPath, milestone)
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
func processIssues(ctx context.Context, allIssues []github.Issue, closedSet map[int]bool, cfg *config.Config, logger *slog.Logger, writer *rundata.Writer, force bool, punchlistPath string, milestone string) error {
	// Initial categorization.
	processable, blocked := categorizeIssues(allIssues, closedSet)

	if len(processable) == 0 {
		fmt.Println("All issues are blocked — nothing to process.")
		printSummary(len(allIssues), len(blocked), 0)
		return nil
	}

	// Collect auth tokens once at the start.
	authEnv, err := sandbox.CollectAuthEnv(logger, cfg.AuthPreference)
	if err != nil {
		return fmt.Errorf("collecting auth: %w", err)
	}

	// Load prompt templates once.
	prompts, err := agent.LoadPrompts(cfg)
	if err != nil {
		return fmt.Errorf("loading prompts: %w", err)
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

	// Track stats across all waves.
	var stats struct {
		implemented      int
		readyToMerge     int
		needsHumanReview int
		failed           int
		blocked          int
	}

	type processedItem struct {
		issue   github.Issue
		outcome agent.IssueOutcome
	}
	var processed []processedItem

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
				fmt.Println("All issues are blocked — nothing to process.")
				printSummary(len(allIssues), len(blocked), 0)
			}
			break
		}

		if wave > 1 {
			logger.Info("re-resolving dependencies",
				"wave", wave,
				"newly_unblocked", len(batch),
			)
			fmt.Printf("\n--- Wave %d: %d newly unblocked issues ---\n", wave, len(batch))
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
				if err := github.AddIssueLabel(cfg.Repo, n, lock.LockLabel); err != nil {
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
			outcome := processIssueFn(ctx, issue, cfg, prompts, authEnv, logger, hook)

			switch outcome.Status {
			case "implemented":
				stats.implemented++
				fmt.Printf("  #%d %s — implemented (PR #%d, %d retries)\n", issue.Number, issue.Title, outcome.PRNumber, outcome.Retries)
				if err := PullAfterMerge(logger); err != nil {
					logger.Warn("stopping loop: could not sync local repo after merge", "error", err)
					goto done
				}
				merged = true
			case "ready-to-merge":
				stats.readyToMerge++
				fmt.Printf("  #%d %s — ready-to-merge (PR #%d, %d retries)\n", issue.Number, issue.Title, outcome.PRNumber, outcome.Retries)
			case "needs-human-review":
				stats.needsHumanReview++
				fmt.Printf("  #%d %s — needs human review (PR #%d)\n", issue.Number, issue.Title, outcome.PRNumber)
			default:
				stats.failed++
				errMsg := ""
				if outcome.Err != nil {
					errMsg = outcome.Err.Error()
				}
				fmt.Printf("  #%d %s — failed: %s\n", issue.Number, issue.Title, errMsg)
			}

			logger.Info("issue outcome",
				"issue_number", outcome.IssueNumber,
				"status", outcome.Status,
				"pr_number", outcome.PRNumber,
				"retries", outcome.Retries,
				"error", outcome.Err,
			)

			if outcome.PRNumber > 0 {
				processed = append(processed, processedItem{issue, outcome})
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
	stats.blocked = len(blocked)
	fmt.Println()
	fmt.Printf("Results: %d implemented, %d ready-to-merge, %d needs-human-review, %d failed, %d skipped (blocked)\n",
		stats.implemented, stats.readyToMerge, stats.needsHumanReview, stats.failed, stats.blocked)

	// Finalize run data.
	if writer != nil {
		summary := rundata.RunSummary{
			Total:       stats.implemented + stats.readyToMerge + stats.needsHumanReview + stats.failed,
			Implemented: stats.implemented,
			Failed:      stats.failed,
		}
		if err := writer.FinalizeRun(summary); err != nil {
			logger.Warn("failed to finalize run data", "error", err)
		}
	}

	if len(processed) > 0 {
		entries := make([]punchlist.Entry, 0, len(processed))
		for _, p := range processed {
			files, err := punchlist.FetchChangedFiles(cfg.Repo, p.outcome.PRNumber)
			if err != nil {
				logger.Warn("failed to fetch changed files for punchlist",
					"pr_number", p.outcome.PRNumber, "error", err)
			}
			spec, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, p.issue.Number)
			if err != nil {
				logger.Warn("failed to read scenario spec for punchlist",
					"issue_number", p.issue.Number, "error", err)
			}
			entries = append(entries, punchlist.Entry{
				IssueNumber:  p.issue.Number,
				IssueTitle:   p.issue.Title,
				IssueBody:    p.issue.Body,
				PRNumber:     p.outcome.PRNumber,
				Repo:         cfg.Repo,
				ScenarioSpec: spec,
				ChangedFiles: files,
			})
		}
		agent.EnrichPunchlistEntries(ctx, entries, prompts, cfg, authEnv, logger)

		if writer != nil {
			for _, e := range entries {
				plData := rundata.PunchlistData{
					VerificationSteps: e.ExtractVerificationSteps(),
					ScenarioCases:     e.ExtractScenarioCases(),
					AcceptanceTests:   e.AcceptanceTests,
					ChangedFiles:      e.ChangedFiles,
				}
				if err := writer.WritePunchlist(e.IssueNumber, plData); err != nil {
					logger.Warn("failed to write punchlist data",
						"issue_number", e.IssueNumber, "error", err)
				}
			}
		}

		text := punchlist.Generate(entries)
		fmt.Println()
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

// newRunDataWriterFn creates a new RunDataWriter. Replaceable for testing.
var newRunDataWriterFn = func(repo, milestone string, issueNumbers []int) (*rundata.Writer, error) {
	return rundata.New(repo, milestone, issueNumbers)
}

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// PullAfterMerge syncs the local repo with the remote after a successful merge.
// If the working tree is dirty, it logs an actionable message and returns an
// error so the orchestration loop can stop gracefully.
func PullAfterMerge(logger *slog.Logger) error {
	_, err := CommandRunner("git", "pull", "--rebase", "origin", "main")
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
		logger.Warn("local repo has uncommitted changes — commit your changes then run: git pull --rebase origin main",
			"dirty_files", dirty,
		)
		return fmt.Errorf("local repo is dirty, cannot pull after merge")
	}

	logger.Warn("failed to pull after merge", "error", err)
	return fmt.Errorf("pull after merge failed: %w", err)
}

// issueNumbers extracts the issue numbers from a slice of issues.
func issueNumbers(issues []github.Issue) []int {
	nums := make([]int, len(issues))
	for i, iss := range issues {
		nums[i] = iss.Number
	}
	return nums
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
