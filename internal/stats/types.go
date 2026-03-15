package stats

import "time"

// RunRecord holds the data for a single run row in the stats database.
// Fields map 1:1 to columns in the runs table.
type RunRecord struct {
	ID               string
	Repo             string
	Milestone        string
	BaseBranch       string
	AutoMergeFeature string
	AutoMergeRollup  string
	StartedAt        time.Time
	FinishedAt       time.Time
	Total            int
	Implemented      int
	Failed           int
	AbortReason      string
}

// IssueOutcomeRecord holds the data for a single row in issue_outcomes.
// Fields map 1:1 to columns in the issue_outcomes table.
type IssueOutcomeRecord struct {
	RunID       string
	IssueNumber int
	Title       string
	Status      string
	PRNumber    int
	Error       string
}

// StepResultRecord holds the data for a single row in step_results.
// Fields map 1:1 to columns in the step_results table.
// Flags is a slice of flag code strings serialized as a JSON array in storage.
type StepResultRecord struct {
	RunID           string
	IssueNumber     int
	StepName        string
	CostUSD         float64
	DurationSeconds float64
	Flags           []string
	StartedAt       time.Time
	FinishedAt      time.Time
}
