package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// ReviewResult holds the outcome of a reviewer agent run.
type ReviewResult struct {
	Verdict     string // "APPROVED", "CHANGES_REQUESTED", or ""
	AgentResult *Result
}

// Review runs the reviewer agent for the given PR and returns the verdict.
func Review(ctx context.Context, issue github.Issue, prNum int, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*ReviewResult, error) {
	data := newPromptData(issue, cfg, Slugify(issue.Title))
	data.PRNumber = prNum

	rendered, err := RenderPrompt(prompts.Reviewer, data)
	if err != nil {
		return nil, fmt.Errorf("rendering reviewer prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "reviewer")
	if err != nil {
		return nil, err
	}

	logger.Info("starting reviewer agent",
		"issue_number", issue.Number,
		"pr_number", prNum,
	)

	result, err := Run(ctx, opts, cfg.NoSandbox, logger)
	if err != nil {
		return nil, fmt.Errorf("running reviewer agent: %w", err)
	}

	verdict := ParseReviewResult(result.Stdout)
	logger.Info("reviewer finished",
		"issue_number", issue.Number,
		"pr_number", prNum,
		"verdict", verdict,
	)

	return &ReviewResult{
		Verdict:     verdict,
		AgentResult: result,
	}, nil
}
