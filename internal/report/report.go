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

// ShippedIssue represents an implemented issue included in the "What was
// shipped" section of the report. Repo and PRNumber are used to construct
// clickable GitHub PR links in non-terminal output formats.
type ShippedIssue struct {
	IssueNumber int
	Title       string
	PRNumber    int
	Repo        string
}

// SprintReport summarises a sprint period for engineering managers.
type SprintReport struct {
	Since time.Time
	Until time.Time
	Repo  string // empty = all repos

	// ExecSummary is an optional LLM-generated executive summary in plain
	// language. When non-empty it is rendered as the first section of all
	// output formats.
	ExecSummary string

	TotalRuns            int
	IssuesProcessed      int
	IssuesImplemented    int
	IssuesFailed         int
	SuccessRate          float64
	FirstPassRate        float64
	TotalCostUSD         float64
	AvgCostPerSuccessUSD float64
	WastedCostUSD        float64

	// ResourceStats fields. Zero values mean no resource data was collected.
	PeakMemoryBytes           int64 // max single-step peak across all steps in window
	TotalCPUNanoseconds       int64 // sum of CPU nanoseconds across all steps
	AvgCPUNanosecondsPerIssue int64 // TotalCPUNanoseconds / IssuesProcessed

	FailureReasons  map[string]int
	NotableFailures []NotableFailure

	// ShippedIssues lists all implemented issues in this period, sorted by
	// issue number. Used to render the "What was shipped" section.
	ShippedIssues []ShippedIssue

	// PriorPeriod holds the report for the equivalent preceding time window.
	// When non-nil and non-empty, it is used to render the period comparison
	// section. Set by the caller after Generate() returns; nil means no prior
	// data is available.
	PriorPeriod *SprintReport
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

	if agg.ResourceStats != nil {
		rpt.PeakMemoryBytes = agg.ResourceStats.MaxPeakMemoryBytes
		rpt.TotalCPUNanoseconds = agg.ResourceStats.TotalCPUNanoseconds
		rpt.AvgCPUNanosecondsPerIssue = agg.ResourceStats.AvgCPUNanosecondsPerIssue
	}

	rpt.NotableFailures = buildNotableFailures(runDetails)
	rpt.ShippedIssues = buildShippedIssues(runDetails)
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

// buildShippedIssues returns all implemented issues sorted by issue number.
func buildShippedIssues(runDetails []rundata.RunDetail) []ShippedIssue {
	var result []ShippedIssue
	for _, rd := range runDetails {
		repo := rd.Repo
		for _, issue := range rd.Issues {
			if issue.Outcome.Status != rundata.OutcomeStatusImplemented &&
				issue.Outcome.Status != rundata.OutcomeStatusReadyToMerge {
				continue
			}
			result = append(result, ShippedIssue{
				IssueNumber: issue.Outcome.IssueNumber,
				Title:       issue.Outcome.Title,
				PRNumber:    issue.Outcome.PRNumber,
				Repo:        repo,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].IssueNumber < result[j].IssueNumber
	})
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

// renderResourceSection writes the Resource Usage block to sb when resource data is present.
func renderResourceSection(sb *strings.Builder, rpt SprintReport) {
	if rpt.PeakMemoryBytes == 0 && rpt.TotalCPUNanoseconds == 0 {
		return
	}
	sb.WriteString("\nResource Usage\n")
	tw := tabwriter.NewWriter(sb, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Peak memory\t%.1f MB\n", float64(rpt.PeakMemoryBytes)/1048576)
	fmt.Fprintf(tw, "  Total CPU\t%.2f s\n", float64(rpt.TotalCPUNanoseconds)/1e9)
	fmt.Fprintf(tw, "  Avg CPU per issue\t%.2f s\n", float64(rpt.AvgCPUNanosecondsPerIssue)/1e9)
	_ = tw.Flush()
}

// RenderTerminal renders rpt as a tabwriter-aligned table for stdout.
func RenderTerminal(rpt SprintReport) string {
	var sb strings.Builder

	if rpt.ExecSummary != "" {
		sb.WriteString("Executive Summary\n\n")
		sb.WriteString(rpt.ExecSummary)
		sb.WriteString("\n\n")
	}

	if rpt.TotalRuns == 0 {
		sb.WriteString("No runs found in this period\n")
		return sb.String()
	}

	// Header
	fmt.Fprintf(&sb, "Sprint Report  %s – %s\n", formatDate(rpt.Since), formatDate(rpt.Until))
	if rpt.Repo != "" {
		fmt.Fprintf(&sb, "Repo: %s\n", rpt.Repo)
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

	// Resource usage (omitted when no resource data)
	renderResourceSection(&sb, rpt)

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

	// Shipped issues
	if len(rpt.ShippedIssues) > 0 {
		fmt.Fprintf(&sb, "\nWhat was Shipped (%d issues)\n", len(rpt.ShippedIssues))
		for _, si := range rpt.ShippedIssues {
			fmt.Fprintf(&sb, "  #%d — %s\n", si.IssueNumber, si.Title)
		}
	}

	// Period comparison
	if rpt.PriorPeriod != nil && rpt.PriorPeriod.TotalRuns > 0 {
		fmt.Fprintf(&sb, "\nCompared to Prior Period (%s – %s)\n",
			formatDate(rpt.PriorPeriod.Since), formatDate(rpt.PriorPeriod.Until))
		tw = tabwriter.NewWriter(&sb, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "  Success rate\t%.1f%% → %.1f%%\t%s\n",
			rpt.PriorPeriod.SuccessRate*100, rpt.SuccessRate*100,
			formatRateDelta(rpt.SuccessRate-rpt.PriorPeriod.SuccessRate))
		fmt.Fprintf(tw, "  First-pass rate\t%.1f%% → %.1f%%\t%s\n",
			rpt.PriorPeriod.FirstPassRate*100, rpt.FirstPassRate*100,
			formatRateDelta(rpt.FirstPassRate-rpt.PriorPeriod.FirstPassRate))
		fmt.Fprintf(tw, "  Issues processed\t%d → %d\t%s\n",
			rpt.PriorPeriod.IssuesProcessed, rpt.IssuesProcessed,
			formatIntDelta(rpt.IssuesProcessed-rpt.PriorPeriod.IssuesProcessed))
		fmt.Fprintf(tw, "  Total cost\t$%.2f → $%.2f\t%s\n",
			rpt.PriorPeriod.TotalCostUSD, rpt.TotalCostUSD,
			formatCostDelta(rpt.TotalCostUSD-rpt.PriorPeriod.TotalCostUSD))
		fmt.Fprintf(tw, "  Avg per success\t$%.2f → $%.2f\t%s\n",
			rpt.PriorPeriod.AvgCostPerSuccessUSD, rpt.AvgCostPerSuccessUSD,
			formatCostDelta(rpt.AvgCostPerSuccessUSD-rpt.PriorPeriod.AvgCostPerSuccessUSD))
		_ = tw.Flush()
	}

	return sb.String()
}

// RenderMarkdown renders rpt as Markdown suitable for Slack or a wiki.
func RenderMarkdown(rpt SprintReport) string {
	var sb strings.Builder

	if rpt.ExecSummary != "" {
		sb.WriteString("## Executive Summary\n\n")
		sb.WriteString(rpt.ExecSummary)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Sprint Report\n\n")
	fmt.Fprintf(&sb, "**Period:** %s – %s\n\n", formatDate(rpt.Since), formatDate(rpt.Until))
	if rpt.Repo != "" {
		fmt.Fprintf(&sb, "**Repo:** %s\n\n", rpt.Repo)
	}

	if rpt.TotalRuns == 0 {
		sb.WriteString("No runs found in this period.\n")
		return sb.String()
	}

	sb.WriteString("## Summary\n\n")
	fmt.Fprintf(&sb, "- **Runs:** %d\n", rpt.TotalRuns)
	fmt.Fprintf(&sb, "- **Issues processed:** %d\n", rpt.IssuesProcessed)
	fmt.Fprintf(&sb, "- **Implemented:** %d\n", rpt.IssuesImplemented)
	fmt.Fprintf(&sb, "- **Failed:** %d\n", rpt.IssuesFailed)
	fmt.Fprintf(&sb, "- **Success rate:** **%.1f%%**\n", rpt.SuccessRate*100)
	fmt.Fprintf(&sb, "- **First-pass rate:** **%.1f%%**\n", rpt.FirstPassRate*100)

	sb.WriteString("\n## Cost\n\n")
	fmt.Fprintf(&sb, "- **Total:** $%.2f\n", rpt.TotalCostUSD)
	fmt.Fprintf(&sb, "- **Avg per success:** $%.2f\n", rpt.AvgCostPerSuccessUSD)
	fmt.Fprintf(&sb, "- **Wasted:** $%.2f\n", rpt.WastedCostUSD)

	// Resource usage (omitted when no resource data)
	if rpt.PeakMemoryBytes > 0 || rpt.TotalCPUNanoseconds > 0 {
		sb.WriteString("\n## Resource Usage\n\n")
		fmt.Fprintf(&sb, "- **Peak memory:** %.1f MB\n", float64(rpt.PeakMemoryBytes)/1048576)
		fmt.Fprintf(&sb, "- **Total CPU:** %.2f s\n", float64(rpt.TotalCPUNanoseconds)/1e9)
		fmt.Fprintf(&sb, "- **Avg CPU per issue:** %.2f s\n", float64(rpt.AvgCPUNanosecondsPerIssue)/1e9)
	}

	if len(rpt.FailureReasons) > 0 {
		sb.WriteString("\n## Failure Reasons\n\n")
		reasons := sortedKeys(rpt.FailureReasons)
		for _, reason := range reasons {
			fmt.Fprintf(&sb, "- **%s:** %d\n", reason, rpt.FailureReasons[reason])
		}
	}

	if len(rpt.NotableFailures) > 0 {
		sb.WriteString("\n## Notable Failures\n\n")
		sb.WriteString("| Issue | Title | Cost | Error |\n")
		sb.WriteString("|-------|-------|------|-------|\n")
		for _, f := range rpt.NotableFailures {
			title := strings.ReplaceAll(f.Title, "|", "\\|")
			errStr := strings.ReplaceAll(truncate(f.Error, 80), "|", "\\|")
			fmt.Fprintf(&sb, "| #%d | %s | $%.2f | %s |\n", f.IssueNumber, title, f.CostUSD, errStr)
		}
	}

	if len(rpt.ShippedIssues) > 0 {
		fmt.Fprintf(&sb, "\n## What was Shipped (%d issues)\n\n", len(rpt.ShippedIssues))
		for _, si := range rpt.ShippedIssues {
			if si.PRNumber > 0 && si.Repo != "" {
				prURL := fmt.Sprintf("https://github.com/%s/pull/%d", si.Repo, si.PRNumber)
				fmt.Fprintf(&sb, "- [#%d](%s) — %s\n", si.IssueNumber, prURL, si.Title)
			} else {
				fmt.Fprintf(&sb, "- #%d — %s\n", si.IssueNumber, si.Title)
			}
		}
	}

	if rpt.PriorPeriod != nil && rpt.PriorPeriod.TotalRuns > 0 {
		fmt.Fprintf(&sb, "\n## Compared to Prior Period (%s – %s)\n\n",
			formatDate(rpt.PriorPeriod.Since), formatDate(rpt.PriorPeriod.Until))
		sb.WriteString("| Metric | Prior | Current | Delta |\n")
		sb.WriteString("|--------|-------|---------|-------|\n")
		fmt.Fprintf(&sb, "| Success rate | %.1f%% | %.1f%% | %s |\n",
			rpt.PriorPeriod.SuccessRate*100, rpt.SuccessRate*100,
			formatRateDelta(rpt.SuccessRate-rpt.PriorPeriod.SuccessRate))
		fmt.Fprintf(&sb, "| First-pass rate | %.1f%% | %.1f%% | %s |\n",
			rpt.PriorPeriod.FirstPassRate*100, rpt.FirstPassRate*100,
			formatRateDelta(rpt.FirstPassRate-rpt.PriorPeriod.FirstPassRate))
		fmt.Fprintf(&sb, "| Issues processed | %d | %d | %s |\n",
			rpt.PriorPeriod.IssuesProcessed, rpt.IssuesProcessed,
			formatIntDelta(rpt.IssuesProcessed-rpt.PriorPeriod.IssuesProcessed))
		fmt.Fprintf(&sb, "| Total cost | $%.2f | $%.2f | %s |\n",
			rpt.PriorPeriod.TotalCostUSD, rpt.TotalCostUSD,
			formatCostDelta(rpt.TotalCostUSD-rpt.PriorPeriod.TotalCostUSD))
		fmt.Fprintf(&sb, "| Avg per success | $%.2f | $%.2f | %s |\n",
			rpt.PriorPeriod.AvgCostPerSuccessUSD, rpt.AvgCostPerSuccessUSD,
			formatCostDelta(rpt.AvgCostPerSuccessUSD-rpt.PriorPeriod.AvgCostPerSuccessUSD))
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

	if rpt.ExecSummary != "" {
		sb.WriteString(`<div style="background:#f0f7ff;border-left:4px solid #0066cc;padding:16px 20px;margin-bottom:24px;border-radius:4px">
<h2 style="color:#0066cc;margin-top:0">Executive Summary</h2>
`)
		for _, para := range strings.Split(rpt.ExecSummary, "\n\n") {
			para = strings.TrimSpace(para)
			if para != "" {
				fmt.Fprintf(&sb, "<p>%s</p>\n", html.EscapeString(para))
			}
		}
		sb.WriteString("</div>\n")
	}

	sb.WriteString(`<h1 style="color:#1a1a1a">Sprint Report</h1>
`)
	fmt.Fprintf(&sb, `<p><strong>Period:</strong> %s – %s</p>
`, html.EscapeString(formatDate(rpt.Since)), html.EscapeString(formatDate(rpt.Until)))
	if rpt.Repo != "" {
		fmt.Fprintf(&sb, `<p><strong>Repo:</strong> %s</p>
`, html.EscapeString(rpt.Repo))
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

	renderHTMLResourceUsage(&sb, rpt)

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

	renderHTMLNotableFailures(&sb, rpt.NotableFailures)
	renderHTMLShippedIssues(&sb, rpt.ShippedIssues)
	renderHTMLPriorPeriod(&sb, rpt)

	sb.WriteString("</body>\n</html>\n")
	return sb.String()
}

// renderHTMLResourceUsage writes the resource usage table section.
// The section is omitted when no resource data exists.
func renderHTMLResourceUsage(sb *strings.Builder, rpt SprintReport) {
	if rpt.PeakMemoryBytes == 0 && rpt.TotalCPUNanoseconds == 0 {
		return
	}
	sb.WriteString(`<h2 style="color:#333;border-bottom:1px solid #ddd">Resource Usage</h2>
<table style="border-collapse:collapse;width:100%">
`)
	writeHTMLRow(sb, "Peak memory", fmt.Sprintf("%.1f MB", float64(rpt.PeakMemoryBytes)/1048576))
	writeHTMLRow(sb, "Total CPU", fmt.Sprintf("%.2f s", float64(rpt.TotalCPUNanoseconds)/1e9))
	writeHTMLRow(sb, "Avg CPU per issue", fmt.Sprintf("%.2f s", float64(rpt.AvgCPUNanosecondsPerIssue)/1e9))
	sb.WriteString("</table>\n")
}

// renderHTMLNotableFailures writes the notable failures table section.
func renderHTMLNotableFailures(sb *strings.Builder, failures []NotableFailure) {
	if len(failures) == 0 {
		return
	}
	sb.WriteString(`<h2 style="color:#333;border-bottom:1px solid #ddd">Notable Failures</h2>
<table style="border-collapse:collapse;width:100%">
<tr style="background:#f5f5f5">
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Issue</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Title</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Cost</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Error</th>
</tr>
`)
	for _, f := range failures {
		fmt.Fprintf(sb,
			`<tr><td style="padding:8px;border:1px solid #ddd">#%d</td><td style="padding:8px;border:1px solid #ddd">%s</td><td style="padding:8px;border:1px solid #ddd">$%.2f</td><td style="padding:8px;border:1px solid #ddd">%s</td></tr>
`,
			f.IssueNumber,
			html.EscapeString(f.Title),
			f.CostUSD,
			html.EscapeString(truncate(f.Error, 120)),
		)
	}
	sb.WriteString("</table>\n")
}

// renderHTMLShippedIssues writes the shipped issues list section.
func renderHTMLShippedIssues(sb *strings.Builder, issues []ShippedIssue) {
	if len(issues) == 0 {
		return
	}
	fmt.Fprintf(sb, `<h2 style="color:#333;border-bottom:1px solid #ddd">What was Shipped (%d issues)</h2>
<ul>
`, len(issues))
	for _, si := range issues {
		if si.PRNumber > 0 && si.Repo != "" {
			prURL := fmt.Sprintf("https://github.com/%s/pull/%d", si.Repo, si.PRNumber)
			fmt.Fprintf(sb, `<li><a href="%s">#%d</a> — %s</li>
`, html.EscapeString(prURL), si.IssueNumber, html.EscapeString(si.Title))
		} else {
			fmt.Fprintf(sb, "<li>#%d — %s</li>\n", si.IssueNumber, html.EscapeString(si.Title))
		}
	}
	sb.WriteString("</ul>\n")
}

// renderHTMLPriorPeriod writes the period comparison table section.
func renderHTMLPriorPeriod(sb *strings.Builder, rpt SprintReport) {
	if rpt.PriorPeriod == nil || rpt.PriorPeriod.TotalRuns == 0 {
		return
	}
	pp := rpt.PriorPeriod
	fmt.Fprintf(sb, `<h2 style="color:#333;border-bottom:1px solid #ddd">Compared to Prior Period (%s – %s)</h2>
<table style="border-collapse:collapse;width:100%%">
<tr style="background:#f5f5f5">
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Metric</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Prior</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Current</th>
  <th style="text-align:left;padding:8px;border:1px solid #ddd">Delta</th>
</tr>
`, html.EscapeString(formatDate(pp.Since)), html.EscapeString(formatDate(pp.Until)))
	writeHTMLDeltaRow(sb, "Success rate",
		fmt.Sprintf("%.1f%%", pp.SuccessRate*100),
		fmt.Sprintf("%.1f%%", rpt.SuccessRate*100),
		formatRateDelta(rpt.SuccessRate-pp.SuccessRate),
		rpt.SuccessRate >= pp.SuccessRate)
	writeHTMLDeltaRow(sb, "First-pass rate",
		fmt.Sprintf("%.1f%%", pp.FirstPassRate*100),
		fmt.Sprintf("%.1f%%", rpt.FirstPassRate*100),
		formatRateDelta(rpt.FirstPassRate-pp.FirstPassRate),
		rpt.FirstPassRate >= pp.FirstPassRate)
	writeHTMLDeltaRow(sb, "Issues processed",
		fmt.Sprintf("%d", pp.IssuesProcessed),
		fmt.Sprintf("%d", rpt.IssuesProcessed),
		formatIntDelta(rpt.IssuesProcessed-pp.IssuesProcessed),
		rpt.IssuesProcessed >= pp.IssuesProcessed)
	writeHTMLDeltaRow(sb, "Total cost",
		fmt.Sprintf("$%.2f", pp.TotalCostUSD),
		fmt.Sprintf("$%.2f", rpt.TotalCostUSD),
		formatCostDelta(rpt.TotalCostUSD-pp.TotalCostUSD),
		rpt.TotalCostUSD <= pp.TotalCostUSD)
	writeHTMLDeltaRow(sb, "Avg per success",
		fmt.Sprintf("$%.2f", pp.AvgCostPerSuccessUSD),
		fmt.Sprintf("$%.2f", rpt.AvgCostPerSuccessUSD),
		formatCostDelta(rpt.AvgCostPerSuccessUSD-pp.AvgCostPerSuccessUSD),
		rpt.AvgCostPerSuccessUSD <= pp.AvgCostPerSuccessUSD)
	sb.WriteString("</table>\n")
}

// writeHTMLRow writes a two-column table row with inline styles.
func writeHTMLRow(sb *strings.Builder, label, value string) {
	fmt.Fprintf(sb,
		`<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold">%s</td><td style="padding:8px;border:1px solid #ddd">%s</td></tr>
`,
		html.EscapeString(label),
		html.EscapeString(value),
	)
}

// writeHTMLDeltaRow writes a four-column comparison row with a color-coded delta cell.
// positive=true means the change is beneficial (rendered green), false means red.
func writeHTMLDeltaRow(sb *strings.Builder, label, prior, current, delta string, positive bool) {
	color := "#cc0000"
	if positive {
		color = "#007700"
	}
	fmt.Fprintf(sb,
		`<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold">%s</td><td style="padding:8px;border:1px solid #ddd">%s</td><td style="padding:8px;border:1px solid #ddd">%s</td><td style="padding:8px;border:1px solid #ddd;color:%s;font-weight:bold">%s</td></tr>
`,
		html.EscapeString(label),
		html.EscapeString(prior),
		html.EscapeString(current),
		color,
		html.EscapeString(delta),
	)
}

// formatRateDelta formats a rate delta (0.0–1.0 scale) as "+12.5%" or "-3.0%".
func formatRateDelta(delta float64) string {
	if delta >= 0 {
		return fmt.Sprintf("+%.1f%%", delta*100)
	}
	return fmt.Sprintf("%.1f%%", delta*100)
}

// formatCostDelta formats a cost delta as "+$1.30" or "-$4.20".
func formatCostDelta(delta float64) string {
	if delta >= 0 {
		return fmt.Sprintf("+$%.2f", delta)
	}
	return fmt.Sprintf("-$%.2f", -delta)
}

// formatIntDelta formats an integer delta as "+7" or "-3".
func formatIntDelta(delta int) string {
	if delta >= 0 {
		return fmt.Sprintf("+%d", delta)
	}
	return fmt.Sprintf("%d", delta)
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
