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
	"github.com/phs/dark-factory/internal/punchlist"
	"github.com/phs/dark-factory/internal/sandbox"
)

// Run is the main entry point for the orchestration loop.
// It fetches issues, resolves dependencies, and either prints the execution
// plan (dry-run) or iterates through processable issues.
// punchlistPath is the optional file path to write the punchlist to (stdout always receives it).
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, milestone string, issue int, dryRun bool, punchlistPath string) error {
	logger.Info("starting orchestration",
		"repo", cfg.Repo,
		"milestone", milestone,
		"dry_run", dryRun,
	)

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

	logger.Info("fetched issues", "count", len(issues))

	// Step 2: Fetch closed issues for dependency resolution.
	closedNumbers, err := github.FetchClosedIssueNumbers(cfg.Repo)
	if err != nil {
		return fmt.Errorf("fetching closed issues: %w", err)
	}
	closedSet := deps.ClosedSet(closedNumbers)

	// Step 3: Categorize issues into blocked and processable.
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

	// Single-issue mode: filter processable to the requested issue.
	if issue != 0 {
		processable, err = filterSingleIssue(processable, blocked, issue)
		if err != nil {
			return err
		}
	}

	logger.Info("dependency resolution complete",
		"total", len(issues),
		"blocked", len(blocked),
		"processable", len(processable),
	)

	// Step 4: Print or process.
	if dryRun {
		printDryRun(processable, blocked, len(issues))
		return nil
	}

	return processIssues(ctx, processable, blocked, len(issues), cfg, logger, punchlistPath)
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

// processIssues runs each processable issue through the agent loop and prints
// summary stats. punchlistPath is the optional file path to write the punchlist to.
func processIssues(ctx context.Context, processable []github.Issue, blocked []blockedIssue, total int, cfg *config.Config, logger *slog.Logger, punchlistPath string) error {
	if len(processable) == 0 {
		fmt.Println("All issues are blocked — nothing to process.")
		printSummary(total, len(blocked), 0)
		return nil
	}

	// Collect auth tokens once at the start.
	authEnv, err := sandbox.CollectAuthEnv(logger)
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

	// Process each issue.
	var stats struct {
		implemented      int
		readyToMerge     int
		needsHumanReview int
		failed           int
	}

	type processedItem struct {
		issue   github.Issue
		outcome agent.IssueOutcome
	}
	var processed []processedItem

	for _, issue := range processable {
		if ctx.Err() != nil {
			logger.Warn("context cancelled, stopping", "error", ctx.Err())
			break
		}

		outcome := agent.ProcessIssue(ctx, issue, cfg, prompts, authEnv, logger)

		switch outcome.Status {
		case "implemented":
			stats.implemented++
			fmt.Printf("  #%d %s — implemented (PR #%d, %d retries)\n", issue.Number, issue.Title, outcome.PRNumber, outcome.Retries)
			if err := PullAfterMerge(logger); err != nil {
				logger.Warn("stopping loop: could not sync local repo after merge", "error", err)
				break
			}
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
	}

	fmt.Println()
	fmt.Printf("Results: %d implemented, %d ready-to-merge, %d needs-human-review, %d failed, %d skipped (blocked)\n",
		stats.implemented, stats.readyToMerge, stats.needsHumanReview, stats.failed, len(blocked))

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
