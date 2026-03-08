package rundata

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// RetryDetail holds the step results for one retry attempt.
type RetryDetail struct {
	Attempt          int
	Retry            StepResult
	QualityReview    StepResult
	FunctionalReview StepResult
}

// IssueDetail aggregates all data for one issue within a run.
type IssueDetail struct {
	IssueNumber      int
	Outcome          Outcome
	LiveStatus       IssueStatus  // from status.json; empty Status if not present
	BlockedBy        []int        // open (not-yet-completed) dependencies; derived by LoadRun
	SpecGenerator    StepResult
	Implement        StepResult
	QualityReview    StepResult
	FunctionalReview StepResult
	Retries          []RetryDetail
	Punchlist        *PunchlistData
	Dialogue         []DialogueEntry
	VerifyResults    []VerifyStepResult
}

// RunDetail holds the full data for one run, including per-issue details.
type RunDetail struct {
	RunMeta
	Issues []IssueDetail
}

// Reader reads run data from a base directory (default: ~/.godark/runs/).
type Reader struct {
	logger  *slog.Logger
	baseDir string
}

// NewReader creates a Reader using ~/.godark/runs/ as the base directory.
// If logger is nil, slog.Default() is used.
func NewReader(logger *slog.Logger) (*Reader, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{
		logger:  logger,
		baseDir: filepath.Join(home, ".godark", "runs"),
	}, nil
}

// NewReaderWithBase creates a Reader using a custom base directory.
// If logger is nil, slog.Default() is used. Intended for testing.
func NewReaderWithBase(baseDir string, logger *slog.Logger) *Reader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reader{
		logger:  logger,
		baseDir: baseDir,
	}
}

// RunDir returns the filesystem path to the directory for the given run.
// It does not validate path components; callers are responsible for validation.
func (r *Reader) RunDir(owner, repo, timestamp string) string {
	return filepath.Join(r.baseDir, owner, repo, timestamp)
}

// ListRuns walks the base directory and returns all RunMeta entries sorted
// most-recent-first by started_at. Missing or corrupt run.json files are
// skipped with a warning log. Returns an empty slice (no error) if the base
// directory does not exist.
func (r *Reader) ListRuns() ([]RunMeta, error) {
	ownerEntries, err := os.ReadDir(r.baseDir)
	if errors.Is(err, os.ErrNotExist) {
		return []RunMeta{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading runs dir: %w", err)
	}

	var metas []RunMeta
	for _, ownerEntry := range ownerEntries {
		if !ownerEntry.IsDir() {
			continue
		}
		owner := ownerEntry.Name()
		ownerDir := filepath.Join(r.baseDir, owner)

		repoEntries, err := os.ReadDir(ownerDir)
		if err != nil {
			r.logger.Warn("skipping owner dir", "owner", owner, "error", err)
			continue
		}

		for _, repoEntry := range repoEntries {
			if !repoEntry.IsDir() {
				continue
			}
			repo := repoEntry.Name()
			repoDir := filepath.Join(ownerDir, repo)

			tsEntries, err := os.ReadDir(repoDir)
			if err != nil {
				r.logger.Warn("skipping repo dir", "repo", owner+"/"+repo, "error", err)
				continue
			}

			for _, tsEntry := range tsEntries {
				if !tsEntry.IsDir() {
					continue
				}
				runDir := filepath.Join(repoDir, tsEntry.Name())
				meta, ok := r.readRunMeta(runDir)
				if !ok {
					continue
				}
				metas = append(metas, meta)
			}
		}
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].StartedAt.After(metas[j].StartedAt)
	})

	return metas, nil
}

// LoadRun reads the full run detail for the given owner, repo, and timestamp.
// Returns an error if the run directory or run.json is missing or unreadable.
func (r *Reader) LoadRun(owner, repo, timestamp string) (*RunDetail, error) {
	// Reject components containing ".." or path separators (same guard as writer.go).
	for _, part := range []string{owner, repo, timestamp} {
		if part == "" {
			return nil, fmt.Errorf("invalid path component: must not be empty")
		}
		if strings.Contains(part, "..") {
			return nil, fmt.Errorf("invalid path component: must not contain ..: %q", part)
		}
		if strings.ContainsAny(part, `\/`) {
			return nil, fmt.Errorf("invalid path component: must not contain path separators: %q", part)
		}
	}
	runDir := filepath.Join(r.baseDir, owner, repo, timestamp)

	metaData, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		return nil, fmt.Errorf("reading run.json: %w", err)
	}

	var meta RunMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("parsing run.json: %w", err)
	}

	detail := &RunDetail{RunMeta: meta}

	issuesDir := filepath.Join(runDir, "issues")
	issueEntries, err := os.ReadDir(issuesDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading issues dir: %w", err)
	}

	seen := make(map[int]bool)
	for _, issueEntry := range issueEntries {
		if !issueEntry.IsDir() {
			continue
		}
		issueNumStr := issueEntry.Name()
		issueNum, err := strconv.Atoi(issueNumStr)
		if err != nil {
			r.logger.Warn("skipping non-numeric issue dir", "dir", issueNumStr)
			continue
		}
		issueDir := filepath.Join(issuesDir, issueNumStr)
		detail.Issues = append(detail.Issues, r.loadIssueDetail(issueDir, issueNum))
		seen[issueNum] = true
	}

	// Add placeholder entries for issues listed in run.json but without a
	// directory yet (e.g., still running, no hook data written).
	for _, num := range meta.IssueNumbers {
		if !seen[num] {
			detail.Issues = append(detail.Issues, IssueDetail{IssueNumber: num})
		}
	}

	// Compute BlockedBy for each incomplete issue using the stored dep graph.
	r.computeBlockedBy(detail.Issues, meta.IssueDeps)

	return detail, nil
}

// computeBlockedBy populates BlockedBy on each issue that has unresolved
// dependencies. An issue is considered resolved if its Outcome.Status is
// non-empty (i.e., outcome.json has been written).
func (r *Reader) computeBlockedBy(issues []IssueDetail, deps []IssueDep) {
	if len(deps) == 0 {
		return
	}

	// Build a set of completed issue numbers.
	completed := make(map[int]bool, len(issues))
	for _, issue := range issues {
		if issue.Outcome.Status != "" {
			completed[issue.IssueNumber] = true
		}
	}

	// Build a map from issue number to its declared dependencies.
	depsMap := make(map[int][]int, len(deps))
	for _, d := range deps {
		depsMap[d.IssueNumber] = d.DependsOn
	}

	// Populate BlockedBy for each issue without a final outcome.
	for i := range issues {
		if issues[i].Outcome.Status != "" {
			continue // already finished
		}
		var openDeps []int
		for _, depNum := range depsMap[issues[i].IssueNumber] {
			if !completed[depNum] {
				openDeps = append(openDeps, depNum)
			}
		}
		issues[i].BlockedBy = openDeps
	}
}

// readRunMeta reads and parses run.json from runDir. Returns (meta, true) on
// success, or (zero, false) with a logged warning on any error.
func (r *Reader) readRunMeta(runDir string) (RunMeta, bool) {
	path := filepath.Join(runDir, "run.json")
	data, err := os.ReadFile(path)
	if err != nil {
		r.logger.Warn("skipping run: cannot read run.json", "dir", runDir, "error", err)
		return RunMeta{}, false
	}

	var meta RunMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		r.logger.Warn("skipping run: corrupt run.json", "dir", runDir, "error", err)
		return RunMeta{}, false
	}

	return meta, true
}

// loadIssueDetail reads all files for one issue directory.
// Missing optional files produce zero-value fields; corrupt files are warned and skipped.
func (r *Reader) loadIssueDetail(issueDir string, issueNum int) IssueDetail {
	return IssueDetail{
		IssueNumber:      issueNum,
		SpecGenerator:    r.readStep(filepath.Join(issueDir, "spec-generator.json")),
		Implement:        r.readStep(filepath.Join(issueDir, "implement.json")),
		QualityReview:    r.readStep(filepath.Join(issueDir, "quality-review.json")),
		FunctionalReview: r.readStep(filepath.Join(issueDir, "functional-review.json")),
		Outcome:          r.readOutcome(filepath.Join(issueDir, "outcome.json")),
		LiveStatus:       r.readIssueStatus(filepath.Join(issueDir, "status.json")),
		Retries:          r.loadRetries(filepath.Join(issueDir, "retries")),
		Punchlist:        r.readPunchlist(filepath.Join(issueDir, "punchlist.json")),
		Dialogue:         r.readDialogue(filepath.Join(issueDir, "dialogue.json")),
		VerifyResults:    r.loadVerifyResults(issueDir),
	}
}

// readIssueStatus reads an IssueStatus from path. Returns zero value if the
// file is missing or corrupt (corrupt files are logged as a warning).
func (r *Reader) readIssueStatus(path string) IssueStatus {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return IssueStatus{}
	}
	if err != nil {
		r.logger.Warn("skipping status file", "path", path, "error", err)
		return IssueStatus{}
	}

	var status IssueStatus
	if err := json.Unmarshal(data, &status); err != nil {
		r.logger.Warn("corrupt status file, using zero value", "path", path, "error", err)
		return IssueStatus{}
	}
	return status
}

// readPunchlist reads a PunchlistData from path. Returns nil if the file is
// missing or corrupt (corrupt files are logged as a warning).
func (r *Reader) readPunchlist(path string) *PunchlistData {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.logger.Warn("skipping punchlist file", "path", path, "error", err)
		return nil
	}

	var pl PunchlistData
	if err := json.Unmarshal(data, &pl); err != nil {
		r.logger.Warn("corrupt punchlist file, skipping", "path", path, "error", err)
		return nil
	}
	return &pl
}

// readDialogue reads dialogue entries from path. Returns nil if the file is
// missing or corrupt (corrupt files are logged as a warning).
func (r *Reader) readDialogue(path string) []DialogueEntry {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.logger.Warn("skipping dialogue file", "path", path, "error", err)
		return nil
	}

	var entries []DialogueEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		r.logger.Warn("corrupt dialogue file, skipping", "path", path, "error", err)
		return nil
	}
	return entries
}

// readStep reads a StepResult from path. Returns zero value if the file is
// missing or corrupt (corrupt files are logged as a warning).
func (r *Reader) readStep(path string) StepResult {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return StepResult{}
	}
	if err != nil {
		r.logger.Warn("skipping step file", "path", path, "error", err)
		return StepResult{}
	}

	var step StepResult
	if err := json.Unmarshal(data, &step); err != nil {
		r.logger.Warn("corrupt step file, using zero value", "path", path, "error", err)
		return StepResult{}
	}
	return step
}

// readOutcome reads an Outcome from path. Returns zero value if the file is
// missing or corrupt (corrupt files are logged as a warning).
func (r *Reader) readOutcome(path string) Outcome {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Outcome{}
	}
	if err != nil {
		r.logger.Warn("skipping outcome file", "path", path, "error", err)
		return Outcome{}
	}

	var outcome Outcome
	if err := json.Unmarshal(data, &outcome); err != nil {
		r.logger.Warn("corrupt outcome file, using zero value", "path", path, "error", err)
		return Outcome{}
	}
	return outcome
}

// loadVerifyResults reads all verify-N.json files from issueDir, sorted by attempt number.
// Returns nil if no verify files are present (backwards compatible).
func (r *Reader) loadVerifyResults(issueDir string) []VerifyStepResult {
	entries, err := os.ReadDir(issueDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.logger.Warn("skipping verify results", "dir", issueDir, "error", err)
		return nil
	}

	var results []VerifyStepResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "verify-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		numStr := strings.TrimSuffix(strings.TrimPrefix(name, "verify-"), ".json")
		attempt, err := strconv.Atoi(numStr)
		if err != nil {
			r.logger.Warn("skipping non-numeric verify file", "file", name)
			continue
		}
		path := filepath.Join(issueDir, name)
		vr := r.readVerifyResult(path, attempt)
		results = append(results, vr)
	}

	if len(results) == 0 {
		return nil
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Attempt < results[j].Attempt
	})
	return results
}

// readVerifyResult reads a VerifyStepResult from path. On missing or corrupt files,
// returns a zero-value VerifyStepResult with the given attempt number.
func (r *Reader) readVerifyResult(path string, attempt int) VerifyStepResult {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return VerifyStepResult{Attempt: attempt}
	}
	if err != nil {
		r.logger.Warn("skipping verify file", "path", path, "error", err)
		return VerifyStepResult{Attempt: attempt}
	}

	var vr VerifyStepResult
	if err := json.Unmarshal(data, &vr); err != nil {
		r.logger.Warn("corrupt verify file, using zero value", "path", path, "error", err)
		return VerifyStepResult{Attempt: attempt}
	}
	return vr
}

// loadRetries reads all retry step results from retriesDir, sorted by attempt number.
// Returns nil if the directory is absent.
func (r *Reader) loadRetries(retriesDir string) []RetryDetail {
	entries, err := os.ReadDir(retriesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.logger.Warn("skipping retries dir", "dir", retriesDir, "error", err)
		return nil
	}

	var retries []RetryDetail
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		attempt, err := strconv.Atoi(entry.Name())
		if err != nil {
			r.logger.Warn("skipping non-numeric retry dir", "dir", entry.Name())
			continue
		}
		retryDir := filepath.Join(retriesDir, entry.Name())
		retries = append(retries, RetryDetail{
			Attempt:          attempt,
			Retry:            r.readStep(filepath.Join(retryDir, "retry.json")),
			QualityReview:    r.readStep(filepath.Join(retryDir, "quality-review.json")),
			FunctionalReview: r.readStep(filepath.Join(retryDir, "functional-review.json")),
		})
	}

	sort.Slice(retries, func(i, j int) bool {
		return retries[i].Attempt < retries[j].Attempt
	})

	return retries
}
