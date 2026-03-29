package tui

import "time"

// IssueStartedMsg is sent when the orchestrator begins processing an issue.
type IssueStartedMsg struct {
	Number int
	Title  string
}

// IssueStageChangedMsg is sent when an in-progress issue transitions to a new stage.
type IssueStageChangedMsg struct {
	Number int
	Stage  string
}

// IssueCompletedMsg is sent when an issue reaches a terminal state.
type IssueCompletedMsg struct {
	Number   int
	Title    string
	Status   string
	PRNumber int
	Retries  int
	ErrMsg   string
	CostUSD  float64
	TraceID  string
}

// WaveStartedMsg is sent when a new dependency re-resolution wave begins.
type WaveStartedMsg struct {
	Wave  int
	Count int
}

// RunFinishedMsg is sent when the full run completes with final aggregate counts.
type RunFinishedMsg struct {
	Implemented      int
	ReadyToMerge     int
	NeedsHumanReview int
	Failed           int
	Blocked          int
}

// RunStartedIssue holds a single issue's number and title for initial table population.
type RunStartedIssue struct {
	Number int
	Title  string
}

// RunStartedMsg is sent when a run begins, carrying header metadata and the
// full issue list so all rows appear as queued immediately.
type RunStartedMsg struct {
	Repo         string
	Milestone    string
	Timestamp    string
	BaseBranch   string
	MergeFeature string
	MergeRollup  string
	Issues       []RunStartedIssue
}

// RollupCreatedMsg is sent when a rollup PR is created and optionally merged.
type RollupCreatedMsg struct {
	PRNumber int
	PRURL    string
	Merged   bool
}

// WatchingMsg is sent when the run transitions from initial processing to watch
// mode. The TUI updates the hint text to "watching for merges" and keeps the
// spinner running to indicate active polling.
type WatchingMsg struct{}

// JudgeInterventionMsg is sent when the judge intervenes during an agent step.
type JudgeInterventionMsg struct {
	IssueNumber int
	Rule        string
	Judgment    string
	Detail      string
	Step        string
}

// RunDoneMsg is sent when the orchestrator goroutine finishes. The TUI stays
// on screen so the user can review results; pressing q/esc/ctrl+c exits.
type RunDoneMsg struct{}

// RateLimitedMsg is sent when the orchestrator enters a hold due to a Claude
// usage limit. ResetsAt is when the limit is expected to reset.
type RateLimitedMsg struct {
	ResetsAt time.Time
}

// RateLimitClearedMsg is sent when the usage-limit hold period ends and the
// orchestrator resumes processing.
type RateLimitClearedMsg struct{}

// CountdownTickMsg is sent every second while rate-limited to drive the TUI countdown.
type CountdownTickMsg struct {
	Time time.Time
}
