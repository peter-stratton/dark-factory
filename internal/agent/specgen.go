package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// GenerateSpec runs the spec generator agent to create a scenario spec for
// the given issue. It follows the same pattern as Implement and Review.
func GenerateSpec(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug)

	rendered, err := RenderPrompt(prompts.SpecGenerator, data)
	if err != nil {
		return nil, fmt.Errorf("rendering spec_generator prompt: %w", err)
	}

	timeout, err := parseTimeout(cfg.AgentTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing agent_timeout: %w", err)
	}

	opts := RunOpts{
		Prompt:      rendered,
		Env:         authEnv,
		Image:       cfg.Docker.Image,
		Repo:        cfg.Repo,
		Branch:      "",
		WorkDir:     "/workspace",
		ClaudeFlags: cfg.ClaudeFlags,
		Timeout:     timeout,
	}

	logger.Info("starting spec generator agent",
		"issue_number", issue.Number,
		"issue_title", issue.Title,
		"branch", fmt.Sprintf("%d-%s", issue.Number, slug),
	)

	return Run(ctx, opts, cfg.NoSandbox, logger)
}
