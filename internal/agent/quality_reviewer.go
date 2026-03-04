package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// QualityReviewResult holds the outcome of a quality reviewer agent run.
type QualityReviewResult struct {
	Verdict     string // "APPROVED", "CHANGES_REQUESTED", or ""
	AgentResult *Result
}

// QualityReview runs the quality reviewer agent for the given PR and returns the verdict.
func QualityReview(ctx context.Context, issue github.Issue, prNum int, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*QualityReviewResult, error) {
	data := newPromptData(issue, cfg, Slugify(issue.Title))
	data.PRNumber = prNum

	rendered, err := RenderPrompt(prompts.QualityReviewer, data)
	if err != nil {
		return nil, fmt.Errorf("rendering quality_reviewer prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "quality_reviewer")
	if err != nil {
		return nil, err
	}

	logger.Info("starting quality reviewer agent",
		"issue_number", issue.Number,
		"pr_number", prNum,
	)

	result, err := Run(ctx, opts, cfg.NoSandbox, logger)
	if err != nil {
		return nil, fmt.Errorf("running quality reviewer agent: %w", err)
	}

	// Use structured verdict from runner JSON first; fall back to parsing result text.
	verdict := result.Verdict
	if verdict == "" {
		verdict = ParseQualityResult(result.ResultText)
	}
	logger.Info("quality reviewer finished",
		"issue_number", issue.Number,
		"pr_number", prNum,
		"verdict", verdict,
	)

	return &QualityReviewResult{
		Verdict:     verdict,
		AgentResult: result,
	}, nil
}

// ParseQualityResult scans agent output for a QUALITY_RESULT line and
// returns "APPROVED", "CHANGES_REQUESTED", or "" if not found.
func ParseQualityResult(stdout string) string {
	upper := strings.ToUpper(stdout)
	for _, line := range strings.Split(upper, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "APPROVED") && strings.Contains(line, "QUALITY") && strings.Contains(line, "RESULT") {
			if strings.Contains(line, "CHANGES") {
				return "CHANGES_REQUESTED"
			}
			return "APPROVED"
		}
		if strings.Contains(line, "CHANGES_REQUESTED") && strings.Contains(line, "QUALITY") {
			return "CHANGES_REQUESTED"
		}
	}
	return ""
}
