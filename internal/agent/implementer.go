package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
)

// Implement runs the implementer agent for a fresh issue implementation.
// It renders the implementer prompt and invokes Run.
// reconBrief, if non-empty, is injected into the prompt as recon agent context.
func Implement(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger, reconBrief string) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug)
	data.ReconBrief = reconBrief

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

	return Run(ctx, opts, logger)
}

// Retry runs the implementer agent in retry mode for an existing PR.
// It renders the retry prompt with the PR number and invokes Run.
// prevSessionID, if non-empty, is passed as GODARK_SESSION_ID so the agent
// can resume its previous session context.
// reviewFeedback, if non-empty, is injected into the retry prompt so the
// agent sees reviewer comments without needing to fetch them from GitHub
// (used for human review cycles initiated by the watch command).
// handoffContext, if non-empty, signals a fresh-session retry: GODARK_SESSION_ID
// is NOT set even when prevSessionID is provided, so the agent starts fresh.
// The handoff context is rendered into the prompt so the new agent understands
// what was tried and what failed without inheriting a degraded context window.
func Retry(ctx context.Context, issue github.Issue, prNumber int, prevSessionID string, reviewFeedback string, handoffContext string, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug)
	data.PRNumber = prNumber
	data.ReviewFeedback = reviewFeedback
	data.HandoffContext = handoffContext

	rendered, err := RenderPrompt(prompts.ImplementerRetry, data)
	if err != nil {
		return nil, fmt.Errorf("rendering implementer_retry prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "implementer_retry")
	if err != nil {
		return nil, err
	}

	// Only resume the prior session when no handoff context is provided.
	// A non-empty handoffContext means we want a fresh session — the agent
	// will get the prior context via the structured handoff in the prompt.
	if prevSessionID != "" && handoffContext == "" {
		opts.Env["GODARK_SESSION_ID"] = prevSessionID
	}

	logger.Info("starting implementer retry agent",
		"issue_number", issue.Number,
		"pr_number", prNumber,
		"resume_session", prevSessionID != "" && handoffContext == "",
		"fresh_session", handoffContext != "",
	)

	return Run(ctx, opts, logger)
}

// VerifyFix runs the implementer agent in verify-fix mode. It renders the
// verify_fix prompt with the provided verifyErrors string and invokes Run
// with role "implementer_retry". prevSessionID, if non-empty, is forwarded
// as GODARK_SESSION_ID so the agent can resume its previous session context.
func VerifyFix(ctx context.Context, issue github.Issue, prNumber int, verifyErrors string, prevSessionID string, cfg *config.Config, prompts *Prompts, authEnv map[string]string, logger *slog.Logger) (*Result, error) {
	slug := Slugify(issue.Title)
	data := newPromptData(issue, cfg, slug)
	data.PRNumber = prNumber
	data.VerifyErrors = verifyErrors

	rendered, err := RenderPrompt(prompts.VerifyFix, data)
	if err != nil {
		return nil, fmt.Errorf("rendering verify_fix prompt: %w", err)
	}

	opts, err := newRunOpts(rendered, cfg, authEnv, "implementer_retry")
	if err != nil {
		return nil, err
	}

	if prevSessionID != "" {
		opts.Env["GODARK_SESSION_ID"] = prevSessionID
	}

	logger.Info("starting verify-fix agent",
		"issue_number", issue.Number,
		"pr_number", prNumber,
		"resume_session", prevSessionID != "",
	)

	return Run(ctx, opts, logger)
}

// BranchName returns the conventional branch name for an issue.
func BranchName(issueNumber int, slug string) string {
	return fmt.Sprintf("%d-%s", issueNumber, slug)
}

func newPromptData(issue github.Issue, cfg *config.Config, slug string) PromptData {
	protectedPaths := strings.Join(cfg.ProtectedPaths, ", ")
	return PromptData{
		IssueNumber:            issue.Number,
		IssueTitle:             issue.Title,
		IssueBody:              issue.Body,
		Slug:                   slug,
		Repo:                   cfg.Repo,
		BuildCommand:           cfg.BuildCommand,
		TestCommand:            cfg.TestCommand,
		ProtectedPaths:         protectedPaths,
		GeneratedPaths:         strings.Join(cfg.GeneratedPaths, ", "),
		ScenarioDir:            cfg.ScenarioDir,
		ReviewDir:              cfg.ReviewDir,
		HasScenarioSpec:        HasScenarioSpec(cfg.ScenarioDir, issue.Number),
		EnforceArchitecture:    cfg.EnforceArchitecture,
		ArchitectureDocContent: readFileOrEmpty(cfg.ArchitectureDoc),
		ArchitectureJSON:       readFileOrEmpty(cfg.ArchitectureJSON),
		ConventionsDocContent:  readFileOrEmpty(cfg.ConventionsDoc),
		ModuleContext:          buildModuleContext(cfg.Modules),
		ComposeServices:        buildComposeServices(cfg.DockerCompose, cfg.HostServices),
		BaseBranch:             cfg.BaseBranch,
		SharedRules:            buildSharedRules(protectedPaths, cfg.ScenarioDir),
	}
}

// buildSharedRules assembles the universally shared subset of CRITICAL RULES
// that appears in every agent prompt: protected path protection and scenario
// directory protection. Each present rule is a bullet terminated by a newline.
// Returns an empty string when neither ProtectedPaths nor ScenarioDir is set.
func buildSharedRules(protectedPaths, scenarioDir string) string {
	var sb strings.Builder
	if protectedPaths != "" {
		sb.WriteString("- Do NOT modify any protected paths: ")
		sb.WriteString(protectedPaths)
		sb.WriteString("\n")
	}
	if scenarioDir != "" {
		sb.WriteString("- Do NOT modify files in ")
		sb.WriteString(scenarioDir)
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildModuleContext renders a human-readable summary of configured modules
// and their dependency relationships for use in prompt templates.
// Returns empty string when no modules are configured.
func buildModuleContext(modules map[string]config.Module) string {
	if len(modules) == 0 {
		return ""
	}
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		mod := modules[name]
		sb.WriteString("- ")
		sb.WriteString(name)
		if len(mod.DependsOn) > 0 {
			sb.WriteString(" (depends_on: ")
			sb.WriteString(strings.Join(mod.DependsOn, ", "))
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildComposeServices renders a human-readable summary of configured
// Docker Compose services and host services for use in prompt templates.
// Returns empty string when no services are configured.
func buildComposeServices(dc *config.DockerCompose, hostServices []config.HostService) string {
	hasCompose := dc != nil && len(dc.Services) > 0
	hasHost := len(hostServices) > 0
	if !hasCompose && !hasHost {
		return ""
	}
	var sb strings.Builder
	if hasCompose {
		for _, svc := range dc.Services {
			sb.WriteString("- ")
			sb.WriteString(svc.Name)
			if svc.Description != "" {
				sb.WriteString(": ")
				sb.WriteString(svc.Description)
			}
			sb.WriteString("\n")
		}
	}
	if hasHost {
		for _, svc := range hostServices {
			sb.WriteString("- ")
			sb.WriteString(svc.Name)
			if svc.Description != "" {
				sb.WriteString(": ")
				sb.WriteString(svc.Description)
			}
			sb.WriteString(" (host)")
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// readFileOrEmpty reads the file at path and returns its contents as a string.
// If the file does not exist or cannot be read, it returns an empty string.
func readFileOrEmpty(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
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
	env["GODARK_GENERATED_PATHS"] = strings.Join(cfg.GeneratedPaths, ",")
	env["GODARK_DENIED_COMMANDS"] = strings.Join(cfg.DeniedCommands, ",")

	jc := cfg.JudgeConfig()
	return RunOpts{
		Prompt:            rendered,
		Role:              role,
		Env:               env,
		Image:             cfg.Docker.Image,
		Repo:              cfg.Repo,
		WorkDir:           "/workspace",
		Timeout:           timeout,
		MountDockerSocket: cfg.DockerCompose != nil,
		JudgeConfig:       &jc,
	}, nil
}

func parseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 30 * time.Minute, nil
	}
	return time.ParseDuration(s)
}
