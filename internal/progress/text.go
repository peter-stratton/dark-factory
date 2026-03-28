package progress

import (
	"fmt"
	"io"
	"os"
)

// TextReporter implements ProgressReporter by writing plain text to an io.Writer.
// Its format strings match the current fmt.Printf calls in orchestrator.go exactly.
type TextReporter struct {
	w io.Writer
}

// NewTextReporter returns a TextReporter that writes to w.
func NewTextReporter(w io.Writer) *TextReporter {
	return &TextReporter{w: w}
}

// New returns a TextReporter that writes to os.Stdout.
func New() *TextReporter {
	return NewTextReporter(os.Stdout)
}

// RunStarted is a no-op for TextReporter; the orchestrator emits this via structured logging.
func (r *TextReporter) RunStarted(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string, issues []IssueSummary) {
}

// IssueStarted is a no-op for TextReporter; the orchestrator emits this via structured logging.
func (r *TextReporter) IssueStarted(issueNumber int, title string) {
}

// IssueStageChanged is a no-op for TextReporter; the orchestrator emits this via structured logging.
func (r *TextReporter) IssueStageChanged(issueNumber int, stage string) {
}

// IssueCompleted writes the outcome line for an issue.
func (r *TextReporter) IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string, _ float64, traceID string) {
	traceSuffix := ""
	if traceID != "" {
		short := traceID
		if len(short) > 8 {
			short = short[:8]
		}
		traceSuffix = fmt.Sprintf(" [trace:%s]", short)
	}
	switch status {
	case "implemented":
		fmt.Fprintf(r.w, "  #%d %s \u2014 implemented (PR #%d, %d retries)%s\n", issueNumber, title, prNumber, retries, traceSuffix)
	case "ready-to-merge":
		fmt.Fprintf(r.w, "  #%d %s \u2014 ready-to-merge (PR #%d, %d retries)%s\n", issueNumber, title, prNumber, retries, traceSuffix)
	case "needs-human-review":
		fmt.Fprintf(r.w, "  #%d %s \u2014 needs human review (PR #%d)%s\n", issueNumber, title, prNumber, traceSuffix)
	default:
		fmt.Fprintf(r.w, "  #%d %s \u2014 failed: %s%s\n", issueNumber, title, errMsg, traceSuffix)
	}
}

// WaveStarted writes the wave header line.
func (r *TextReporter) WaveStarted(wave, count int) {
	fmt.Fprintf(r.w, "\n--- Wave %d: %d newly unblocked issues ---\n", wave, count)
}

// AllBlocked writes the all-blocked message followed by the summary line.
func (r *TextReporter) AllBlocked(total, blocked int) {
	fmt.Fprintln(r.w, "All issues are blocked \u2014 nothing to process.")
	fmt.Fprintf(r.w, "Summary: %d total, %d blocked, %d processable\n", total, blocked, 0)
}

// RollupCreated writes the rollup PR creation line, then either the merged or
// open-for-review line.
func (r *TextReporter) RollupCreated(prNumber int, prURL string, merged bool) {
	fmt.Fprintf(r.w, "Rollup PR #%d created: %s\n", prNumber, prURL)
	if merged {
		fmt.Fprintf(r.w, "Rollup PR #%d merged.\n", prNumber)
	} else {
		fmt.Fprintf(r.w, "Rollup PR #%d is open for review: %s\n", prNumber, prURL)
	}
}

// RunFinished writes a blank line followed by the results summary.
func (r *TextReporter) RunFinished(implemented, readyToMerge, needsHumanReview, failed, blocked int) {
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "Results: %d implemented, %d ready-to-merge, %d needs-human-review, %d failed, %d skipped (blocked)\n",
		implemented, readyToMerge, needsHumanReview, failed, blocked)
}

// JudgeIntervention writes a judge intervention line.
func (r *TextReporter) JudgeIntervention(issueNumber int, rule, judgment, detail, step string) {
	fmt.Fprintf(r.w, "  #%d — judge %s: %s (%s) [%s]\n", issueNumber, judgment, rule, detail, step)
}

// PunchlistText writes a punchlist fragment directly.
func (r *TextReporter) PunchlistText(text string) {
	fmt.Fprint(r.w, text)
}
