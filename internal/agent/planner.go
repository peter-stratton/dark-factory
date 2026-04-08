package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// Plan runs the planner agent to produce a structured implementation brief for
// an issue. It reads the issue, recon brief, architecture doc, and conventions
// doc, then outputs an approach, key decisions, task breakdown, and risk flags.
// The planner has read-only permissions and does not modify code.
func Plan(ctx context.Context, issue github.Issue, integration bool, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger, reconBrief string) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug, integration)
	data.ReconBrief = reconBrief

	rendered, err := RenderPrompt(prompts.Planner, data)
	if err != nil {
		return nil, fmt.Errorf("rendering planner prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "planner", integration)
	if err != nil {
		return nil, err
	}

	logger.Info("starting planner agent",
		"issue_number", issue.Number,
		"issue_title", issue.Title,
	)

	return Run(ctx, opts, logger)
}
