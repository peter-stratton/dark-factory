package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/phs/dark-factory/internal/analysis"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/stats"
	"github.com/spf13/cobra"
)

// newAnalyzeReader is a testability seam: replaced in tests to inject a custom reader.
var newAnalyzeReader = func(logger *slog.Logger) (*rundata.Reader, error) {
	return rundata.NewReader(logger)
}

// newAnalyzeDB is a testability seam: replaced in tests to inject a custom DB.
var newAnalyzeDB = func() (*stats.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	return stats.Open(filepath.Join(home, ".godark", "stats.db"))
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze run data and print aggregate report",
	Long: `Read run data from the stats database (~/.godark/stats.db) by default,
apply filters, and print an aggregate report including outcome distribution,
flag frequencies, retry statistics, cost statistics, and detected prompt gaps.

Use --legacy to fall back to scanning run directories (~/.godark/runs/) instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		milestone, _ := cmd.Flags().GetString("milestone")
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		jsonOut, _ := cmd.Flags().GetBool("json")
		legacy, _ := cmd.Flags().GetBool("legacy")

		since, until, err := parseDateRange(sinceStr, untilStr)
		if err != nil {
			return err
		}

		if legacy {
			reader, err := newAnalyzeReader(nil)
			if err != nil {
				return fmt.Errorf("creating reader: %w", err)
			}
			return runAnalyze(cmd.OutOrStdout(), reader, slog.Default(), repo, milestone, since, until, jsonOut)
		}

		db, err := newAnalyzeDB()
		if err != nil {
			return fmt.Errorf("opening stats database: %w", err)
		}
		defer db.Close()

		return runAnalyzeDB(cmd.OutOrStdout(), db, repo, milestone, since, until, jsonOut)
	},
}

func init() {
	f := analyzeCmd.Flags()
	f.String("repo", "", "Filter to runs for this repository (owner/repo)")
	f.String("milestone", "", "Filter to runs for this milestone")
	f.String("since", "", "Only include runs started at or after this date (RFC 3339 or YYYY-MM-DD)")
	f.String("until", "", "Only include runs started at or before this date (RFC 3339 or YYYY-MM-DD)")
	f.Bool("json", false, "Output as JSON instead of human-readable table")
	f.Bool("legacy", false, "Fall back to filesystem scan (~/.godark/runs/) instead of the stats database")
	rootCmd.AddCommand(analyzeCmd)
}

// runAnalyzeDB is the DB-backed analysis path. It queries the stats database,
// converts records to RunDetail, and writes the report to w.
func runAnalyzeDB(w io.Writer, db *stats.DB, repo, milestone string, since, until *time.Time, jsonOut bool) error {
	filter := stats.RunFilter{
		Repo:      repo,
		Milestone: milestone,
	}
	if since != nil {
		filter.Since = *since
	}
	if until != nil {
		filter.Until = *until
	}

	ctx := context.Background()

	runs, err := stats.QueryRuns(ctx, db, filter)
	if err != nil {
		return fmt.Errorf("querying runs: %w", err)
	}
	if len(runs) == 0 {
		fmt.Fprintln(w, "No runs found")
		return nil
	}

	outcomes, err := stats.QueryIssueOutcomes(ctx, db, filter)
	if err != nil {
		return fmt.Errorf("querying issue outcomes: %w", err)
	}

	steps, err := stats.QueryStepResults(ctx, db, filter)
	if err != nil {
		return fmt.Errorf("querying step results: %w", err)
	}

	runDetails := stats.ToRunDetails(runs, outcomes, steps)

	report := analysis.Aggregate(runDetails)
	gaps := analysis.DetectGaps(runDetails)

	if jsonOut {
		return printAnalyzeJSON(w, report, gaps)
	}
	printAnalyzeReport(w, report, gaps)
	return nil
}

// runAnalyze is the core logic for the analyze command (legacy filesystem path).
// It reads runs from reader, applies filters, and writes the report to w.
// If logger is nil, slog.Default() is used.
func runAnalyze(w io.Writer, reader *rundata.Reader, logger *slog.Logger, repo, milestone string, since, until *time.Time, jsonOut bool) error {
	if logger == nil {
		logger = slog.Default()
	}

	metas, err := reader.ListRuns()
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	filtered := filterRunMetas(metas, repo, milestone, since, until)
	if len(filtered) == 0 {
		fmt.Fprintln(w, "No matching runs found")
		return nil
	}

	runs := make([]rundata.RunDetail, 0, len(filtered))
	for _, meta := range filtered {
		owner, repoName, err := splitRepo(meta.Repo)
		if err != nil {
			logger.Warn("skipping run: invalid repo format", "repo", meta.Repo, "error", err)
			continue
		}
		timestamp := meta.StartedAt.UTC().Format("20060102-150405")
		detail, err := reader.LoadRun(owner, repoName, timestamp)
		if err != nil {
			logger.Warn("skipping run: cannot load run data", "repo", meta.Repo, "timestamp", timestamp, "error", err)
			continue
		}
		runs = append(runs, *detail)
	}

	if len(runs) == 0 {
		fmt.Fprintln(w, "No matching runs found")
		return nil
	}

	report := analysis.Aggregate(runs)
	gaps := analysis.DetectGaps(runs)

	if jsonOut {
		return printAnalyzeJSON(w, report, gaps)
	}
	printAnalyzeReport(w, report, gaps)
	return nil
}

// parseDateRange parses sinceStr and untilStr into time.Time pointers.
// Accepts RFC 3339 and YYYY-MM-DD formats. Returns nil for empty strings.
func parseDateRange(sinceStr, untilStr string) (*time.Time, *time.Time, error) {
	since, err := parseDate(sinceStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing --since: %w", err)
	}
	until, err := parseDate(untilStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing --until: %w", err)
	}
	return since, until, nil
}

// parseDate parses a date string in RFC 3339 or YYYY-MM-DD format.
// Returns nil for empty strings.
func parseDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	// Try RFC 3339 first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	// Try YYYY-MM-DD (start of day UTC).
	if t, err := time.Parse("2006-01-02", s); err == nil {
		t = t.UTC()
		return &t, nil
	}
	return nil, fmt.Errorf("unrecognised date format %q: use RFC 3339 or YYYY-MM-DD", s)
}

// filterRunMetas returns the subset of metas matching the given filters.
// Empty filter strings match all values. since and until are inclusive.
func filterRunMetas(metas []rundata.RunMeta, repo, milestone string, since, until *time.Time) []rundata.RunMeta {
	var result []rundata.RunMeta
	for _, m := range metas {
		if repo != "" && m.Repo != repo {
			continue
		}
		if milestone != "" && m.Milestone != milestone {
			continue
		}
		if since != nil && m.StartedAt.Before(*since) {
			continue
		}
		if until != nil && m.StartedAt.After(*until) {
			continue
		}
		result = append(result, m)
	}
	return result
}

// splitRepo splits "owner/repo" into its two components.
func splitRepo(repo string) (owner, name string, err error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q: expected owner/name format", repo)
	}
	return parts[0], parts[1], nil
}

// analyzeOutput is the JSON representation of the full analyze report.
type analyzeOutput struct {
	Report analysis.Report      `json:"report"`
	Gaps   []analysis.PromptGap `json:"gaps"`
}

// printAnalyzeJSON marshals the report and gaps to JSON and writes to w.
func printAnalyzeJSON(w io.Writer, report analysis.Report, gaps []analysis.PromptGap) error {
	out := analyzeOutput{Report: report, Gaps: gaps}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// printAnalyzeReport writes a human-readable report to w.
func printAnalyzeReport(w io.Writer, report analysis.Report, gaps []analysis.PromptGap) {
	fmt.Fprintf(w, "Analyzed %d runs, %d issues\n", report.RunCount, report.IssueCount)

	// Outcome table.
	fmt.Fprintf(w, "\nOutcomes\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Status\tCount\tPercent\n")
	statuses := make([]string, 0, len(report.Outcomes))
	for status := range report.Outcomes {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		count := report.Outcomes[status]
		var pct float64
		if report.IssueCount > 0 {
			pct = float64(count) / float64(report.IssueCount) * 100
		}
		fmt.Fprintf(tw, "  %s\t%d\t%.1f%%\n", status, count, pct)
	}
	_ = tw.Flush()

	// Flag frequency table.
	fmt.Fprintf(w, "\nFlag Frequencies\n")
	if len(report.FlagFrequencies) == 0 {
		fmt.Fprintf(w, "  (none)\n")
	} else {
		tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "  Code\tCount\tPercent\n")
		for _, ff := range report.FlagFrequencies {
			fmt.Fprintf(tw, "  %s\t%d\t%.1f%%\n", ff.Code, ff.Count, ff.Percent)
		}
		_ = tw.Flush()
	}

	// Retry stats.
	fmt.Fprintf(w, "\nRetry Stats\n")
	fmt.Fprintf(w, "  Avg per issue:  %.2f\n", report.RetryStats.AvgPerIssue)
	fmt.Fprintf(w, "  Max retries:    %d\n", report.RetryStats.MaxRetries)
	fmt.Fprintf(w, "  Exhausted:      %d\n", report.RetryStats.ExhaustedCount)
	fmt.Fprintf(w, "  Recovery rate:  %.1f%%\n", report.RetryStats.RecoveryRate*100)

	// Cost stats.
	fmt.Fprintf(w, "\nCost Stats\n")
	fmt.Fprintf(w, "  Total:          $%.2f\n", report.CostStats.TotalUSD)
	fmt.Fprintf(w, "  Per issue:      $%.2f\n", report.CostStats.AvgPerIssueUSD)
	fmt.Fprintf(w, "  Per run:        $%.2f\n", report.CostStats.AvgPerRunUSD)

	printCostByStep(w, report)

	// Duration stats.
	fmt.Fprintf(w, "\nDuration Stats\n")
	fmt.Fprintf(w, "  Avg implement duration: %s\n", formatDuration(report.DurationStats.AvgImplementSeconds))
	fmt.Fprintf(w, "  Avg review duration:    %s\n", formatDuration(report.DurationStats.AvgReviewSeconds))

	printRepoStats(w, report)
	printVerifyStats(w, report)
	printPromptGaps(w, gaps)
}

// printCostByStep writes the "Cost by Step" section to w.
func printCostByStep(w io.Writer, report analysis.Report) {
	fmt.Fprintf(w, "\nCost by Step\n")
	if len(report.CostStats.CostByStep) == 0 {
		fmt.Fprintf(w, "  (no cost data)\n")
		return
	}
	type stepCost struct {
		name string
		cost float64
	}
	steps := make([]stepCost, 0, len(report.CostStats.CostByStep))
	for name, cost := range report.CostStats.CostByStep {
		if cost > 0 {
			steps = append(steps, stepCost{name, cost})
		}
	}
	sort.Slice(steps, func(i, j int) bool {
		if steps[i].cost != steps[j].cost {
			return steps[i].cost > steps[j].cost
		}
		return steps[i].name < steps[j].name
	})
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Step\tTotal Cost\tPercent\n")
	for _, s := range steps {
		var pct float64
		if report.CostStats.TotalUSD > 0 {
			pct = s.cost / report.CostStats.TotalUSD * 100
		}
		fmt.Fprintf(tw, "  %s\t$%.4f\t%.1f%%\n", s.name, s.cost, pct)
	}
	_ = tw.Flush()
}

// printRepoStats writes the "Success by Repo" section to w.
func printRepoStats(w io.Writer, report analysis.Report) {
	if len(report.RepoStats) == 0 {
		return
	}
	fmt.Fprintf(w, "\nSuccess by Repo\n")
	repoNames := make([]string, 0, len(report.RepoStats))
	for repo := range report.RepoStats {
		repoNames = append(repoNames, repo)
	}
	sort.Strings(repoNames)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Repo\tTotal\tImplemented\tFailed\tSuccess Rate\n")
	for _, repo := range repoNames {
		rs := report.RepoStats[repo]
		fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\t%.1f%%\n", repo, rs.Total, rs.Implemented, rs.Failed, rs.SuccessRate*100)
	}
	_ = tw.Flush()
}

// printVerifyStats writes the "Verify Check Failures" section to w.
func printVerifyStats(w io.Writer, report analysis.Report) {
	if len(report.VerifyStats) == 0 {
		return
	}
	fmt.Fprintf(w, "\nVerify Check Failures\n")
	checkNames := make([]string, 0, len(report.VerifyStats))
	for check := range report.VerifyStats {
		checkNames = append(checkNames, check)
	}
	sort.Strings(checkNames)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  Check\tFailures\tFailure Rate\n")
	for _, check := range checkNames {
		count := report.VerifyStats[check]
		var pct float64
		if report.IssueCount > 0 {
			pct = float64(count) / float64(report.IssueCount) * 100
		}
		fmt.Fprintf(tw, "  %s\t%d\t%.1f%%\n", check, count, pct)
	}
	_ = tw.Flush()
}

// printPromptGaps writes the "Prompt Gaps" section to w.
func printPromptGaps(w io.Writer, gaps []analysis.PromptGap) {
	fmt.Fprintf(w, "\nPrompt Gaps\n")
	if len(gaps) == 0 {
		fmt.Fprintf(w, "  (none detected)\n")
		return
	}
	for _, g := range gaps {
		fmt.Fprintf(w, "  %s\n", g.Finding)
		fmt.Fprintf(w, "    fail rate with: %.1f%% (%d samples)\n", g.FailRateWith*100, g.SamplesWith)
		fmt.Fprintf(w, "    fail rate without: %.1f%% (%d samples)\n", g.FailRateWithout*100, g.SamplesWithout)
	}
}

// formatDuration formats a duration in seconds as a human-readable string (e.g. "5m00s").
func formatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}
