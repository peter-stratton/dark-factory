package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/stats"
)

// generateSummary is a testability seam: replaced in tests to stub the Claude API call.
// It accepts the fully-built prompt and returns the generated summary text.
// Returns an error if the API key is missing or the API call fails.
var generateSummary = func(ctx context.Context, prompt string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	apiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return callClaudeAPI(apiCtx, apiKey, prompt)
}

// callClaudeAPI sends a prompt to the Anthropic Messages API and returns the text response.
func callClaudeAPI(ctx context.Context, apiKey, prompt string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		Messages  []message `json:"messages"`
	}{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		Messages:  []message{{Role: "user", Content: prompt}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body) // drain to allow TCP connection reuse
		resp.Body.Close()                     //nolint:errcheck // best-effort close; ignore error
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
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
