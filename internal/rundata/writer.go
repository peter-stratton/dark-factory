package rundata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// AutoMerge holds the auto-merge strategy settings recorded at run start.
type AutoMerge struct {
	// Feature controls merging of the feature branch into the base branch.
	// Valid values: "none", "low_risk", "all".
	Feature string `json:"feature"`
	// Rollup controls what happens to the base branch → default branch after
	// a run completes. Valid values: "none", "manual", "auto".
	Rollup string `json:"rollup"`
}

// RunMeta is the structure persisted to run.json.
type RunMeta struct {
	Repo         string            `json:"repo"`
	Milestone    string            `json:"milestone"`
	BaseBranch   string            `json:"base_branch,omitempty"`
	AutoMerge    *AutoMerge        `json:"auto_merge,omitempty"`
	IssueNumbers []int             `json:"issue_numbers"`
	IssueDeps    []IssueDep        `json:"issue_deps,omitempty"`
	IssueTitles  map[string]string `json:"issue_titles,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	Summary      *RunSummary       `json:"summary,omitempty"`
	RunnerInfo   string            `json:"runner_info,omitempty"`
}

// IssueDep records the dependency info for a single issue.
type IssueDep struct {
	IssueNumber int   `json:"issue_number"`
	DependsOn   []int `json:"depends_on"`
}

// IssueStatus records the live execution status for a single issue.
// Written to status.json during processing; distinct from the final outcome.
type IssueStatus struct {
	Status string `json:"status"`
}

// RunSummary holds the outcome summary written by FinalizeRun.
type RunSummary struct {
	Total          int    `json:"total"`
	Implemented    int    `json:"implemented"`
	Failed         int    `json:"failed"`
	AbortReason    string `json:"abort_reason,omitempty"`
	RollupPRNumber int    `json:"rollup_pr_number,omitempty"`
	RollupPRURL    string `json:"rollup_pr_url,omitempty"`
}

// Flag records a quality issue detected in a review step.
type Flag struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StepResult holds the output of a single agent step.
type StepResult struct {
	Output           string     `json:"output,omitempty"`
	Error            string     `json:"error,omitempty"`
	TimedOut         bool       `json:"timed_out,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	DurationSeconds  float64    `json:"duration_seconds,omitempty"`
	CostUSD          float64    `json:"cost_usd,omitempty"`
	Flags            []Flag     `json:"flags,omitempty"`
	ToolTrace        []string   `json:"tool_trace,omitempty"`
	SessionID        string     `json:"session_id,omitempty"`
	PeakMemoryBytes  int64      `json:"peak_memory_bytes,omitempty"`
	CPUNanoseconds   int64      `json:"cpu_nanoseconds,omitempty"`
	TraceID          string     `json:"trace_id,omitempty"`
}

// OutcomeStatus constants mirror the agent.OutcomeStatus values for use in
// packages that cannot import the orchestration layer (e.g. presentation).
// These must stay in sync with the constants defined in internal/agent/loop.go.
const (
	OutcomeStatusImplemented      = "implemented"
	OutcomeStatusReadyToMerge     = "ready-to-merge"
	OutcomeStatusNeedsHumanReview = "needs-human-review"
	OutcomeStatusFailed           = "failed"
)

// Outcome records the final result for a single issue.
type Outcome struct {
	IssueNumber int    `json:"issue_number"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	PRNumber    int    `json:"pr_number"`
	Error       string `json:"error,omitempty"`
	TraceID     string `json:"trace_id,omitempty"`
	CloneSHA    string `json:"clone_sha,omitempty"`
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
func New(repo, milestone string, issueNumbers []int, baseBranch string, autoMerge AutoMerge) (*Writer, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	return NewWithBase(filepath.Join(home, ".godark", "runs"), repo, milestone, issueNumbers, baseBranch, autoMerge)
}

// NewWithBase creates a new Writer using a custom base directory instead of
// the default ~/.godark/runs/. Intended for testing.
func NewWithBase(baseDir, repo, milestone string, issueNumbers []int, baseBranch string, autoMerge AutoMerge) (*Writer, error) {
	owner, repoName, err := validateRepo(repo)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	timestamp := now.Format("20060102-150405")
	dir := filepath.Join(baseDir, owner, repoName, timestamp)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating run directory: %w", err)
	}

	w := &Writer{
		dir:       dir,
		repo:      repo,
		milestone: milestone,
		startedAt: now,
	}

	var autoMergePtr *AutoMerge
	if autoMerge.Feature != "" || autoMerge.Rollup != "" {
		cp := autoMerge
		autoMergePtr = &cp
	}
	meta := RunMeta{
		Repo:         repo,
		Milestone:    milestone,
		BaseBranch:   baseBranch,
		AutoMerge:    autoMergePtr,
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

// issueDir returns the directory path for the given issue number.
func (w *Writer) issueDir(issueNum int) string {
	return filepath.Join(w.dir, "issues", strconv.Itoa(issueNum))
}

// IssueDir returns the directory path for the given issue number.
// Callers use this to locate step result files after processing.
func (w *Writer) IssueDir(issueNum int) string {
	return w.issueDir(issueNum)
}

// IssueCostUSD returns the total cost of all recorded step results in issueDir.
// It reads recon.json, spec-generator.json, implement.json, quality-review.json,
// functional-review.json, and all retry subdirectory JSON files. Errors reading
// individual files are silently ignored — cost computation is best-effort.
func IssueCostUSD(issueDir string) float64 {
	var total float64
	for _, name := range []string{"recon.json", "spec-generator.json", "planner.json", "implement.json", "quality-review.json", "functional-review.json"} {
		total += readStepCost(filepath.Join(issueDir, name))
	}
	retriesDir := filepath.Join(issueDir, "retries")
	entries, err := os.ReadDir(retriesDir)
	if err != nil {
		// No retries directory or unreadable — not an error.
		return total
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		retryDir := filepath.Join(retriesDir, entry.Name())
		for _, name := range []string{"retry.json", "quality-review.json", "functional-review.json"} {
			total += readStepCost(filepath.Join(retryDir, name))
		}
	}
	return total
}

// readStepCost reads a StepResult JSON file and returns its CostUSD field.
// Returns 0.0 if the file does not exist or cannot be parsed.
func readStepCost(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0.0
	}
	var step struct {
		CostUSD float64 `json:"cost_usd"`
	}
	if err := json.Unmarshal(data, &step); err != nil {
		return 0.0
	}
	return step.CostUSD
}

// issueRetryDir returns the directory path for the given issue and retry number.
func (w *Writer) issueRetryDir(issueNum, retryNum int) string {
	return filepath.Join(w.issueDir(issueNum), "retries", strconv.Itoa(retryNum))
}

// WriteReconResult writes the recon step result for the given issue.
// Path: issues/<issueNum>/recon.json
func (w *Writer) WriteReconResult(issueNum int, step StepResult) error {
	path := filepath.Join(w.issueDir(issueNum), "recon.json")
	return writeJSONMkdirs(path, step)
}

// WritePlannerResult writes the planner step result for the given issue.
// Path: issues/<issueNum>/planner.json
func (w *Writer) WritePlannerResult(issueNum int, step StepResult) error {
	path := filepath.Join(w.issueDir(issueNum), "planner.json")
	return writeJSONMkdirs(path, step)
}

// WriteSpecGeneratorResult writes the spec generator step result for the given issue.
// Path: issues/<issueNum>/spec-generator.json
func (w *Writer) WriteSpecGeneratorResult(issueNum int, step StepResult) error {
	path := filepath.Join(w.issueDir(issueNum), "spec-generator.json")
	return writeJSONMkdirs(path, step)
}

// WriteImplementResult writes the implement step result for the given issue.
// Path: issues/<issueNum>/implement.json
func (w *Writer) WriteImplementResult(issueNum int, step StepResult) error {
	path := filepath.Join(w.issueDir(issueNum), "implement.json")
	return writeJSONMkdirs(path, step)
}

// WriteMergeCoordinatorResult writes the merge coordinator step result for the given issue.
// Path: issues/<issueNum>/merge_coordinator.json
func (w *Writer) WriteMergeCoordinatorResult(issueNum int, step StepResult) error {
	path := filepath.Join(w.issueDir(issueNum), "merge_coordinator.json")
	return writeJSONMkdirs(path, step)
}

// WriteReviewResult writes a review step result for the given issue.
// kind must be "quality" or "functional".
// Path: issues/<issueNum>/<kind>-review.json
func (w *Writer) WriteReviewResult(issueNum int, kind string, step StepResult) error {
	if kind != "quality" && kind != "functional" {
		return fmt.Errorf("review kind must be %q or %q, got %q", "quality", "functional", kind)
	}
	path := filepath.Join(w.issueDir(issueNum), kind+"-review.json")
	return writeJSONMkdirs(path, step)
}

// WriteRetryResult writes a retry step result for the given issue and retry number.
// Path: issues/<issueNum>/retries/<retryNum>/retry.json
func (w *Writer) WriteRetryResult(issueNum, retryNum int, step StepResult) error {
	path := filepath.Join(w.issueRetryDir(issueNum, retryNum), "retry.json")
	return writeJSONMkdirs(path, step)
}

// WriteRetryReviewResult writes a quality review result for a retry step.
// Path: issues/<issueNum>/retries/<retryNum>/quality-review.json
func (w *Writer) WriteRetryReviewResult(issueNum, retryNum int, step StepResult) error {
	path := filepath.Join(w.issueRetryDir(issueNum, retryNum), "quality-review.json")
	return writeJSONMkdirs(path, step)
}

// WriteRetryFunctionalReviewResult writes the functional review that triggered a retry.
// Path: issues/<issueNum>/retries/<retryNum>/functional-review.json
func (w *Writer) WriteRetryFunctionalReviewResult(issueNum, retryNum int, step StepResult) error {
	path := filepath.Join(w.issueRetryDir(issueNum, retryNum), "functional-review.json")
	return writeJSONMkdirs(path, step)
}

// WriteOutcome writes the outcome for the issue identified by outcome.IssueNumber.
// Path: issues/<issueNum>/outcome.json
func (w *Writer) WriteOutcome(outcome Outcome) error {
	path := filepath.Join(w.issueDir(outcome.IssueNumber), "outcome.json")
	return writeJSONMkdirs(path, outcome)
}

// WriteIssueStatus writes the live execution status for the given issue.
// Path: issues/<issueNum>/status.json
func (w *Writer) WriteIssueStatus(issueNum int, status IssueStatus) error {
	path := filepath.Join(w.issueDir(issueNum), "status.json")
	return writeJSONMkdirs(path, status)
}

// WriteIssueDeps updates run.json with the dependency graph for all issues.
// It reads the current run.json, sets IssueDeps, and writes it back.
func (w *Writer) WriteIssueDeps(deps []IssueDep) error {
	path := filepath.Join(w.dir, "run.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading run.json: %w", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing run.json: %w", err)
	}

	meta.IssueDeps = deps
	if err := writeJSON(path, meta); err != nil {
		return fmt.Errorf("updating run.json with issue deps: %w", err)
	}
	return nil
}

// WriteIssueTitles updates run.json with the title map for all issues.
// It reads the current run.json, sets IssueTitles, and writes it back.
func (w *Writer) WriteIssueTitles(titles map[string]string) error {
	path := filepath.Join(w.dir, "run.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading run.json: %w", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("parsing run.json: %w", err)
	}

	meta.IssueTitles = titles
	if err := writeJSON(path, meta); err != nil {
		return fmt.Errorf("updating run.json with issue titles: %w", err)
	}
	return nil
}

// DialogueEntry records one turn in the agent dialogue for an issue.
type DialogueEntry struct {
	Role    string `json:"role"`              // "implementer", "quality_reviewer", or "reviewer"
	Round   int    `json:"round"`             // 1-indexed retry round
	Body    string `json:"body"`              // raw comment text
	Outcome string `json:"outcome,omitempty"` // "accepted" or "changes_requested" (reviewers only)
}

// WriteDialogue writes the dialogue entries for the given issue.
// Path: issues/<issueNum>/dialogue.json
func (w *Writer) WriteDialogue(issueNum int, entries []DialogueEntry) error {
	path := filepath.Join(w.issueDir(issueNum), "dialogue.json")
	return writeJSONMkdirs(path, entries)
}

// CheckResult holds the outcome of a single verification check.
type CheckResult struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Output   string `json:"output,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// VerifyStepResult holds the outcome of one verify attempt (initial or fix retry).
type VerifyStepResult struct {
	Attempt      int           `json:"attempt"` // 0-indexed
	Checks       []CheckResult `json:"checks"`
	AllPassed    bool          `json:"all_passed"`
	FixAttempted bool          `json:"fix_attempted"`
	TraceID      string        `json:"trace_id,omitempty"`
}

// WriteVerifyResult writes a verify step result for the given issue and attempt.
// Path: issues/<issueNum>/verify-<attempt>.json
func (w *Writer) WriteVerifyResult(issueNum int, step VerifyStepResult) error {
	path := filepath.Join(w.issueDir(issueNum), fmt.Sprintf("verify-%d.json", step.Attempt))
	return writeJSONMkdirs(path, step)
}

// WriteRollupVerifyResult writes a verify step result for the rollup PR.
// Path: rollup/verify-<attempt>.json
func (w *Writer) WriteRollupVerifyResult(step VerifyStepResult) error {
	path := filepath.Join(w.dir, "rollup", fmt.Sprintf("verify-%d.json", step.Attempt))
	return writeJSONMkdirs(path, step)
}

// WriteRollupMergeCoordinatorResult writes a merge coordinator step result
// for the rollup PR.
// Path: rollup/merge_coordinator-<attempt>.json
func (w *Writer) WriteRollupMergeCoordinatorResult(attempt int, step StepResult) error {
	path := filepath.Join(w.dir, "rollup", fmt.Sprintf("merge_coordinator-%d.json", attempt))
	return writeJSONMkdirs(path, step)
}

// RiskGate records the outcome of a single risk gate evaluation.
type RiskGate struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// RiskAssessment holds the result of the risk classifier for a PR.
type RiskAssessment struct {
	IsLowRisk bool       `json:"is_low_risk"`
	Gates     []RiskGate `json:"gates"`
}

// WriteRiskAssessment writes the risk assessment for the given issue.
// Path: issues/<issueNum>/risk-assessment.json
func (w *Writer) WriteRiskAssessment(issueNum int, assessment RiskAssessment) error {
	path := filepath.Join(w.issueDir(issueNum), "risk-assessment.json")
	return writeJSONMkdirs(path, assessment)
}

// FailurePattern records a single detected failure pattern from log analysis.
type FailurePattern struct {
	Code     string `json:"code"`
	Count    int    `json:"count"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// FailureAnalysis holds the post-mortem analysis for a failed issue.
type FailureAnalysis struct {
	Patterns          []FailurePattern `json:"patterns"`
	LogLinesCaptured  int              `json:"log_lines_captured"`
	ContainerExitCode int              `json:"container_exit_code"`
}

// WriteFailureAnalysis writes the failure analysis for the given issue.
// Path: issues/<issueNum>/failure-analysis.json
func (w *Writer) WriteFailureAnalysis(issueNum int, analysis FailureAnalysis) error {
	path := filepath.Join(w.issueDir(issueNum), "failure-analysis.json")
	return writeJSONMkdirs(path, analysis)
}

// JudgeIntervention records a single judge intervention during an agent run.
type JudgeIntervention struct {
	Rule       string         `json:"rule"`
	Judgment   string         `json:"judgment"`
	Detail     string         `json:"detail"`
	Counts     map[string]int `json:"counts,omitempty"`
	DetectedAt time.Time      `json:"detected_at"`
	Step       string         `json:"step"`
}

// WriteJudgeIntervention appends an intervention record for the given issue.
// Path: issues/<issueNum>/judge-interventions.json
// If the file already exists it is read, the new entry appended, and the
// whole array written back; if absent a new single-element array is created.
func (w *Writer) WriteJudgeIntervention(issueNum int, intervention JudgeIntervention) error {
	path := filepath.Join(w.issueDir(issueNum), "judge-interventions.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directories for %s: %w", path, err)
	}

	var existing []JudgeIntervention
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	existing = append(existing, intervention)
	return writeJSON(path, existing)
}

// WriteContainerLog writes the raw container log for the given issue.
// Path: issues/<issueNum>/container-log.txt
func (w *Writer) WriteContainerLog(issueNum int, log string) error {
	path := filepath.Join(w.issueDir(issueNum), "container-log.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directories for %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(log), 0o644)
}

// PunchlistData holds the per-issue punchlist content persisted to punchlist.json.
type PunchlistData struct {
	VerificationSteps []string `json:"verification_steps,omitempty"`
	ScenarioCases     []string `json:"scenario_cases,omitempty"`
	AcceptanceTests   []string `json:"acceptance_tests,omitempty"`
	ChangedFiles      []string `json:"changed_files,omitempty"`
	// EnrichmentStatus records whether LLM enrichment was skipped, failed, or succeeded.
	// Values: "skipped" (no prompt configured), "failed" (LLM call or parse error), "success".
	EnrichmentStatus string `json:"enrichment_status,omitempty"`
}

// WritePunchlist writes the punchlist data for the given issue.
// Path: issues/<issueNum>/punchlist.json
func (w *Writer) WritePunchlist(issueNum int, data PunchlistData) error {
	path := filepath.Join(w.issueDir(issueNum), "punchlist.json")
	return writeJSONMkdirs(path, data)
}

// SpecDeltaData holds the scenario spec diff persisted to spec-delta.json.
type SpecDeltaData struct {
	Before       string   `json:"before"`
	After        string   `json:"after"`
	AddedCases   []string `json:"added_cases,omitempty"`
	RemovedCases []string `json:"removed_cases,omitempty"`
	ChangedCases int      `json:"changed_cases"`
	SetupChanged bool     `json:"setup_changed"`
}

// WriteSpecDelta writes the spec delta for the given issue.
// Path: issues/<issueNum>/spec-delta.json
func (w *Writer) WriteSpecDelta(issueNum int, delta SpecDeltaData) error {
	path := filepath.Join(w.issueDir(issueNum), "spec-delta.json")
	return writeJSONMkdirs(path, delta)
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
