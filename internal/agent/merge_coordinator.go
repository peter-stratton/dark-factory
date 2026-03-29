package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// MergeCoordinate runs the merge coordinator agent to resolve merge conflicts
// for a pull request. It injects conflict details into the prompt and delegates
// to the standard Run() pipeline.
func MergeCoordinate(ctx context.Context, issue github.Issue, prNum int, conflictInfo string, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug)
	data.PRNumber = prNum
	data.ConflictInfo = conflictInfo

	rendered, err := RenderPrompt(prompts.MergeCoordinator, data)
	if err != nil {
		return nil, fmt.Errorf("rendering merge coordinator prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "merge_coordinator")
	if err != nil {
		return nil, err
	}

	logger.Info("starting merge coordinator agent",
		"issue_number", issue.Number,
		"pr_number", prNum,
	)

	return Run(ctx, opts, logger)
}
