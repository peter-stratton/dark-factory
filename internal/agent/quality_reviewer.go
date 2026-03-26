package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// QualityReviewResult holds the outcome of a quality reviewer agent run.
type QualityReviewResult struct {
	Verdict     string // "APPROVED", "CHANGES_REQUESTED", or ""
	AgentResult *Result
}

// QualityReview runs the quality reviewer agent for the given PR and returns the verdict.
// cycle is 0-indexed: 0 = first pass, 1 = first retry, etc.
func QualityReview(ctx context.Context, issue github.Issue, prNum int, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger, cycle int) (*QualityReviewResult, error) {
	maxAttempts := cfg.MaxRetries + 1

	data := newPromptData(issue, cfg, Slugify(issue.Title))
	data.PRNumber = prNum
	data.StrictnessDirective = strictnessDirective(cycle, maxAttempts, cfg.QualityStrictnessDecay)

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
		"cycle", cycle+1,
	)

	result, err := Run(ctx, opts, logger)
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

// strictnessDirective returns the strictness override text to append to the
// quality reviewer prompt for a given cycle. Returns empty string for the
// first pass (cycle==0), when decay is disabled, or when maxAttempts <= 1.
func strictnessDirective(cycle, maxAttempts int, decay bool) string {
	if !decay || maxAttempts <= 1 || cycle == 0 {
		return ""
	}
	if cycle == maxAttempts-1 {
		return "STRICTNESS OVERRIDE: This is the final quality review cycle. Only block on security vulnerabilities or data-safety issues. All other observations should be noted as comments but must NOT result in CHANGES_REQUESTED. Only print AGENT_RESULT=CHANGES_REQUESTED if you find a genuine security vulnerability."
	}
	return fmt.Sprintf("STRICTNESS OVERRIDE: This is quality review retry %d. Narrow your focus to security vulnerabilities and correctness issues only. Do not block on style, naming, minor performance, or code craft observations. Only print AGENT_RESULT=CHANGES_REQUESTED for security or correctness defects.", cycle+1)
}

// ParseQualityResult scans agent output for an AGENT_RESULT line and
// returns "APPROVED", "CHANGES_REQUESTED", or "" if not found.
func ParseQualityResult(stdout string) string {
	return ParseVerdict(stdout, "AGENT")
}
