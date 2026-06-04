package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/punchlist"
)

// PunchlistPromptData holds the template data for a punchlist prompt.
type PunchlistPromptData struct {
	IssueNumber  int
	IssueTitle   string
	IssueBody    string
	PRDiff       string
	ChangedFiles []string
	ScenarioSpec string
}

// GenerateAcceptanceTests calls the LLM to produce acceptance test suggestions
// for a single punchlist entry. Returns nil on any failure (graceful degradation).
func GenerateAcceptanceTests(ctx context.Context, entry punchlist.Entry, prompts *Prompts, cfg *config.Config, authEnv map[string]string, logger *slog.Logger) []string {
	if prompts.Punchlist == "" {
		logger.Info("punchlist acceptance test generation skipped: no prompt configured",
			"issue_number", entry.IssueNumber)
		return nil
	}

	// Fetch PR diff.
	diff, err := punchlist.FetchPRDiff(cfg.Repo, entry.PRNumber, cfg.Truncation.PRDiff)
	if err != nil {
		logger.Warn("failed to fetch PR diff for acceptance tests",
			"pr_number", entry.PRNumber, "error", err)
	}

	data := PunchlistPromptData{
		IssueNumber:  entry.IssueNumber,
		IssueTitle:   entry.IssueTitle,
		IssueBody:    entry.IssueBody,
		PRDiff:       diff,
		ChangedFiles: entry.ChangedFiles,
		ScenarioSpec: entry.ScenarioSpec,
	}

	tmpl, err := template.New("punchlist").Parse(prompts.Punchlist)
	if err != nil {
		logger.Warn("failed to parse punchlist prompt template", "error", err)
		return nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		logger.Warn("failed to render punchlist prompt", "error", err)
		return nil
	}

	// Clone the branch that holds the work under review. During a milestone run
	// this is the integration base branch (where feature PRs merge); for single
	// runs with no base branch it is the repo's actual default branch. Never
	// assume "main": repos default to develop, master, etc., and a wrong branch
	// makes `git checkout` fail, exiting the container before any tests are
	// generated.
	branch := cfg.BaseBranch
	if branch == "" {
		branch = cfg.EffectiveDefaultBranch(cfg.Repo)
	}

	timeout := 10 * time.Minute
	opts := RunOpts{
		Prompt:  buf.String(),
		Role:    "punchlist",
		Env:     authEnv,
		Image:   cfg.Docker.Image,
		Repo:    cfg.Repo,
		Branch:  branch,
		Timeout: timeout,
	}

	result, err := Run(ctx, opts, logger)
	if err != nil {
		logger.Warn("punchlist agent run failed", "error", err)
		return nil
	}
	if result.TimedOut {
		logger.Warn("punchlist agent timed out")
		return nil
	}

	tests := parseAcceptanceTests(result.ResultText, entry.IssueNumber, logger)
	if tests == nil {
		// Try stdout as fallback.
		tests = parseAcceptanceTests(result.Stdout, entry.IssueNumber, logger)
	}
	if tests != nil {
		logger.Info("punchlist acceptance tests generated",
			"issue_number", entry.IssueNumber,
			"count", len(tests))
	}
	return tests
}

// EnrichPunchlistEntries calls GenerateAcceptanceTests for each entry concurrently
// (bounded to 3 goroutines) and populates AcceptanceTests on each entry in place.
func EnrichPunchlistEntries(ctx context.Context, entries []punchlist.Entry, prompts *Prompts, cfg *config.Config, authEnv map[string]string, logger *slog.Logger) {
	if prompts.Punchlist == "" {
		logger.Info("punchlist enrichment skipped: no prompt configured")
		return
	}

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for i := range entries {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tests := GenerateAcceptanceTests(ctx, entries[idx], prompts, cfg, authEnv, logger)
			entries[idx].AcceptanceTests = tests
		}(i)
	}

	wg.Wait()
}

// extractJSONArray finds and returns a JSON array substring from text.
// It strips markdown code fences first, then tries to unmarshal the full text.
// On failure, it scans for the outermost [...] using bracket-depth tracking.
// Returns (jsonSubstring, true) on success, ("", false) if no valid array found.
func extractJSONArray(text string) (string, bool) {
	// Strip markdown code fences if present.
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		// Remove first line (```json or ```) and last line (```)
		if len(lines) >= 2 {
			end := len(lines) - 1
			for end > 0 && strings.TrimSpace(lines[end]) == "" {
				end--
			}
			if strings.TrimSpace(lines[end]) == "```" {
				lines = lines[1:end]
			} else {
				lines = lines[1:]
			}
			text = strings.Join(lines, "\n")
		}
	}

	// Try the full text first.
	var probe []string
	if json.Unmarshal([]byte(text), &probe) == nil {
		return text, true
	}

	// Find outermost [...] using bracket-depth tracking.
	start := -1
	depth := 0
	for i, ch := range text {
		switch ch {
		case '[':
			if depth == 0 {
				start = i
			}
			depth++
		case ']':
			depth--
			if depth == 0 && start >= 0 {
				candidate := text[start : i+1]
				if json.Unmarshal([]byte(candidate), &probe) == nil {
					return candidate, true
				}
				// Reset and keep scanning for another array.
				start = -1
			}
		}
	}
	return "", false
}

// parseAcceptanceTests extracts a JSON array of strings from LLM output.
// Handles optional markdown code fences. Caps at 5 items.
func parseAcceptanceTests(text string, issueNumber int, logger *slog.Logger) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	jsonStr, ok := extractJSONArray(text)
	if !ok {
		logger.Warn("failed to parse acceptance tests JSON", "issue_number", issueNumber, "raw_response", text)
		return nil
	}

	var tests []string
	if err := json.Unmarshal([]byte(jsonStr), &tests); err != nil {
		logger.Warn("failed to parse acceptance tests JSON", "issue_number", issueNumber, "error", err, "raw_response", text)
		return nil
	}

	if len(tests) > 5 {
		tests = tests[:5]
	}
	return tests
}
