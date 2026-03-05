package rundata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
)

// timeNow is a variable so tests can override it.
var timeNow = time.Now

// configSnapshot holds the config fields recorded in run.json.
type configSnapshot struct {
	MaxRetries int  `json:"max_retries"`
	NoSandbox  bool `json:"no_sandbox"`
	NoMerge    bool `json:"no_merge"`
}

// runData is the schema for run.json.
type runData struct {
	StartedAt      string         `json:"started_at"`
	FinishedAt     *string        `json:"finished_at"`
	Repo           string         `json:"repo"`
	Milestone      string         `json:"milestone"`
	IssueNumbers   []int          `json:"issue_numbers"`
	ConfigSnapshot configSnapshot `json:"config_snapshot"`
	Summary        *RunSummary    `json:"summary,omitempty"`
}

// RunSummary holds the final count statistics for a run.
type RunSummary struct {
	Total        int     `json:"total"`
	Implemented  int     `json:"implemented"`
	Failed       int     `json:"failed"`
	Skipped      int     `json:"skipped"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// StepResult is the per-step JSON schema used by implement.json, review files, and retry files.
type StepResult struct {
	StartedAt       string   `json:"started_at"`
	FinishedAt      string   `json:"finished_at"`
	DurationSeconds float64  `json:"duration_seconds"`
	CostUSD         float64  `json:"cost_usd"`
	SessionID       string   `json:"session_id"`
	Verdict         string   `json:"verdict,omitempty"`
	ToolTrace       []string `json:"tool_trace"`
	TimedOut        bool     `json:"timed_out"`
	ExitCode        int      `json:"exit_code"`
}

// Outcome is the per-issue outcome JSON schema for outcome.json.
type Outcome struct {
	IssueNumber  int     `json:"issue_number"`
	IssueTitle   string  `json:"issue_title"`
	Status       string  `json:"status"`
	PRNumber     int     `json:"pr_number"`
	Retries      int     `json:"retries"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Error        *string `json:"error"`
}

// RunDataWriter manages the run directory and writes JSON data files.
type RunDataWriter struct {
	dir          string
	repo         string
	milestone    string
	startedAt    time.Time
	issueNumbers []int
	cfg          *config.Config
}

// New creates the run directory under ~/.godark/runs/<owner>/<repo>/<timestamp>/
// and writes the initial run.json. repo must be in "owner/repo" format.
func New(repo, milestone string, issueNumbers []int, cfg *config.Config) (*RunDataWriter, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid repo format %q, expected owner/repo", repo)
	}
	owner, repoName := parts[0], parts[1]

	now := timeNow()
	timestamp := now.Format("20060102-150405")
	dir := filepath.Join(homeDir, ".godark", "runs", owner, repoName, timestamp)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating run directory: %w", err)
	}

	if issueNumbers == nil {
		issueNumbers = []int{}
	}

	w := &RunDataWriter{
		dir:          dir,
		repo:         repo,
		milestone:    milestone,
		startedAt:    now,
		issueNumbers: issueNumbers,
		cfg:          cfg,
	}

	if err := w.writeRunJSON(nil); err != nil {
		return nil, fmt.Errorf("writing initial run.json: %w", err)
	}

	return w, nil
}

// Dir returns the run directory path.
func (w *RunDataWriter) Dir() string {
	return w.dir
}

// UpdateIssueNumbers updates run.json with the resolved issue numbers.
// Call this after issue resolution when the final list is known.
func (w *RunDataWriter) UpdateIssueNumbers(nums []int) error {
	if nums == nil {
		nums = []int{}
	}
	w.issueNumbers = nums
	return w.writeRunJSON(nil)
}

// FinalizeRun updates run.json with the finished_at timestamp and summary.
func (w *RunDataWriter) FinalizeRun(summary RunSummary) error {
	now := timeNow().UTC().Format(time.RFC3339)
	data := w.buildRunData()
	data.FinishedAt = &now
	data.Summary = &summary
	return writeJSON(filepath.Join(w.dir, "run.json"), data)
}

// WriteImplementResult writes issues/<n>/implement.json for the given issue.
func (w *RunDataWriter) WriteImplementResult(issueNumber int, result *agent.Result) error {
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNumber), "implement.json")
	return writeJSON(path, resultToStep(result))
}

// WriteReviewResult writes a review JSON file.
// kind is "quality" or "functional".
// If retry == 0, writes to issues/<n>/<kind>-review.json.
// If retry > 0, writes to issues/<n>/retries/<retry>/<kind>-review.json.
func (w *RunDataWriter) WriteReviewResult(issueNumber int, kind string, retry int, result *agent.Result) error {
	var path string
	if retry == 0 {
		path = filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNumber), kind+"-review.json")
	} else {
		path = filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNumber), "retries", fmt.Sprintf("%d", retry), kind+"-review.json")
	}
	return writeJSON(path, resultToStep(result))
}

// WriteRetryResult writes issues/<n>/retries/<n>/retry.json.
// n is the retry number (1-based).
func (w *RunDataWriter) WriteRetryResult(issueNumber, n int, result *agent.Result) error {
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNumber), "retries", fmt.Sprintf("%d", n), "retry.json")
	return writeJSON(path, resultToStep(result))
}

// WriteOutcome writes issues/<n>/outcome.json for the given issue.
func (w *RunDataWriter) WriteOutcome(outcome Outcome) error {
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", outcome.IssueNumber), "outcome.json")
	return writeJSON(path, outcome)
}

func (w *RunDataWriter) writeRunJSON(summary *RunSummary) error {
	data := w.buildRunData()
	data.Summary = summary
	return writeJSON(filepath.Join(w.dir, "run.json"), data)
}

func (w *RunDataWriter) buildRunData() runData {
	nums := w.issueNumbers
	if nums == nil {
		nums = []int{}
	}
	return runData{
		StartedAt:    w.startedAt.UTC().Format(time.RFC3339),
		FinishedAt:   nil,
		Repo:         w.repo,
		Milestone:    w.milestone,
		IssueNumbers: nums,
		ConfigSnapshot: configSnapshot{
			MaxRetries: w.cfg.MaxRetries,
			NoSandbox:  w.cfg.NoSandbox,
			NoMerge:    w.cfg.NoMerge,
		},
	}
}

func resultToStep(result *agent.Result) StepResult {
	now := timeNow().UTC().Format(time.RFC3339)
	toolTrace := result.ToolTrace
	if toolTrace == nil {
		toolTrace = []string{}
	}
	return StepResult{
		StartedAt:       now,
		FinishedAt:      now,
		DurationSeconds: 0,
		CostUSD:         result.CostUSD,
		SessionID:       result.SessionID,
		Verdict:         result.Verdict,
		ToolTrace:       toolTrace,
		TimedOut:        result.TimedOut,
		ExitCode:        result.ExitCode,
	}
}

func writeJSON(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
