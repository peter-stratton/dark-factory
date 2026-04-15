package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// Recon runs the recon agent to gather context for an issue before implementation.
// It follows the same pattern as GenerateSpec. Recon is non-blocking — callers
// should treat errors as warnings and proceed with an empty brief.
func Recon(ctx context.Context, issue github.Issue, integration bool, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug, integration)

	rendered, err := RenderPrompt(prompts.Recon, data)
	if err != nil {
		return nil, fmt.Errorf("rendering recon prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "recon", integration)
	if err != nil {
		return nil, err
	}

	logger.Info("starting recon agent",
		"issue_number", issue.Number,
		"issue_title", issue.Title,
	)

	result, err := Run(ctx, opts, logger)
	if err != nil {
		return nil, err
	}
	result.Prompt = rendered
	return result, nil
}
