package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/phs/dark-factory/internal/stats"
	"github.com/spf13/cobra"
)

// openStatsDBAt checks that dbPath exists and opens it.
// Extracted for testability so tests can exercise the real error message.
func openStatsDBAt(dbPath string) (*stats.DB, error) {
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("No stats database found. Run `godark run` or `godark implement` first.")
	}
	return stats.Open(dbPath)
}

// newReportDB is a testability seam: replaced in tests to inject a custom DB.
var newReportDB = func() (*stats.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	return openStatsDBAt(filepath.Join(home, ".godark", "stats.db"))
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a sprint-scoped summary from the stats database",
	Long: `Read run data from the stats database (~/.godark/stats.db),
apply a date range and optional repo filter, and print a sprint summary report.

The --since flag accepts a duration string (e.g. "2w" for 2 weeks, "30d" for 30 days).
The --until flag accepts a date string (RFC 3339 or YYYY-MM-DD), defaulting to now.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sinceStr, _ := cmd.Flags().GetString("since")
		untilStr, _ := cmd.Flags().GetString("until")
		repo, _ := cmd.Flags().GetString("repo")
		format, _ := cmd.Flags().GetString("format")

		switch format {
		case "terminal", "markdown", "html":
			// valid
		default:
			return fmt.Errorf("invalid --format %q: must be terminal, markdown, or html", format)
		}

		// Resolve until time.
		var untilTime time.Time
		if untilStr == "" {
			untilTime = time.Now()
		} else {
			t, err := parseDate(untilStr)
			if err != nil {
				return fmt.Errorf("parsing --until: %w", err)
			}
			untilTime = *t
		}

		// Parse --since as a duration and compute the since time.
		duration, err := parseSinceDuration(sinceStr)
		if err != nil {
			return fmt.Errorf("parsing --since: %w", err)
		}
		sinceTime := untilTime.Add(-duration)

		db, err := newReportDB()
		if err != nil {
			return err
		}
		defer db.Close()

		return runReport(cmd.OutOrStdout(), db, repo, sinceTime, untilTime, format)
	},
}

func init() {
	f := reportCmd.Flags()
	f.String("since", "2w", `Duration before --until to start report window (e.g. 2w, 30d, 7d)`)
	f.String("until", "", "End of report window (RFC 3339 or YYYY-MM-DD; default now)")
	f.String("repo", "", "Filter to runs for this repository (owner/repo)")
	f.String("format", "terminal", "Output format: terminal, markdown, or html")
	rootCmd.AddCommand(reportCmd)
}

// runReport queries the stats database and renders a report.
func runReport(w io.Writer, db *stats.DB, repo string, since, until time.Time, format string) error {
	filter := stats.RunFilter{
		Repo:  repo,
		Since: since,
		Until: until,
	}

	ctx := context.Background()

	runs, err := stats.QueryRuns(ctx, db, filter)
	if err != nil {
		return fmt.Errorf("querying runs: %w", err)
	}

	outcomes, err := stats.QueryIssueOutcomes(ctx, db, filter)
	if err != nil {
		return fmt.Errorf("querying issue outcomes: %w", err)
	}

	steps, err := stats.QueryStepResults(ctx, db, filter)
	if err != nil {
		return fmt.Errorf("querying step results: %w", err)
	}

	return renderReport(w, format, runs, outcomes, steps)
}

// renderReport is a stub renderer. Actual rendering is implemented in a later issue.
func renderReport(_ io.Writer, _ string, _ []stats.RunRecord, _ []stats.IssueOutcomeRecord, _ []stats.StepResultRecord) error {
	return nil
}

// parseSinceDuration parses a duration string with "d" (days) or "w" (weeks) suffix.
// Examples: "2w" → 14 days, "30d" → 30 days, "7d" → 7 days.
func parseSinceDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q: expected format like 2w or 30d", s)
	}
	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid duration %q: expected format like 2w or 30d", s)
	}
	switch suffix {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration %q: expected format like 2w or 30d", s)
	}
}
