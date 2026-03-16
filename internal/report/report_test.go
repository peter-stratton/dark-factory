package report_test

import (
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/report"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/stats"
)

// makeRun creates a minimal RunRecord for testing.
func makeRun(id, repo string) stats.RunRecord {
	return stats.RunRecord{
		ID:    id,
		Repo:  repo,
		Total: 5,
	}
}

// makeOutcome creates an IssueOutcomeRecord for testing.
func makeOutcome(runID string, issueNum int, status, title, errStr string) stats.IssueOutcomeRecord {
	return stats.IssueOutcomeRecord{
		RunID:       runID,
		IssueNumber: issueNum,
		Title:       title,
		Status:      status,
		Error:       errStr,
	}
}

// makeStep creates a minimal StepResultRecord for testing.
func makeStep(runID string, issueNum int, step string, cost float64) stats.StepResultRecord {
	return stats.StepResultRecord{
		RunID:       runID,
		IssueNumber: issueNum,
		StepName:    step,
		CostUSD:     cost,
	}
}

// buildTestData constructs 3 runs with 15 issues total: 10 implemented, 5 failed.
func buildTestData() ([]stats.RunRecord, []stats.IssueOutcomeRecord, []stats.StepResultRecord) {
	runs := []stats.RunRecord{
		makeRun("run-1", "owner/repo"),
		makeRun("run-2", "owner/repo"),
		makeRun("run-3", "owner/repo"),
	}

	var outcomes []stats.IssueOutcomeRecord
	var steps []stats.StepResultRecord

	// Run 1: 5 implemented
	for i := 1; i <= 5; i++ {
		outcomes = append(outcomes, makeOutcome("run-1", i, rundata.OutcomeStatusImplemented, "Feature "+string(rune('A'+i-1)), ""))
		steps = append(steps, makeStep("run-1", i, "implement", 0.50))
	}
	// Run 2: 5 implemented
	for i := 6; i <= 10; i++ {
		outcomes = append(outcomes, makeOutcome("run-2", i, rundata.OutcomeStatusImplemented, "Feature "+string(rune('A'+i-1)), ""))
		steps = append(steps, makeStep("run-2", i, "implement", 0.40))
	}
	// Run 3: 5 failed with varying costs
	outcomes = append(outcomes, makeOutcome("run-3", 11, rundata.OutcomeStatusFailed, "Broken feature A", "compile error"))
	steps = append(steps, makeStep("run-3", 11, "implement", 1.00))

	outcomes = append(outcomes, makeOutcome("run-3", 12, rundata.OutcomeStatusFailed, "Broken feature B", "test timeout"))
	steps = append(steps, makeStep("run-3", 12, "implement", 0.80))

	outcomes = append(outcomes, makeOutcome("run-3", 13, rundata.OutcomeStatusFailed, "Broken feature C", "build failed"))
	steps = append(steps, makeStep("run-3", 13, "implement", 0.60))

	outcomes = append(outcomes, makeOutcome("run-3", 14, rundata.OutcomeStatusFailed, "Broken feature D", "lint error"))
	steps = append(steps, makeStep("run-3", 14, "implement", 0.40))

	outcomes = append(outcomes, makeOutcome("run-3", 15, rundata.OutcomeStatusFailed, "Broken feature E", "runtime panic"))
	steps = append(steps, makeStep("run-3", 15, "implement", 0.20))

	return runs, outcomes, steps
}

func TestTerminalOutput(t *testing.T) {
	runs, outcomes, steps := buildTestData()
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(runs, outcomes, steps, since, until, "owner/repo")
	out := report.RenderTerminal(rpt)

	if rpt.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", rpt.TotalRuns)
	}
	if rpt.IssuesProcessed != 15 {
		t.Errorf("IssuesProcessed = %d, want 15", rpt.IssuesProcessed)
	}
	if rpt.IssuesImplemented != 10 {
		t.Errorf("IssuesImplemented = %d, want 10", rpt.IssuesImplemented)
	}
	if rpt.IssuesFailed != 5 {
		t.Errorf("IssuesFailed = %d, want 5", rpt.IssuesFailed)
	}

	// Check terminal output contains key labels.
	for _, want := range []string{"Sprint Report", "Summary", "Runs", "Implemented", "Success rate", "Cost"} {
		if !strings.Contains(out, want) {
			t.Errorf("terminal output missing %q", want)
		}
	}

	// Notable failures section should appear since we have failures.
	if !strings.Contains(out, "Notable Failures") {
		t.Error("terminal output missing Notable Failures section")
	}
}

func TestMarkdownOutput(t *testing.T) {
	runs, outcomes, steps := buildTestData()
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(runs, outcomes, steps, since, until, "")
	out := report.RenderMarkdown(rpt)

	// Check for ## headers.
	if !strings.Contains(out, "## Sprint Report") {
		t.Error("markdown missing ## Sprint Report header")
	}
	if !strings.Contains(out, "## Summary") {
		t.Error("markdown missing ## Summary header")
	}
	if !strings.Contains(out, "## Cost") {
		t.Error("markdown missing ## Cost header")
	}

	// Check for **bold** metrics.
	if !strings.Contains(out, "**") {
		t.Error("markdown missing bold metrics")
	}

	// Check for failure table.
	if !strings.Contains(out, "## Notable Failures") {
		t.Error("markdown missing ## Notable Failures header")
	}
	if !strings.Contains(out, "| Issue |") {
		t.Error("markdown missing failure table header row")
	}
}

func TestHTMLOutput(t *testing.T) {
	runs, outcomes, steps := buildTestData()
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(runs, outcomes, steps, since, until, "owner/repo")
	out := report.RenderHTML(rpt)

	// Must be valid HTML shell.
	if !strings.Contains(out, "<html>") {
		t.Error("HTML output missing <html> tag")
	}
	if !strings.Contains(out, "</html>") {
		t.Error("HTML output missing </html> closing tag")
	}

	// Inline styles — no external CSS links.
	if strings.Contains(out, "<link") {
		t.Error("HTML output must not contain <link> tags (no external CSS)")
	}
	if !strings.Contains(out, "style=") {
		t.Error("HTML output missing inline style attributes")
	}

	// Metric values present.
	if !strings.Contains(out, "15") {
		t.Error("HTML output missing IssuesProcessed value")
	}
	if !strings.Contains(out, "owner/repo") {
		t.Error("HTML output missing repo name")
	}
}

func TestNoFailures(t *testing.T) {
	runs := []stats.RunRecord{makeRun("run-1", "owner/repo")}
	outcomes := []stats.IssueOutcomeRecord{
		makeOutcome("run-1", 1, rundata.OutcomeStatusImplemented, "All good", ""),
		makeOutcome("run-1", 2, rundata.OutcomeStatusImplemented, "Also good", ""),
	}
	steps := []stats.StepResultRecord{
		makeStep("run-1", 1, "implement", 0.10),
		makeStep("run-1", 2, "implement", 0.10),
	}

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	rpt := report.Generate(runs, outcomes, steps, since, until, "")

	if len(rpt.NotableFailures) != 0 {
		t.Errorf("expected 0 notable failures, got %d", len(rpt.NotableFailures))
	}

	// Terminal output should omit the Notable Failures section entirely.
	out := report.RenderTerminal(rpt)
	if strings.Contains(out, "Notable Failures") {
		t.Error("terminal output should omit Notable Failures section when there are none")
	}

	// Markdown should also omit the section.
	mdOut := report.RenderMarkdown(rpt)
	if strings.Contains(mdOut, "## Notable Failures") {
		t.Error("markdown should omit Notable Failures section when there are none")
	}
}

func TestEmptyPeriod(t *testing.T) {
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(nil, nil, nil, since, until, "")

	if rpt.TotalRuns != 0 {
		t.Errorf("expected 0 runs, got %d", rpt.TotalRuns)
	}

	termOut := report.RenderTerminal(rpt)
	if !strings.Contains(termOut, "No runs found in this period") {
		t.Errorf("terminal output for empty period should say 'No runs found in this period', got: %s", termOut)
	}

	htmlOut := report.RenderHTML(rpt)
	if !strings.Contains(htmlOut, "No runs found") {
		t.Errorf("HTML output for empty period should say 'No runs found', got: %s", htmlOut)
	}

	mdOut := report.RenderMarkdown(rpt)
	if !strings.Contains(mdOut, "No runs found") {
		t.Errorf("markdown output for empty period should say 'No runs found', got: %s", mdOut)
	}
}

func TestNotableFailuresTop5(t *testing.T) {
	// Create 7 failed issues; only top 5 by cost should appear.
	runs := []stats.RunRecord{makeRun("run-1", "owner/repo")}
	var outcomes []stats.IssueOutcomeRecord
	var steps []stats.StepResultRecord
	for i := 1; i <= 7; i++ {
		outcomes = append(outcomes, makeOutcome("run-1", i, rundata.OutcomeStatusFailed, "Issue", "err"))
		steps = append(steps, makeStep("run-1", i, "implement", float64(i)*0.10))
	}

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	rpt := report.Generate(runs, outcomes, steps, since, until, "")

	if len(rpt.NotableFailures) != 5 {
		t.Errorf("expected 5 notable failures (capped), got %d", len(rpt.NotableFailures))
	}

	// The most expensive is issue #7 (cost 0.70).
	if rpt.NotableFailures[0].IssueNumber != 7 {
		t.Errorf("expected most expensive failure to be issue #7, got #%d", rpt.NotableFailures[0].IssueNumber)
	}
}

// TestExecSummaryTerminal verifies the executive summary appears before stats in terminal output.
func TestExecSummaryTerminal(t *testing.T) {
	runs, outcomes, steps := buildTestData()
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(runs, outcomes, steps, since, until, "owner/repo")
	rpt.ExecSummary = "The team shipped ten features this sprint, improving reliability."

	out := report.RenderTerminal(rpt)

	if !strings.Contains(out, "Executive Summary") {
		t.Error("terminal output missing 'Executive Summary' header")
	}
	if !strings.Contains(out, rpt.ExecSummary) {
		t.Error("terminal output missing executive summary text")
	}
	// Summary should appear before the stats section.
	summaryIdx := strings.Index(out, "Executive Summary")
	statsIdx := strings.Index(out, "Sprint Report")
	if summaryIdx > statsIdx {
		t.Error("executive summary should appear before the Sprint Report header")
	}
}

// TestExecSummaryMarkdown verifies the executive summary renders as ## Executive Summary.
func TestExecSummaryMarkdown(t *testing.T) {
	runs, outcomes, steps := buildTestData()
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(runs, outcomes, steps, since, until, "")
	rpt.ExecSummary = "Ten features were shipped, streamlining the developer workflow."

	out := report.RenderMarkdown(rpt)

	if !strings.Contains(out, "## Executive Summary") {
		t.Error("markdown output missing '## Executive Summary' header")
	}
	if !strings.Contains(out, rpt.ExecSummary) {
		t.Error("markdown output missing executive summary text")
	}
	// Summary should appear before the ## Sprint Report section.
	summaryIdx := strings.Index(out, "## Executive Summary")
	reportIdx := strings.Index(out, "## Sprint Report")
	if summaryIdx > reportIdx {
		t.Error("executive summary section should appear before ## Sprint Report")
	}
}

// TestExecSummaryHTML verifies the executive summary renders in a styled card in HTML.
func TestExecSummaryHTML(t *testing.T) {
	runs, outcomes, steps := buildTestData()
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(runs, outcomes, steps, since, until, "owner/repo")
	rpt.ExecSummary = "The sprint delivered reliability improvements across the board."

	out := report.RenderHTML(rpt)

	if !strings.Contains(out, "Executive Summary") {
		t.Error("HTML output missing 'Executive Summary' text")
	}
	if !strings.Contains(out, rpt.ExecSummary) {
		t.Error("HTML output missing executive summary text")
	}
	// Summary card should appear before the h1 Sprint Report heading.
	summaryIdx := strings.Index(out, "Executive Summary")
	reportIdx := strings.Index(out, "<h1")
	if summaryIdx > reportIdx {
		t.Error("executive summary card should appear before the h1 Sprint Report heading")
	}
}

// TestNoExecSummaryOmitted verifies the Executive Summary section is absent when ExecSummary is empty.
func TestNoExecSummaryOmitted(t *testing.T) {
	runs, outcomes, steps := buildTestData()
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	rpt := report.Generate(runs, outcomes, steps, since, until, "owner/repo")
	// ExecSummary is zero-value (empty string).

	for _, tc := range []struct {
		name string
		out  string
	}{
		{"terminal", report.RenderTerminal(rpt)},
		{"markdown", report.RenderMarkdown(rpt)},
		{"html", report.RenderHTML(rpt)},
	} {
		if strings.Contains(tc.out, "Executive Summary") {
			t.Errorf("%s output should omit Executive Summary when ExecSummary is empty", tc.name)
		}
	}
}
