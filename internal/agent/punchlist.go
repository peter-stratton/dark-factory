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

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/punchlist"
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

// maxPRDiffLen is the maximum number of bytes to include from a PR diff.
const maxPRDiffLen = 30_000

// GenerateAcceptanceTests calls the LLM to produce acceptance test suggestions
// for a single punchlist entry. Returns nil on any failure (graceful degradation).
func GenerateAcceptanceTests(ctx context.Context, entry punchlist.Entry, prompts *Prompts, cfg *config.Config, authEnv map[string]string, logger *slog.Logger) []string {
	if prompts.Punchlist == "" {
		logger.Debug("no punchlist prompt loaded, skipping acceptance test generation")
		return nil
	}

	// Fetch PR diff.
	diff, err := punchlist.FetchPRDiff(cfg.Repo, entry.PRNumber, maxPRDiffLen)
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

	timeout := 2 * time.Minute
	opts := RunOpts{
		Prompt:  buf.String(),
		Role:    "punchlist",
		Env:     authEnv,
		Timeout: timeout,
	}

	result, err := Run(ctx, opts, cfg.NoSandbox, logger)
	if err != nil {
		logger.Warn("punchlist agent run failed", "error", err)
		return nil
	}
	if result.TimedOut {
		logger.Warn("punchlist agent timed out")
		return nil
	}

	tests := parseAcceptanceTests(result.ResultText, logger)
	if tests == nil {
		// Try stdout as fallback.
		tests = parseAcceptanceTests(result.Stdout, logger)
	}
	return tests
}

// EnrichPunchlistEntries calls GenerateAcceptanceTests for each entry concurrently
// (bounded to 3 goroutines) and populates AcceptanceTests on each entry in place.
func EnrichPunchlistEntries(ctx context.Context, entries []punchlist.Entry, prompts *Prompts, cfg *config.Config, authEnv map[string]string, logger *slog.Logger) {
	if prompts.Punchlist == "" {
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

// parseAcceptanceTests extracts a JSON array of strings from LLM output.
// Handles optional markdown code fences. Caps at 5 items.
func parseAcceptanceTests(text string, logger *slog.Logger) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

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

	var tests []string
	if err := json.Unmarshal([]byte(text), &tests); err != nil {
		// Try to find a JSON array within the text.
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(text[start:end+1]), &tests); err2 != nil {
				logger.Debug("failed to parse acceptance tests JSON", "error", err2, "text", text)
				return nil
			}
		} else {
			logger.Debug("failed to parse acceptance tests JSON", "error", err, "text", text)
			return nil
		}
	}

	if len(tests) > 5 {
		tests = tests[:5]
	}
	return tests
}
