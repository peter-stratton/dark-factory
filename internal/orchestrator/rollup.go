package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// RollupResult holds information about a created rollup PR.
type RollupResult struct {
	PRNumber int
	PRURL    string
}

// createRollupPRFn is the function called to create the rollup PR.
// Replaceable for testing.
var createRollupPRFn = createRollupPR

// createRollupPR creates a PR from the configured base branch into the default
// branch (main), and optionally merges it when rollup is "auto".
//
// Preconditions (must be checked by the caller):
//   - cfg.AutoMerge.Rollup is "manual" or "auto"
//   - cfg.BaseBranch is non-empty
//   - at least one issue was implemented
func createRollupPR(ctx context.Context, cfg *config.Config, implementedIssues []github.Issue, logger *slog.Logger) (*RollupResult, error) {
	baseBranch := cfg.BaseBranch
	targetBranch := "main"

	title := fmt.Sprintf("chore: merge %s into %s", baseBranch, targetBranch)
	body := buildRollupPRBody(baseBranch, targetBranch, implementedIssues)

	out, err := CommandRunner("gh", "pr", "create",
		"--repo", cfg.Repo,
		"--head", baseBranch,
		"--base", targetBranch,
		"--title", title,
		"--body", body,
	)
	if err != nil {
		return nil, fmt.Errorf("creating rollup PR: %w", err)
	}

	prURL := strings.TrimSpace(string(out))
	prNum := parsePRNumberFromURL(prURL)

	result := &RollupResult{PRNumber: prNum, PRURL: prURL}

	logger.Info("created rollup PR", "pr_number", prNum, "pr_url", prURL)
	fmt.Printf("Rollup PR created: %s\n", prURL)

	if cfg.AutoMerge.Rollup == "auto" {
		if cfg.WaitForChecks != nil {
			timeout, parseErr := time.ParseDuration(cfg.WaitForChecks.Timeout)
			if parseErr != nil {
				return result, fmt.Errorf("parsing wait_for_checks timeout: %w", parseErr)
			}
			failures, waitErr := agent.WaitForChecks(ctx, cfg.Repo, prNum, cfg.WaitForChecks.Required, timeout, logger)
			if waitErr != nil {
				return result, fmt.Errorf("waiting for rollup PR checks: %w", waitErr)
			}
			if len(failures) > 0 {
				names := make([]string, len(failures))
				for i, f := range failures {
					names[i] = f.Name
				}
				return result, fmt.Errorf("rollup PR CI checks failed: %s", strings.Join(names, ", "))
			}
		}

		_, mergeErr := CommandRunner("gh", "pr", "merge", fmt.Sprintf("%d", prNum),
			"--repo", cfg.Repo, "--squash", "--delete-branch")
		if mergeErr != nil {
			return result, fmt.Errorf("merging rollup PR: %w", mergeErr)
		}
		logger.Info("merged rollup PR", "pr_number", prNum, "pr_url", prURL)
		fmt.Printf("Rollup PR merged: %s\n", prURL)
	} else {
		logger.Info("rollup PR ready for human review", "pr_number", prNum, "pr_url", prURL)
		fmt.Printf("Rollup PR ready for review: %s\n", prURL)
	}

	return result, nil
}

// buildRollupPRBody constructs the PR body listing the implemented issues.
func buildRollupPRBody(baseBranch, targetBranch string, implementedIssues []github.Issue) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Rollup PR: merging `%s` into `%s` after completing %d implemented issue(s).\n",
		baseBranch, targetBranch, len(implementedIssues)))
	sb.WriteString("\n## Implemented Issues\n\n")
	for _, iss := range implementedIssues {
		sb.WriteString(fmt.Sprintf("- #%d %s\n", iss.Number, iss.Title))
	}
	return sb.String()
}

// parsePRNumberFromURL extracts the PR number from a GitHub PR URL such as
// https://github.com/owner/repo/pull/42. Returns 0 when the URL cannot be
// parsed.
func parsePRNumberFromURL(url string) int {
	url = strings.TrimRight(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return 0
	}
	n, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0
	}
	return n
}
