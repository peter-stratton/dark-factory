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

	// Check if the feature branch already exists on the remote.
	branch := BranchName(issue.Number, slug)
	out, err := GuardRunner("git", "ls-remote", "--heads", "origin", branch)
	if err == nil && strings.TrimSpace(string(out)) != "" {
		data.BranchExists = true
	}

	rendered, err := RenderPrompt(prompts.Implementer, data)
	if err != nil {
		return nil, fmt.Errorf("rendering implementer prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "implementer")
	if err != nil {
		return nil, err
	}

	logger.Info("starting implementer agent",
		"issue_number", issue.Number,
		"issue_title", issue.Title,
		"branch", branch,
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

	opts, err := newRunOpts(rendered, cfg, authEnv, "implementer_retry")
	if err != nil {
		return nil, err
	}

	logger.Info("starting implementer retry agent",
		"issue_number", issue.Number,
		"pr_number", prNumber,
	)

	return Run(ctx, opts, cfg.NoSandbox, logger)
}

// BranchName returns the conventional branch name for an issue.
func BranchName(issueNumber int, slug string) string {
	return fmt.Sprintf("%d-%s", issueNumber, slug)
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

// newRunOpts builds a RunOpts from a rendered prompt, config, and role. This
// consolidates the repeated timeout-parsing + opts-construction that every
// agent function needs.
func newRunOpts(rendered string, cfg *config.Config, authEnv map[string]string, role string) (RunOpts, error) {
	timeout, err := parseTimeout(cfg.AgentTimeout)
	if err != nil {
		return RunOpts{}, fmt.Errorf("parsing agent_timeout: %w", err)
	}

	// Merge authEnv with GODARK_PROTECTED_PATHS so the agent runner can
	// enforce protected-path guardrails via in-process hooks.
	env := make(map[string]string, len(authEnv)+1)
	for k, v := range authEnv {
		env[k] = v
	}
	env["GODARK_PROTECTED_PATHS"] = strings.Join(cfg.ProtectedPaths, ",")

	return RunOpts{
		Prompt:  rendered,
		Role:    role,
		Env:     env,
		Image:   cfg.Docker.Image,
		Repo:    cfg.Repo,
		WorkDir: "/workspace",
		Timeout: timeout,
	}, nil
}

func parseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 30 * time.Minute, nil
	}
	return time.ParseDuration(s)
}
