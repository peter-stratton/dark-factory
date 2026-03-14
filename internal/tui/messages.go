package tui

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
