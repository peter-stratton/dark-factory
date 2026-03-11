package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
)

// implementedIssue records the number and title of an issue that was merged
// during a run, for inclusion in the rollup PR body.
type implementedIssue struct {
	Number int
	Title  string
}

// RollupResult holds the outcome of a rollup PR creation attempt.
type RollupResult struct {
	PRNumber int
	PRURL    string
	Merged   bool
}

// rollupDefaultBranch is the branch to which rollup PRs are targeted.
// "main" is standard for this project; not configurable per the issue spec.
const rollupDefaultBranch = "main"

// createRollupPRFn is the function used to create a rollup PR.
// Replaceable for testing.
var createRollupPRFn = createRollupPR

// createRollupPR creates a pull request from cfg.BaseBranch into
// rollupDefaultBranch, and optionally merges it (rollup == "auto").
//
// Preconditions (caller must verify before calling):
//   - cfg.AutoMerge.Rollup is "manual" or "auto"
//   - cfg.BaseBranch is non-empty
//   - len(issues) > 0
func createRollupPR(ctx context.Context, cfg *config.Config, issues []implementedIssue, logger *slog.Logger) (RollupResult, error) {
	if logger == nil {
		logger = slog.Default()
	}

	title := fmt.Sprintf("chore: merge %s into %s", cfg.BaseBranch, rollupDefaultBranch)
	body := buildRollupBody(cfg.BaseBranch, rollupDefaultBranch, issues)

	out, err := CommandRunner("gh", "pr", "create",
		"--repo", cfg.Repo,
		"--head", cfg.BaseBranch,
		"--base", rollupDefaultBranch,
		"--title", title,
		"--body", body,
		"--json", "number,url",
	)
	if err != nil {
		return RollupResult{}, fmt.Errorf("creating rollup PR: %w", err)
	}

	var created struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(out, &created); err != nil {
		return RollupResult{}, fmt.Errorf("parsing rollup PR response: %w", err)
	}

	result := RollupResult{
		PRNumber: created.Number,
		PRURL:    created.URL,
	}

	logger.Info("rollup PR created",
		"pr_number", result.PRNumber,
		"pr_url", result.PRURL,
		"head", cfg.BaseBranch,
		"base", rollupDefaultBranch,
	)

	if cfg.AutoMerge.Rollup != "auto" {
		return result, nil
	}

	// Wait for CI checks if configured before merging.
	if cfg.WaitForChecks != nil {
		ciTimeout, _ := time.ParseDuration(cfg.WaitForChecks.Timeout) // already validated
		failures, err := agent.WaitForChecks(ctx, cfg.Repo, result.PRNumber, cfg.WaitForChecks.Required, ciTimeout, logger)
		if err != nil {
			return result, fmt.Errorf("waiting for rollup PR CI checks: %w", err)
		}
		if len(failures) > 0 {
			names := make([]string, len(failures))
			for i, f := range failures {
				names[i] = f.Name
			}
			return result, fmt.Errorf("rollup PR CI checks failed: %s", strings.Join(names, ", "))
		}
	}

	// Merge the rollup PR.
	if _, err := CommandRunner("gh", "pr", "merge",
		fmt.Sprintf("%d", result.PRNumber),
		"--repo", cfg.Repo,
		"--squash",
		"--delete-branch",
	); err != nil {
		return result, fmt.Errorf("merging rollup PR #%d: %w", result.PRNumber, err)
	}

	result.Merged = true
	logger.Info("rollup PR merged",
		"pr_number", result.PRNumber,
		"pr_url", result.PRURL,
	)

	return result, nil
}

// buildRollupBody builds the PR description for a rollup PR.
func buildRollupBody(baseBranch, defaultBranch string, issues []implementedIssue) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Merges `%s` → `%s` after automated implementation run.\n\n", baseBranch, defaultBranch))
	sb.WriteString("## Implemented Issues\n\n")
	for _, iss := range issues {
		sb.WriteString(fmt.Sprintf("- #%d %s\n", iss.Number, iss.Title))
	}
	return sb.String()
}
