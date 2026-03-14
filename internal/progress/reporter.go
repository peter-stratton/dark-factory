package progress

// ProgressReporter receives progress events from the orchestrator.
type ProgressReporter interface {
	// RunStarted signals the beginning of a run with metadata.
	RunStarted(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string, issueCount int)
	// IssueStarted signals that processing has begun for an issue.
	IssueStarted(issueNumber int, title string)
	// IssueStageChanged signals a stage transition for an in-progress issue.
	IssueStageChanged(issueNumber int, stage string)
	// IssueCompleted signals the final outcome for an issue.
	IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string)
	// WaveStarted signals a new dependency re-resolution wave.
	WaveStarted(wave, count int)
	// AllBlocked signals that all issues are blocked.
	AllBlocked(total, blocked int)
	// RollupCreated signals rollup PR creation and optional merge.
	RollupCreated(prNumber int, prURL string, merged bool)
	// RunFinished signals the final summary.
	RunFinished(implemented, readyToMerge, needsHumanReview, failed, blocked int)
	// PunchlistText outputs a punchlist fragment as it becomes available.
	PunchlistText(text string)
}
