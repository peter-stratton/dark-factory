package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/deps"
	"github.com/phs/dark-factory/internal/detect"
	"github.com/phs/dark-factory/internal/dialogue"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/label"
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
	// Preflight: fail fast if working tree is dirty (skip in dry-run mode).
	if !dryRun {
		if err := CheckWorkingTree(); err != nil {
			return err
		}
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

	// Write the dependency graph to run metadata so the dashboard can derive
	// pending vs blocked status for issues without a final outcome.
	if writer != nil {
		issueDeps := buildIssueDepsForRundata(allIssues, closedSet)
		if err := writer.WriteIssueDeps(issueDeps); err != nil {
			logger.Warn("failed to write issue deps to run metadata", "error", err)
		}
	}

	if len(processable) == 0 {
		fmt.Println("All issues are blocked — nothing to process.")
		printSummary(len(allIssues), len(blocked), 0)
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

	// Track stats across all waves.
	var stats struct {
		implemented      int
		readyToMerge     int
		needsHumanReview int
		failed           int
		blocked          int
		abortReason      string
	}

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
			outcome := processIssueFn(ctx, issue, cfg, prompts, authEnv, logger, hook)

			// Write dialogue after processing if we have a PR and a writer.
			if writer != nil && outcome.PRNumber > 0 {
				bodies, fetchErr := fetchPRCommentBodiesFn(cfg.Repo, outcome.PRNumber)
				if fetchErr != nil {
					logger.Warn("failed to fetch PR comment bodies for dialogue",
						"issue_number", issue.Number, "error", fetchErr)
				} else {
					implNotes, reviewNotes, qualityNotes := dialogue.ParseComments(bodies)
					entries := BuildDialogueEntries(implNotes, reviewNotes, qualityNotes)
					if len(entries) > 0 {
						if err := writer.WriteDialogue(issue.Number, entries); err != nil {
							logger.Warn("failed to write dialogue",
								"issue_number", issue.Number, "error", err)
						}
					}
				}
			}

			switch outcome.Status {
			case "implemented":
				stats.implemented++
				fmt.Printf("  #%d %s — implemented (PR #%d, %d retries)\n", issue.Number, issue.Title, outcome.PRNumber, outcome.Retries)
				if err := PullAfterMerge(logger); err != nil {
					logger.Warn("stopping loop: could not sync local repo after merge", "error", err)
					stats.abortReason = fmt.Sprintf("could not sync after merge: %v", err)
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
						fmt.Print(text)
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
			AbortReason: stats.abortReason,
		}
		if err := writer.FinalizeRun(summary); err != nil {
			logger.Warn("failed to finalize run data", "error", err)
		}
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

// BuildDialogueEntries interleaves implementation, quality review, and
// functional review notes by round, returning a slice of DialogueEntry
// values suitable for persisting.
func BuildDialogueEntries(implNotes []dialogue.ImplementationNotes, reviewNotes []dialogue.ReviewNotes, qualityNotes []dialogue.QualityReviewNotes) []rundata.DialogueEntry {
	maxRounds := len(implNotes)
	if len(reviewNotes) > maxRounds {
		maxRounds = len(reviewNotes)
	}
	if len(qualityNotes) > maxRounds {
		maxRounds = len(qualityNotes)
	}

	var entries []rundata.DialogueEntry
	for round := 1; round <= maxRounds; round++ {
		i := round - 1
		if i < len(implNotes) {
			entries = append(entries, rundata.DialogueEntry{
				Role:  "implementer",
				Round: round,
				Body:  implNotes[i].Raw,
			})
		}
		if i < len(qualityNotes) {
			entries = append(entries, rundata.DialogueEntry{
				Role:  "quality_reviewer",
				Round: round,
				Body:  qualityNotes[i].Raw,
			})
		}
		if i < len(reviewNotes) {
			entries = append(entries, rundata.DialogueEntry{
				Role:  "reviewer",
				Round: round,
				Body:  reviewNotes[i].Raw,
			})
		}
	}
	return entries
}

// newRunDataWriterFn creates a new RunDataWriter. Replaceable for testing.
var newRunDataWriterFn = func(repo, milestone string, issueNumbers []int) (*rundata.Writer, error) {
	return rundata.New(repo, milestone, issueNumbers)
}

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner = func(name string, args ...string) ([]byte, error) {
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
