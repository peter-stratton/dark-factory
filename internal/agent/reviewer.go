package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// ReviewResult holds the outcome of a reviewer agent run.
type ReviewResult struct {
	Verdict     string // "APPROVED", "CHANGES_REQUESTED", or ""
	AgentResult *Result
}

// Review runs the reviewer agent for the given PR and returns the verdict.
func Review(ctx context.Context, issue github.Issue, prNum int, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger, hasSpec bool) (*ReviewResult, error) {
	data := newPromptData(issue, cfg, Slugify(issue.Title))
	data.PRNumber = prNum
	data.HasScenarioSpec = hasSpec

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

	result, err := Run(ctx, opts, logger)
	if err != nil {
		return nil, fmt.Errorf("running reviewer agent: %w", err)
	}

	// Use structured verdict from runner JSON first; fall back to parsing result text.
	verdict := result.Verdict
	if verdict == "" {
		verdict = ParseReviewResult(result.ResultText)
	}
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
