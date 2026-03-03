package orchestrator

import (
	"fmt"
	"log/slog"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/deps"
	"github.com/phs/dark-factory/internal/github"
)

// Run is the main entry point for the orchestration loop.
// It fetches issues, resolves dependencies, and either prints the execution
// plan (dry-run) or iterates through processable issues.
func Run(cfg *config.Config, logger *slog.Logger, dryRun bool) error {
	logger.Info("starting orchestration",
		"repo", cfg.Repo,
		"milestone", cfg.Milestone,
		"dry_run", dryRun,
	)

	// Step 1: Fetch open issues for the milestone.
	issues, err := github.FetchMilestoneIssues(cfg.Repo, cfg.Milestone)
	if err != nil {
		return fmt.Errorf("fetching milestone issues: %w", err)
	}

	if len(issues) == 0 {
		logger.Info("no issues found", "milestone", cfg.Milestone)
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

	logger.Info("dependency resolution complete",
		"total", len(issues),
		"blocked", len(blocked),
		"processable", len(processable),
	)

	// Step 4: Print or process.
	if dryRun {
		printDryRun(processable, blocked, len(issues))
	} else {
		processIssues(processable, blocked, len(issues), logger)
	}

	return nil
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

// processIssues iterates processable issues and logs placeholder messages.
func processIssues(processable []github.Issue, blocked []blockedIssue, total int, logger *slog.Logger) {
	if len(processable) == 0 {
		fmt.Println("All issues are blocked — nothing to process.")
		printSummary(total, len(blocked), 0)
		return
	}

	for _, issue := range processable {
		logger.Info("would process issue (agent execution not implemented yet)",
			"issue_number", issue.Number,
			"title", issue.Title,
			"priority", issue.Priority,
		)
		fmt.Printf("Processing #%d %s — not implemented yet (Phase 2)\n", issue.Number, issue.Title)
	}
	fmt.Println()

	printSummary(total, len(blocked), len(processable))
}

// printSummary outputs the final count line.
func printSummary(total, blocked, processable int) {
	fmt.Printf("Summary: %d total, %d blocked, %d processable\n", total, blocked, processable)
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
