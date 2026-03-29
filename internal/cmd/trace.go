package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/peter-stratton/dark-factory/internal/stats"
	"github.com/spf13/cobra"
)

// newTraceDB is a testability seam: replaced in tests to inject a custom DB.
var newTraceDB = func() (*stats.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	return openStatsDBAt(filepath.Join(home, ".godark", "stats.db"))
}

var traceCmd = &cobra.Command{
	Use:   "trace <issue-number|trace-id>",
	Short: "Show the decision flow for an issue trace",
	Long: `Query stats.db and render the full decision flow for an issue.

Accepts either an issue number (resolves to the most recent trace) or a
trace ID directly. Outputs a structured timeline showing every stage, its
duration, cost, outcome, and flags.

When an issue number is provided, --repo and --run can narrow which trace
is selected. When a trace ID is provided, --repo and --run are ignored.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := newTraceDB()
		if err != nil {
			return err
		}
		defer db.Close()

		repo, _ := cmd.Flags().GetString("repo")
		runID, _ := cmd.Flags().GetString("run")
		jsonOut, _ := cmd.Flags().GetBool("json")

		return runTrace(cmd.OutOrStdout(), db, args[0], repo, runID, jsonOut)
	},
}

func init() {
	f := traceCmd.Flags()
	f.String("repo", "", "Filter to a specific repository (owner/repo)")
	f.String("run", "", "Filter to a specific run ID")
	f.Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(traceCmd)
}

func runTrace(w io.Writer, db *stats.DB, arg string, repo string, runID string, jsonOut bool) error {
	ctx := context.Background()

	var traceID string
	var issueNumber int

	if n, err := strconv.Atoi(arg); err == nil {
		issueNumber = n
		tid, qerr := stats.QueryLatestTraceForIssue(ctx, db, n, repo, runID)
		if qerr != nil {
			return fmt.Errorf("looking up trace for issue #%d: %w", n, qerr)
		}
		if tid == "" {
			return fmt.Errorf("no trace found for issue #%d", n)
		}
		traceID = tid
	} else {
		traceID = arg
	}

	outcome, err := stats.QueryOutcomeByTraceID(ctx, db, traceID)
	if err != nil {
		return fmt.Errorf("querying outcome: %w", err)
	}
	if outcome == nil {
		return fmt.Errorf("no trace found for trace ID %s", traceID)
	}

	steps, err := stats.QueryStepsByTraceID(ctx, db, traceID)
	if err != nil {
		return fmt.Errorf("querying steps: %w", err)
	}

	if issueNumber == 0 {
		issueNumber = outcome.IssueNumber
	}

	if jsonOut {
		return renderTraceJSON(w, traceID, issueNumber, outcome, steps)
	}
	return renderTraceText(w, traceID, issueNumber, outcome, steps)
}

type traceJSON struct {
	TraceID     string         `json:"trace_id"`
	IssueNumber int            `json:"issue_number"`
	Outcome     outcomeJSON    `json:"outcome"`
	Steps       []stepJSON     `json:"steps"`
}

type outcomeJSON struct {
	Status   string `json:"status"`
	PRNumber int    `json:"pr_number"`
	Error    string `json:"error,omitempty"`
	CloneSHA string `json:"clone_sha,omitempty"`
}

type stepJSON struct {
	StepName        string   `json:"step_name"`
	DurationSeconds float64  `json:"duration_seconds"`
	CostUSD         float64  `json:"cost_usd"`
	StartedAt       string   `json:"started_at"`
	Flags           []string `json:"flags"`
}

func renderTraceJSON(w io.Writer, traceID string, issueNumber int, outcome *stats.IssueOutcomeRecord, steps []stats.StepResultRecord) error {
	data := traceJSON{
		TraceID:     traceID,
		IssueNumber: issueNumber,
		Outcome: outcomeJSON{
			Status:   outcome.Status,
			PRNumber: outcome.PRNumber,
			Error:    outcome.Error,
			CloneSHA: outcome.CloneSHA,
		},
		Steps: make([]stepJSON, 0, len(steps)),
	}
	for _, s := range steps {
		data.Steps = append(data.Steps, stepJSON{
			StepName:        s.StepName,
			DurationSeconds: s.DurationSeconds,
			CostUSD:         s.CostUSD,
			StartedAt:       s.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Flags:           s.Flags,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

func renderTraceText(w io.Writer, traceID string, issueNumber int, outcome *stats.IssueOutcomeRecord, steps []stats.StepResultRecord) error {
	fmt.Fprintf(w, "Trace: %s\n", traceID)
	fmt.Fprintf(w, "Issue: #%d\n", issueNumber)
	fmt.Fprintf(w, "Status: %s\n", outcome.Status)
	if outcome.PRNumber > 0 {
		fmt.Fprintf(w, "PR: #%d\n", outcome.PRNumber)
	}
	if outcome.Error != "" {
		fmt.Fprintf(w, "Error: %s\n", outcome.Error)
	}
	if outcome.CloneSHA != "" {
		fmt.Fprintf(w, "Clone SHA: %s\n", outcome.CloneSHA)
	}
	fmt.Fprintln(w)

	if len(steps) == 0 {
		fmt.Fprintln(w, "No steps recorded.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Step\tDuration\tCost\tStarted\tFlags\n")
	for _, s := range steps {
		flags := ""
		if len(s.Flags) > 0 {
			flags = strings.Join(s.Flags, ", ")
		}
		fmt.Fprintf(tw, "%s\t%s\t$%.4f\t%s\t%s\n",
			s.StepName,
			traceFormatDuration(s.DurationSeconds),
			s.CostUSD,
			s.StartedAt.UTC().Format("2006-01-02 15:04:05"),
			flags,
		)
	}
	return tw.Flush()
}

// traceFormatDuration formats a duration in seconds as a human-readable string.
func traceFormatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}
