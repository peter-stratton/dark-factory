package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
	"github.com/spf13/cobra"
)

// newTraceReader is a testability seam: replaced in tests to inject a custom Reader.
var newTraceReader = func() (*rundata.Reader, error) {
	return rundata.NewReader(nil)
}

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
		detail, _ := cmd.Flags().GetBool("detail")

		return runTrace(cmd.OutOrStdout(), db, args[0], repo, runID, jsonOut, detail)
	},
}

func init() {
	f := traceCmd.Flags()
	f.String("repo", "", "Filter to a specific repository (owner/repo)")
	f.String("run", "", "Filter to a specific run ID")
	f.Bool("json", false, "Output as JSON")
	f.Bool("detail", false, "Show full decision chain")
	rootCmd.AddCommand(traceCmd)
}

func runTrace(w io.Writer, db *stats.DB, arg string, repo string, runID string, jsonOut bool, detail bool) error {
	if detail && jsonOut {
		return fmt.Errorf("--detail and --json are mutually exclusive")
	}

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

	if detail {
		return renderTraceDetailFromDB(ctx, w, db, traceID, issueNumber, outcome, steps)
	}

	return renderTraceText(w, traceID, issueNumber, outcome, steps)
}

// renderTraceDetailFromDB loads the full run data from disk and renders the
// detailed decision chain. Falls back to renderTraceText when run data is
// not available on disk.
func renderTraceDetailFromDB(ctx context.Context, w io.Writer, db *stats.DB, traceID string, issueNumber int, outcome *stats.IssueOutcomeRecord, steps []stats.StepResultRecord) error {
	runRecord, err := stats.QueryRunByID(ctx, db, outcome.RunID)
	if err != nil {
		return fmt.Errorf("querying run: %w", err)
	}
	if runRecord == nil {
		fmt.Fprintln(w, "Run data not found on disk; showing summary from stats.db")
		return renderTraceText(w, traceID, issueNumber, outcome, steps)
	}

	parts := strings.SplitN(runRecord.Repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("malformed repo in run record: %q", runRecord.Repo)
	}
	owner, repoName := parts[0], parts[1]

	reader, err := newTraceReader()
	if err != nil {
		return fmt.Errorf("creating run reader: %w", err)
	}

	runDetail, err := reader.LoadRun(owner, repoName, runRecord.ID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(w, "Run data not found on disk; showing summary from stats.db")
			return renderTraceText(w, traceID, issueNumber, outcome, steps)
		}
		return fmt.Errorf("loading run data: %w", err)
	}

	var issueDetail *rundata.IssueDetail
	for i := range runDetail.Issues {
		if runDetail.Issues[i].IssueNumber == issueNumber {
			issueDetail = &runDetail.Issues[i]
			break
		}
	}
	if issueDetail == nil {
		fmt.Fprintln(w, "Run data not found on disk; showing summary from stats.db")
		return renderTraceText(w, traceID, issueNumber, outcome, steps)
	}

	return renderTraceDetail(w, traceID, issueNumber, outcome, *issueDetail)
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

// renderTraceDetail renders the full decision chain for a single issue.
func renderTraceDetail(w io.Writer, traceID string, issueNumber int, outcome *stats.IssueOutcomeRecord, detail rundata.IssueDetail) error {
	fmt.Fprintf(w, "Trace: %s\n", traceID)
	fmt.Fprintf(w, "Issue: #%d\n\n", issueNumber)

	renderStepSection(w, "RECON", detail.Recon)
	renderStepSection(w, "IMPLEMENT", detail.Implement)

	// Verify results (initial, before retries)
	for _, vr := range detail.VerifyResults {
		renderVerifySection(w, vr)
	}

	// Retries in chronological order with inline dialogue
	for _, retry := range detail.Retries {
		renderRetrySection(w, retry, detail.Dialogue)
	}

	renderStepSection(w, "QUALITY REVIEW", detail.QualityReview)
	renderStepSection(w, "FUNCTIONAL REVIEW", detail.FunctionalReview)

	// Risk assessment
	if detail.RiskAssessment != nil {
		renderRiskSection(w, detail.RiskAssessment)
	}

	// Judge interventions
	for _, ji := range detail.JudgeInterventions {
		fmt.Fprintf(w, "=== JUDGE INTERVENTION ===\n")
		fmt.Fprintf(w, "Rule: %s\n", ji.Rule)
		fmt.Fprintf(w, "Judgment: %s\n", ji.Judgment)
		fmt.Fprintf(w, "Detail: %s\n", ji.Detail)
		fmt.Fprintf(w, "Step: %s\n\n", ji.Step)
	}

	// Dialogue entries not covered by retries (round 0 or unmatched)
	for _, d := range detail.Dialogue {
		if d.Round == 0 {
			fmt.Fprintf(w, "=== DIALOGUE ===\n")
			fmt.Fprintf(w, "Role: %s\n", d.Role)
			if d.Outcome != "" {
				fmt.Fprintf(w, "Outcome: %s\n", d.Outcome)
			}
			fmt.Fprintf(w, "Body: %s\n\n", truncateString(d.Body, 500))
		}
	}

	// Outcome
	fmt.Fprintf(w, "=== OUTCOME ===\n")
	fmt.Fprintf(w, "Status: %s\n", outcome.Status)
	if outcome.PRNumber > 0 {
		fmt.Fprintf(w, "PR: #%d\n", outcome.PRNumber)
	}
	if outcome.Error != "" {
		fmt.Fprintf(w, "Error: %s\n", outcome.Error)
	}

	return nil
}

// renderStepSection renders a single step with prompt, output, tool trace, and flags.
func renderStepSection(w io.Writer, name string, step rundata.StepResult) {
	fmt.Fprintf(w, "=== %s (%s, $%.4f) ===\n", name,
		traceFormatDuration(step.DurationSeconds), step.CostUSD)

	if step.Prompt != "" {
		fmt.Fprintf(w, "Prompt: %s\n", truncateLines(step.Prompt, 3))
	} else {
		fmt.Fprintln(w, "Prompt: [not captured]")
	}

	if step.Output != "" {
		fmt.Fprintf(w, "Output: %s\n", truncateString(step.Output, 500))
	} else {
		fmt.Fprintln(w, "Output: [empty]")
	}

	fmt.Fprintf(w, "Tool trace: %d calls\n", len(step.ToolTrace))

	if len(step.Flags) > 0 {
		codes := make([]string, len(step.Flags))
		for i, f := range step.Flags {
			codes[i] = f.Code
		}
		fmt.Fprintf(w, "Flags: %s\n", strings.Join(codes, ", "))
	} else {
		fmt.Fprintln(w, "Flags: none")
	}
	fmt.Fprintln(w)
}

// renderVerifySection renders a single verify attempt.
func renderVerifySection(w io.Writer, vr rundata.VerifyStepResult) {
	fmt.Fprintf(w, "=== VERIFY (attempt %d) ===\n", vr.Attempt)
	if len(vr.Checks) > 0 {
		parts := make([]string, len(vr.Checks))
		for i, c := range vr.Checks {
			result := "FAIL"
			if c.Passed {
				result = "PASS"
			}
			parts[i] = fmt.Sprintf("%s %s", c.Name, result)
		}
		fmt.Fprintf(w, "Checks: %s\n", strings.Join(parts, ", "))
	}
	fixStr := "no"
	if vr.FixAttempted {
		fixStr = "yes"
	}
	fmt.Fprintf(w, "Fix attempted: %s\n\n", fixStr)
}

// renderRetrySection renders a retry with its reviews, inserting dialogue for that round.
func renderRetrySection(w io.Writer, retry rundata.RetryDetail, dialogue []rundata.DialogueEntry) {
	fmt.Fprintf(w, "=== RETRY %d (%s, $%.4f) ===\n", retry.Attempt,
		traceFormatDuration(retry.Retry.DurationSeconds), retry.Retry.CostUSD)

	if retry.Retry.Prompt != "" {
		fmt.Fprintf(w, "Prompt: %s\n", truncateLines(retry.Retry.Prompt, 3))
	} else {
		fmt.Fprintln(w, "Prompt: [not captured]")
	}

	if retry.Retry.Output != "" {
		fmt.Fprintf(w, "Output: %s\n", truncateString(retry.Retry.Output, 500))
	} else {
		fmt.Fprintln(w, "Output: [empty]")
	}
	fmt.Fprintln(w)

	// Inline dialogue for this round
	for _, d := range dialogue {
		if d.Round == retry.Attempt {
			fmt.Fprintf(w, "--- Dialogue (%s) ---\n", d.Role)
			if d.Outcome != "" {
				fmt.Fprintf(w, "Outcome: %s\n", d.Outcome)
			}
			fmt.Fprintf(w, "Body: %s\n\n", truncateString(d.Body, 500))
		}
	}

	renderStepSection(w, fmt.Sprintf("RETRY %d QUALITY REVIEW", retry.Attempt), retry.QualityReview)
	renderStepSection(w, fmt.Sprintf("RETRY %d FUNCTIONAL REVIEW", retry.Attempt), retry.FunctionalReview)
}

// renderRiskSection renders the risk assessment gates.
func renderRiskSection(w io.Writer, ra *rundata.RiskAssessment) {
	fmt.Fprintf(w, "=== RISK ASSESSMENT ===\n")
	lowRisk := "no"
	if ra.IsLowRisk {
		lowRisk = "yes"
	}
	fmt.Fprintf(w, "Low risk: %s\n", lowRisk)
	if len(ra.Gates) > 0 {
		parts := make([]string, len(ra.Gates))
		for i, g := range ra.Gates {
			result := "FAIL"
			if g.Passed {
				result = "PASS"
			}
			parts[i] = fmt.Sprintf("%s %s", g.Name, result)
		}
		fmt.Fprintf(w, "Gates: %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintln(w)
}

// truncateLines returns the first n lines of s.
func truncateLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// truncateString returns the first n characters of s, appending "..." if truncated.
func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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
