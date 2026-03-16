package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/phs/dark-factory/internal/progress"
)

// sender is a private interface over *tea.Program to allow unit testing without
// a real terminal or event loop.
type sender interface {
	Send(msg tea.Msg)
}

// TUIReporter implements progress.ProgressReporter by dispatching Bubble Tea
// messages to the running program. It is created in the cmd layer once the
// tea.Program is initialised, and passed to the orchestrator as a ProgressReporter.
type TUIReporter struct {
	p sender
}

// Compile-time assertion that TUIReporter implements progress.ProgressReporter.
var _ progress.ProgressReporter = (*TUIReporter)(nil)

// NewTUIReporter returns a TUIReporter that sends messages to p.
func NewTUIReporter(p *tea.Program) *TUIReporter {
	return &TUIReporter{p: p}
}

// RunStarted sends RunStartedMsg with run header metadata and the full issue list.
func (r *TUIReporter) RunStarted(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string, issues []progress.IssueSummary) {
	tuiIssues := make([]RunStartedIssue, len(issues))
	for i, iss := range issues {
		tuiIssues[i] = RunStartedIssue{Number: iss.Number, Title: iss.Title}
	}
	r.p.Send(RunStartedMsg{
		Repo:         repo,
		Milestone:    milestone,
		Timestamp:    timestamp,
		BaseBranch:   baseBranch,
		MergeFeature: mergeFeature,
		MergeRollup:  mergeRollup,
		Issues:       tuiIssues,
	})
}

// IssueStarted sends IssueStartedMsg when the orchestrator begins an issue.
func (r *TUIReporter) IssueStarted(issueNumber int, title string) {
	r.p.Send(IssueStartedMsg{Number: issueNumber, Title: title})
}

// IssueStageChanged sends IssueStageChangedMsg on a stage transition.
func (r *TUIReporter) IssueStageChanged(issueNumber int, stage string) {
	r.p.Send(IssueStageChangedMsg{Number: issueNumber, Stage: stage})
}

// IssueCompleted sends IssueCompletedMsg with the terminal outcome for an issue.
func (r *TUIReporter) IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string, costUSD float64) {
	r.p.Send(IssueCompletedMsg{
		Number:   issueNumber,
		Title:    title,
		Status:   status,
		PRNumber: prNumber,
		Retries:  retries,
		ErrMsg:   errMsg,
		CostUSD:  costUSD,
	})
}

// WaveStarted sends WaveStartedMsg when a new dependency wave begins.
func (r *TUIReporter) WaveStarted(wave, count int) {
	r.p.Send(WaveStartedMsg{Wave: wave, Count: count})
}

// AllBlocked sends RunFinishedMsg with all issues counted as blocked.
func (r *TUIReporter) AllBlocked(total, blocked int) {
	r.p.Send(RunFinishedMsg{Blocked: total})
}

// RollupCreated sends RollupCreatedMsg with PR details and merge status.
func (r *TUIReporter) RollupCreated(prNumber int, prURL string, merged bool) {
	r.p.Send(RollupCreatedMsg{PRNumber: prNumber, PRURL: prURL, Merged: merged})
}

// RunFinished sends RunFinishedMsg with the final aggregate counts.
func (r *TUIReporter) RunFinished(implemented, readyToMerge, needsHumanReview, failed, blocked int) {
	r.p.Send(RunFinishedMsg{
		Implemented:      implemented,
		ReadyToMerge:     readyToMerge,
		NeedsHumanReview: needsHumanReview,
		Failed:           failed,
		Blocked:          blocked,
	})
}

// RunAborted sends RunAbortedMsg with the abort reason so the TUI can display it.
func (r *TUIReporter) RunAborted(reason string) {
	r.p.Send(RunAbortedMsg{Reason: reason})
}

// PunchlistText is a no-op in TUI mode. The punchlist is written to a file;
// the TUI does not display it inline.
func (r *TUIReporter) PunchlistText(text string) {}
