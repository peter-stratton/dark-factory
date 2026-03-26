package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/peter-stratton/dark-factory/internal/stats"
)

// claudeRunner is a testability seam: replaced in tests to stub the claude CLI call.
// It accepts the prompt via stdin and returns the generated text from stdout.
var claudeRunner = func(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "--print")
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("claude --print: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("claude --print: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// generateSummary is a testability seam: replaced in tests to stub the claude CLI call.
// It accepts the fully-built prompt and returns the generated summary text.
var generateSummary = func(ctx context.Context, prompt string) (string, error) {
	apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return claudeRunner(apiCtx, prompt)
}

// buildSummaryPrompt constructs the prompt for executive summary generation.
// It reads phase overview files for any milestones found in runs, and includes
// issue titles and statuses from outcomes as source material.
func buildSummaryPrompt(runs []stats.RunRecord, outcomes []stats.IssueOutcomeRecord) string {
	var sb strings.Builder

	sb.WriteString("You are writing an executive summary for an engineering sprint report. ")
	sb.WriteString("Write 2-3 paragraphs in plain, non-technical language suitable for engineering managers. ")
	sb.WriteString("Explain what was accomplished, why it matters, and the scope and impact of the work. ")
	sb.WriteString("Avoid technical jargon. Focus on business value and progress.\n\n")

	// Include phase overview content when available.
	milestones := collectMilestones(runs)
	hasOverview := false
	for _, m := range milestones {
		overview := readPhaseOverview(m)
		if overview != "" {
			if !hasOverview {
				sb.WriteString("## Phase Context\n\n")
				hasOverview = true
			}
			fmt.Fprintf(&sb, "### %s\n\n%s\n\n", m, overview)
		}
	}

	// List issues with statuses as source material.
	sb.WriteString("## Issues Processed\n\n")
	if len(outcomes) == 0 {
		sb.WriteString("No issues processed in this period.\n")
	} else {
		for _, o := range outcomes {
			if o.Title != "" {
				fmt.Fprintf(&sb, "- [%s] #%d: %s\n", o.Status, o.IssueNumber, o.Title)
			}
		}
	}

	sb.WriteString("\nPlease write the executive summary now.")
	return sb.String()
}

// collectMilestones returns unique, non-empty milestone strings from runs in
// the order they are first encountered.
func collectMilestones(runs []stats.RunRecord) []string {
	seen := make(map[string]bool)
	var result []string
	for _, r := range runs {
		if r.Milestone != "" && !seen[r.Milestone] {
			seen[r.Milestone] = true
			result = append(result, r.Milestone)
		}
	}
	return result
}

// milestoneRE extracts a phase number from strings like "Phase 22" or "phase 3".
var milestoneRE = regexp.MustCompile(`(?i)phase\s+(\d+)`)

// readPhaseOverview reads docs/phase-overviews/phase-NN-*.md files for the
// given milestone string. Returns the concatenated file content, or an empty
// string if no matching files are found.
func readPhaseOverview(milestone string) string {
	m := milestoneRE.FindStringSubmatch(milestone)
	if len(m) < 2 {
		return ""
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return ""
	}
	pattern := filepath.Join("docs", "phase-overviews", fmt.Sprintf("phase-%02d-*.md", n))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	var parts []string
	for _, path := range matches {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		parts = append(parts, string(data))
	}
	return strings.Join(parts, "\n\n")
}
