package progress

import "time"

// IssueSummary holds the number and title of an issue in the run. Used to
// populate the TUI's issue table upfront so all issues appear as queued.
type IssueSummary struct {
	Number int
	Title  string
}

// ProgressReporter receives progress events from the orchestrator.
type ProgressReporter interface {
	// RunStarted signals the beginning of a run with metadata and the full
	// list of issues that will be processed.
	RunStarted(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string, issues []IssueSummary)
	// IssueStarted signals that processing has begun for an issue.
	IssueStarted(issueNumber int, title string)
	// IssueStageChanged signals a stage transition for an in-progress issue.
	IssueStageChanged(issueNumber int, stage string)
	// IssueCompleted signals the final outcome for an issue.
	IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string, costUSD float64, traceID string)
	// WaveStarted signals a new dependency re-resolution wave.
	WaveStarted(wave, count int)
	// AllBlocked signals that all issues are blocked.
	AllBlocked(total, blocked int)
	// RollupCreated signals rollup PR creation and optional merge.
	RollupCreated(prNumber int, prURL string, merged bool)
	// RunFinished signals the final summary.
	RunFinished(implemented, readyToMerge, needsHumanReview, failed, blocked int)
	// JudgeIntervention signals that the judge intervened during an agent step.
	JudgeIntervention(issueNumber int, rule, judgment, detail, step string)
	// PunchlistText outputs a punchlist fragment as it becomes available.
	PunchlistText(text string)
	// RateLimited signals that a Claude usage limit has been hit and the
	// orchestrator is holding until resetsAt.
	RateLimited(resetsAt time.Time)
	// RateLimitCleared signals that the hold period has ended and processing
	// is resuming.
	RateLimitCleared()
}
