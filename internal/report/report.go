// Package report generates sprint summary reports from run statistics.
// It sits in the domain layer and depends only on internal/analysis,
// internal/stats, and internal/rundata.
package report

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/phs/dark-factory/internal/analysis"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/stats"
)

// NotableFailure represents a failed issue that incurred significant cost.
type NotableFailure struct {
	IssueNumber int
	Title       string
	Error       string
	CostUSD     float64
}

// SprintReport summarises a sprint period for engineering managers.
type SprintReport struct {
	Since   time.Time
	Until   time.Time
	Repo string // empty = all repos

	TotalRuns            int
	IssuesProcessed      int
	IssuesImplemented    int
	IssuesFailed         int
	SuccessRate          float64
	FirstPassRate        float64
	TotalCostUSD         float64
	AvgCostPerSuccessUSD float64
	WastedCostUSD        float64

	FailureReasons  map[string]int
	NotableFailures []NotableFailure
}

// Generate computes a SprintReport from pre-queried stats DB records and
// the time range that was used to query them.
func Generate(runs []stats.RunRecord, outcomes []stats.IssueOutcomeRecord, steps []stats.StepResultRecord, since, until time.Time, repo string) SprintReport {
	rpt := SprintReport{
		Since: since,
		Until: until,
		Repo:  repo,
	}

	if len(runs) == 0 {
		return rpt
	}

	runDetails := stats.ToRunDetails(runs, outcomes, steps)
	agg := analysis.Aggregate(runDetails)

	rpt.TotalRuns = agg.RunCount
	rpt.IssuesProcessed = agg.IssueCount
	rpt.IssuesImplemented = agg.Outcomes[rundata.OutcomeStatusImplemented] + agg.Outcomes[rundata.OutcomeStatusReadyToMerge]
	rpt.IssuesFailed = agg.Outcomes[rundata.OutcomeStatusFailed] + agg.Outcomes[rundata.OutcomeStatusNeedsHumanReview]
	if agg.IssueCount > 0 {
		rpt.SuccessRate = float64(rpt.IssuesImplemented) / float64(agg.IssueCount)
	}
	rpt.FirstPassRate = agg.FirstPassSuccessRate
	rpt.TotalCostUSD = agg.CostStats.TotalUSD
	rpt.AvgCostPerSuccessUSD = agg.AvgCostPerSuccessUSD
	rpt.WastedCostUSD = agg.WastedCostUSD
	rpt.FailureReasons = agg.FailureReasons

	rpt.NotableFailures = buildNotableFailures(runDetails)
	return rpt
}

// buildNotableFailures returns the top 5 most expensive failed issues.
func buildNotableFailures(runDetails []rundata.RunDetail) []NotableFailure {
	type issueCost struct {
		issue   rundata.IssueDetail
		costUSD float64
	}

	var failures []issueCost
	for _, rd := range runDetails {
		for _, issue := range rd.Issues {
			if issue.Outcome.Status != rundata.OutcomeStatusFailed &&
				issue.Outcome.Status != rundata.OutcomeStatusNeedsHumanReview {
				continue
			}
			cost := computeIssueCost(issue)
			failures = append(failures, issueCost{issue: issue, costUSD: cost})
		}
	}

	sort.Slice(failures, func(i, j int) bool {
		if failures[i].costUSD != failures[j].costUSD {
			return failures[i].costUSD > failures[j].costUSD
		}
		return failures[i].issue.IssueNumber < failures[j].issue.IssueNumber
	})

	const maxFailures = 5
	if len(failures) > maxFailures {
		failures = failures[:maxFailures]
	}

	result := make([]NotableFailure, 0, len(failures))
	for _, f := range failures {
		result = append(result, NotableFailure{
			IssueNumber: f.issue.Outcome.IssueNumber,
			Title:       f.issue.Outcome.Title,
			Error:       f.issue.Outcome.Error,
			CostUSD:     f.costUSD,
		})
	}
	return result
}

// computeIssueCost sums the cost of all steps (including retries) for an issue.
func computeIssueCost(issue rundata.IssueDetail) float64 {
	total := issue.Recon.CostUSD + issue.SpecGenerator.CostUSD +
		issue.Implement.CostUSD + issue.QualityReview.CostUSD +
		issue.FunctionalReview.CostUSD
	for _, retry := range issue.Retries {
		total += retry.Retry.CostUSD + retry.QualityReview.CostUSD + retry.FunctionalReview.CostUSD
	}
	return total
}

// RenderTerminal renders rpt as a tabwriter-aligned table for stdout.
func RenderTerminal(rpt SprintReport) string {
	var sb strings.Builder

	if rpt.TotalRuns == 0 {
		sb.WriteString("No runs found in this period\n")
		return sb.String()
	}

	// Header
	sb.WriteString(fmt.Sprintf("Sprint Report  %s – %s\n", formatDate(rpt.Since), formatDate(rpt.Until)))
	if rpt.Repo != "" {
		sb.WriteString(fmt.Sprintf("Repo: %s\n", rpt.Repo))
	}

	// Summary table
	sb.WriteString("\nSummary\n")
	tw := tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Runs\t%d\n", rpt.TotalRuns)
	fmt.Fprintf(tw, "  Issues processed\t%d\n", rpt.IssuesProcessed)
	fmt.Fprintf(tw, "  Implemented\t%d\n", rpt.IssuesImplemented)
	fmt.Fprintf(tw, "  Failed\t%d\n", rpt.IssuesFailed)
	fmt.Fprintf(tw, "  Success rate\t%.1f%%\n", rpt.SuccessRate*100)
	fmt.Fprintf(tw, "  First-pass rate\t%.1f%%\n", rpt.FirstPassRate*100)
	_ = tw.Flush()

	// Cost table
	sb.WriteString("\nCost\n")
	tw = tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Total\t$%.2f\n", rpt.TotalCostUSD)
	fmt.Fprintf(tw, "  Avg per success\t$%.2f\n", rpt.AvgCostPerSuccessUSD)
	fmt.Fprintf(tw, "  Wasted\t$%.2f\n", rpt.WastedCostUSD)
	_ = tw.Flush()

	// Failure reasons
	if len(rpt.FailureReasons) > 0 {
		sb.WriteString("\nFailure Reasons\n")
		tw = tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		reasons := sortedKeys(rpt.FailureReasons)
		for _, reason := range reasons {
			fmt.Fprintf(tw, "  %s\t%d\n", reason, rpt.FailureReasons[reason])
		}
		_ = tw.Flush()
	}

	// Notable failures
	if len(rpt.NotableFailures) > 0 {
		sb.WriteString("\nNotable Failures (top 5 by cost)\n")
		tw = tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "  Issue\tTitle\tCost\tError\n")
		for _, f := range rpt.NotableFailures {
			title := truncate(f.Title, 40)
			errStr := truncate(f.Error, 60)
			fmt.Fprintf(tw, "  #%d\t%s\t$%.2f\t%s\n", f.IssueNumber, title, f.CostUSD, errStr)
		}
		_ = tw.Flush()
	}

	return sb.String()
}

// RenderMarkdown renders rpt as Markdown suitable for Slack or a wiki.
func RenderMarkdown(rpt SprintReport) string {
	var sb strings.Builder

	sb.WriteString("## Sprint Report\n\n")
	sb.WriteString(fmt.Sprintf("**Period:** %s – %s\n\n", formatDate(rpt.Since), formatDate(rpt.Until)))
	if rpt.Repo != "" {
		sb.WriteString(fmt.Sprintf("**Repo:** %s\n\n", rpt.Repo))
	}

	if rpt.TotalRuns == 0 {
		sb.WriteString("No runs found in this period.\n")
		return sb.String()
	}

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Runs:** %d\n", rpt.TotalRuns))
	sb.WriteString(fmt.Sprintf("- **Issues processed:** %d\n", rpt.IssuesProcessed))
	sb.WriteString(fmt.Sprintf("- **Implemented:** %d\n", rpt.IssuesImplemented))
	sb.WriteString(fmt.Sprintf("- **Failed:** %d\n", rpt.IssuesFailed))
	sb.WriteString(fmt.Sprintf("- **Success rate:** **%.1f%%**\n", rpt.SuccessRate*100))
	sb.WriteString(fmt.Sprintf("- **First-pass rate:** **%.1f%%**\n", rpt.FirstPassRate*100))

	sb.WriteString("\n## Cost\n\n")
	sb.WriteString(fmt.Sprintf("- **Total:** $%.2f\n", rpt.TotalCostUSD))
	sb.WriteString(fmt.Sprintf("- **Avg per success:** $%.2f\n", rpt.AvgCostPerSuccessUSD))
	sb.WriteString(fmt.Sprintf("- **Wasted:** $%.2f\n", rpt.WastedCostUSD))

	if len(rpt.FailureReasons) > 0 {
		sb.WriteString("\n## Failure Reasons\n\n")
		reasons := sortedKeys(rpt.FailureReasons)
		for _, reason := range reasons {
			sb.WriteString(fmt.Sprintf("- **%s:** %d\n", reason, rpt.FailureReasons[reason]))
		}
	}

	if len(rpt.NotableFailures) > 0 {
		sb.WriteString("\n## Notable Failures\n\n")
		sb.WriteString("| Issue | Title | Cost | Error |\n")
		sb.WriteString("|-------|-------|------|-------|\n")
		for _, f := range rpt.NotableFailures {
			title := strings.ReplaceAll(f.Title, "|", "\\|")
			errStr := strings.ReplaceAll(truncate(f.Error, 80), "|", "\\|")
			sb.WriteString(fmt.Sprintf("| #%d | %s | $%.2f | %s |\n", f.IssueNumber, title, f.CostUSD, errStr))
		}
	}

	return sb.String()
}

// RenderHTML renders rpt as self-contained inline-styled HTML suitable for email.
func RenderHTML(rpt SprintReport) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Sprint Report</title></head>
<body style="font-family:sans-serif;max-width:800px;margin:0 auto;padding:20px;color:#333">
`)

	sb.WriteString(`<h1 style="color:#1a1a1a">Sprint Report</h1>
`)
	sb.WriteString(fmt.Sprintf(`<p><strong>Period:</strong> %s – %s</p>
`, html.EscapeString(formatDate(rpt.Since)), html.EscapeString(formatDate(rpt.Until))))
	if rpt.Repo != "" {
		sb.WriteString(fmt.Sprintf(`<p><strong>Repo:</strong> %s</p>
`, html.EscapeString(rpt.Repo)))
	}

	if rpt.TotalRuns == 0 {
		sb.WriteString(`<p style="color:#888">No runs found in this period.</p>
</body>
</html>
`)
		return sb.String()
	}

	sb.WriteString(`<h2 style="color:#333;border-bottom:1px solid #ddd">Summary</h2>
<table style="border-collapse:collapse;width:100%">
`)
	writeHTMLRow(&sb, "Runs", fmt.Sprintf("%d", rpt.TotalRuns))
	writeHTMLRow(&sb, "Issues processed", fmt.Sprintf("%d", rpt.IssuesProcessed))
	writeHTMLRow(&sb, "Implemented", fmt.Sprintf("%d", rpt.IssuesImplemented))
	writeHTMLRow(&sb, "Failed", fmt.Sprintf("%d", rpt.IssuesFailed))
	writeHTMLRow(&sb, "Success rate", fmt.Sprintf("%.1f%%", rpt.SuccessRate*100))
	writeHTMLRow(&sb, "First-pass rate", fmt.Sprintf("%.1f%%", rpt.FirstPassRate*100))
	sb.WriteString("</table>\n")

	sb.WriteString(`<h2 style="color:#333;border-bottom:1px solid #ddd">Cost</h2>
<table style="border-collapse:collapse;width:100%">
`)
	writeHTMLRow(&sb, "Total", fmt.Sprintf("$%.2f", rpt.TotalCostUSD))
	writeHTMLRow(&sb, "Avg per success", fmt.Sprintf("$%.2f", rpt.AvgCostPerSuccessUSD))
	writeHTMLRow(&sb, "Wasted", fmt.Sprintf("$%.2f", rpt.WastedCostUSD))
	sb.WriteString("</table>\n")

	if len(rpt.FailureReasons) > 0 {
		sb.WriteString(`<h2 style="color:#333;border-bottom:1px solid #ddd">Failure Reasons</h2>
<table style="border-collapse:collapse;width:100%">
`)
		reasons := sortedKeys(rpt.FailureReasons)
		for _, reason := range reasons {
			writeHTMLRow(&sb, reason, fmt.Sprintf("%d", rpt.FailureReasons[reason]))
		}
		sb.WriteString("</table>\n")
	}

	if len(rpt.NotableFailures) > 0 {
		sb.WriteString(`<h2 style="color:#333;border-bottom:1px solid #ddd">Notable Failures</h2>
<table style="border-collapse:collapse;width:100%">
<tr style="background:#f5f5f5">
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Issue</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Title</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Cost</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Error</th>
</tr>
`)
		for _, f := range rpt.NotableFailures {
			sb.WriteString(fmt.Sprintf(
				`<tr><td style="padding:8px;border:1px solid #ddd">#%d</td><td style="padding:8px;border:1px solid #ddd">%s</td><td style="padding:8px;border:1px solid #ddd">$%.2f</td><td style="padding:8px;border:1px solid #ddd">%s</td></tr>
`,
				f.IssueNumber,
				html.EscapeString(f.Title),
				f.CostUSD,
				html.EscapeString(truncate(f.Error, 120)),
			))
		}
		sb.WriteString("</table>\n")
	}

	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}

// writeHTMLRow writes a two-column table row with inline styles.
func writeHTMLRow(sb *strings.Builder, label, value string) {
	sb.WriteString(fmt.Sprintf(
		`<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold">%s</td><td style="padding:8px;border:1px solid #ddd">%s</td></tr>
`,
		html.EscapeString(label),
		html.EscapeString(value),
	))
}

// formatDate formats a time as YYYY-MM-DD. Returns "(none)" for zero time.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "(none)"
	}
	return t.Format("2006-01-02")
}

// sortedKeys returns the map keys in sorted order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// truncate truncates s to at most n runes, appending "..." if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
