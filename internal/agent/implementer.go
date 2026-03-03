package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
)

// Implement runs the implementer agent for a fresh issue implementation.
// It renders the implementer prompt and invokes Run.
func Implement(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug)

	rendered, err := RenderPrompt(prompts.Implementer, data)
	if err != nil {
		return nil, fmt.Errorf("rendering implementer prompt: %w", err)
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

	logger.Info("starting implementer agent",
		"issue_number", issue.Number,
		"issue_title", issue.Title,
		"branch", fmt.Sprintf("%d-%s", issue.Number, slug),
	)

	return Run(ctx, opts, cfg.NoSandbox, logger)
}

// Retry runs the implementer agent in retry mode for an existing PR.
// It renders the retry prompt with the PR number and invokes Run.
func Retry(ctx context.Context, issue github.Issue, prNumber int, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug)
	data.PRNumber = prNumber

	rendered, err := RenderPrompt(prompts.ImplementerRetry, data)
	if err != nil {
		return nil, fmt.Errorf("rendering implementer_retry prompt: %w", err)
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

	logger.Info("starting implementer retry agent",
		"issue_number", issue.Number,
		"pr_number", prNumber,
	)

	return Run(ctx, opts, cfg.NoSandbox, logger)
}

func newPromptData(issue github.Issue, cfg *config.Config, slug string) PromptData {
	return PromptData{
		IssueNumber:    issue.Number,
		IssueTitle:     issue.Title,
		IssueBody:      issue.Body,
		Slug:           slug,
		Repo:           cfg.Repo,
		BuildCommand:   cfg.BuildCommand,
		TestCommand:    cfg.TestCommand,
		ProtectedPaths: strings.Join(cfg.ProtectedPaths, ", "),
		ScenarioDir:    cfg.ScenarioDir,
		ReviewDir:      cfg.ReviewDir,
	}
}

func parseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 30 * time.Minute, nil
	}
	return time.ParseDuration(s)
}
