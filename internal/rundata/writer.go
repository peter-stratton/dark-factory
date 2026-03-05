package rundata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunMeta is the structure persisted to run.json.
type RunMeta struct {
	Repo         string      `json:"repo"`
	Milestone    string      `json:"milestone"`
	IssueNumbers []int       `json:"issue_numbers"`
	StartedAt    time.Time   `json:"started_at"`
	FinishedAt   *time.Time  `json:"finished_at,omitempty"`
	Summary      *RunSummary `json:"summary,omitempty"`
}

// RunSummary holds the outcome summary written by FinalizeRun.
type RunSummary struct {
	Total       int `json:"total"`
	Implemented int `json:"implemented"`
	Failed      int `json:"failed"`
}

// Flag records a quality issue detected in a review step.
type Flag struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StepResult holds the output of a single agent step.
type StepResult struct {
	Output          string     `json:"output,omitempty"`
	Error           string     `json:"error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	DurationSeconds float64    `json:"duration_seconds,omitempty"`
	CostUSD         float64    `json:"cost_usd,omitempty"`
	Flags           []Flag     `json:"flags,omitempty"`
}

// Outcome records the final result for a single issue.
type Outcome struct {
	IssueNumber int    `json:"issue_number"`
	Status      string `json:"status"`
	PRNumber    int    `json:"pr_number"`
}

// Writer manages a per-run directory and writes JSON files for each agent loop step.
type Writer struct {
	dir       string
	repo      string
	milestone string
	startedAt time.Time
}

// New creates a new Writer for the given repo and milestone. It creates the run
// directory under ~/.godark/runs/<owner>/<repo>/<YYYYMMDD-HHMMSS>/ and writes
// an initial run.json. Repo must be in "owner/name" format; components
// containing ".." or path separators are rejected.
func New(repo, milestone string, issueNumbers []int) (*Writer, error) {
	owner, repoName, err := validateRepo(repo)
	if err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	now := time.Now().UTC()
	timestamp := now.Format("20060102-150405")
	dir := filepath.Join(home, ".godark", "runs", owner, repoName, timestamp)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating run directory: %w", err)
	}

	w := &Writer{
		dir:       dir,
		repo:      repo,
		milestone: milestone,
		startedAt: now,
	}

	meta := RunMeta{
		Repo:         repo,
		Milestone:    milestone,
		IssueNumbers: issueNumbers,
		StartedAt:    now,
	}
	if err := writeJSON(filepath.Join(dir, "run.json"), meta); err != nil {
		return nil, fmt.Errorf("writing run.json: %w", err)
	}

	return w, nil
}

// Dir returns the run directory path.
func (w *Writer) Dir() string {
	return w.dir
}

// WriteImplementResult writes the implement step result for the given issue.
// Path: issues/<issueNum>/implement.json
func (w *Writer) WriteImplementResult(issueNum int, step StepResult) error {
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNum), "implement.json")
	return writeJSONMkdirs(path, step)
}

// WriteReviewResult writes a review step result for the given issue.
// kind must be "quality" or "functional".
// Path: issues/<issueNum>/<kind>-review.json
func (w *Writer) WriteReviewResult(issueNum int, kind string, step StepResult) error {
	if kind != "quality" && kind != "functional" {
		return fmt.Errorf("review kind must be %q or %q, got %q", "quality", "functional", kind)
	}
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNum), kind+"-review.json")
	return writeJSONMkdirs(path, step)
}

// WriteRetryResult writes a retry step result for the given issue and retry number.
// Path: issues/<issueNum>/retries/<retryNum>/retry.json
func (w *Writer) WriteRetryResult(issueNum, retryNum int, step StepResult) error {
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNum),
		"retries", fmt.Sprintf("%d", retryNum), "retry.json")
	return writeJSONMkdirs(path, step)
}

// WriteRetryReviewResult writes a quality review result for a retry step.
// Path: issues/<issueNum>/retries/<retryNum>/quality-review.json
func (w *Writer) WriteRetryReviewResult(issueNum, retryNum int, step StepResult) error {
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", issueNum),
		"retries", fmt.Sprintf("%d", retryNum), "quality-review.json")
	return writeJSONMkdirs(path, step)
}

// WriteOutcome writes the outcome for the issue identified by outcome.IssueNumber.
// Path: issues/<issueNum>/outcome.json
func (w *Writer) WriteOutcome(outcome Outcome) error {
	path := filepath.Join(w.dir, "issues", fmt.Sprintf("%d", outcome.IssueNumber), "outcome.json")
	return writeJSONMkdirs(path, outcome)
}

// FinalizeRun updates run.json with the finished_at timestamp and summary.
func (w *Writer) FinalizeRun(summary RunSummary) error {
	path := filepath.Join(w.dir, "run.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading run.json: %w", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing run.json: %w", err)
	}

	now := time.Now().UTC()
	meta.FinishedAt = &now
	meta.Summary = &summary

	if err := writeJSON(path, meta); err != nil {
		return fmt.Errorf("updating run.json: %w", err)
	}
	return nil
}

// validateRepo checks that repo is in "owner/name" format and that no
// component contains ".." or path separators.
func validateRepo(repo string) (owner, name string, err error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repo must be in owner/name format, got %q", repo)
	}
	for _, part := range parts {
		if part == "" {
			return "", "", fmt.Errorf("repo component must not be empty")
		}
		if strings.Contains(part, "..") {
			return "", "", fmt.Errorf("repo component must not contain ..: %q", part)
		}
		if strings.ContainsAny(part, `\/`) {
			return "", "", fmt.Errorf("repo component must not contain path separators: %q", part)
		}
	}
	return parts[0], parts[1], nil
}

// writeJSON marshals v and writes it to path, truncating any existing file.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// writeJSONMkdirs creates parent directories as needed, then writes JSON.
func writeJSONMkdirs(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directories for %s: %w", path, err)
	}
	return writeJSON(path, v)
}
