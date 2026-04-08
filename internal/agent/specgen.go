package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// GenerateSpec runs the spec generator agent to create a scenario spec for
// the given issue. It follows the same pattern as Implement and Review.
func GenerateSpec(ctx context.Context, issue github.Issue, integration bool, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug, integration)
	// spec_generator uses narrower ScenarioDir wording ("existing files") to allow
	// creating new files inside ScenarioDir. Exclude the broad ScenarioDir bullet from
	// SharedRules to avoid contradicting that agent-specific rule.
	data.SharedRules = buildSharedRules(data.ProtectedPaths, "")

	rendered, err := RenderPrompt(prompts.SpecGenerator, data)
	if err != nil {
		return nil, fmt.Errorf("rendering spec_generator prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "spec_generator", integration)
	if err != nil {
		return nil, err
	}

	logger.Info("starting spec generator agent",
		"issue_number", issue.Number,
		"issue_title", issue.Title,
		"branch", BranchName(issue.Number, slug),
	)

	return Run(ctx, opts, logger)
}
