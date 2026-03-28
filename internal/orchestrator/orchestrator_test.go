package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/agent"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/logging"
	"github.com/peter-stratton/dark-factory/internal/notify"
	"github.com/peter-stratton/dark-factory/internal/progress"
	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
)

// fakeNotifier records every event received and optionally returns an error.
type fakeNotifier struct {
	received []notify.Event
	err      error
}

func (f *fakeNotifier) Send(_ context.Context, event notify.Event) error {
	if f.err != nil {
		return f.err
	}
	f.received = append(f.received, event)
	return nil
}

// fakeReporter records all reporter calls for test assertions.
type fakeReporter struct {
	issueCompleted []fakeIssueCompleted
	waveStarted    []fakeWaveStarted
	runFinished    []fakeRunFinished
	allBlocked     []fakeAllBlocked
	rollupCreated  []fakeRollupCreated
	punchlistTexts []string
}

type fakeIssueCompleted struct {
	issueNumber int
	title       string
	status      string
	prNumber    int
	retries     int
	errMsg      string
	costUSD     float64
	traceID     string
}

type fakeWaveStarted struct {
	wave  int
	count int
}

type fakeRunFinished struct {
	implemented      int
	readyToMerge     int
	needsHumanReview int
	failed           int
	blocked          int
}

type fakeAllBlocked struct {
	total   int
	blocked int
}

type fakeRollupCreated struct {
	prNumber int
	prURL    string
	merged   bool
}

func (r *fakeReporter) RunStarted(_, _, _, _, _, _ string, _ []progress.IssueSummary) {}
func (r *fakeReporter) IssueStarted(_ int, _ string)              {}
func (r *fakeReporter) IssueStageChanged(_ int, _ string)         {}
func (r *fakeReporter) IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string, costUSD float64, traceID string) {
	r.issueCompleted = append(r.issueCompleted, fakeIssueCompleted{issueNumber, title, status, prNumber, retries, errMsg, costUSD, traceID})
}
func (r *fakeReporter) WaveStarted(wave, count int) {
	r.waveStarted = append(r.waveStarted, fakeWaveStarted{wave, count})
}
func (r *fakeReporter) AllBlocked(total, blocked int) {
	r.allBlocked = append(r.allBlocked, fakeAllBlocked{total, blocked})
}
func (r *fakeReporter) RollupCreated(prNumber int, prURL string, merged bool) {
	r.rollupCreated = append(r.rollupCreated, fakeRollupCreated{prNumber, prURL, merged})
}
func (r *fakeReporter) RunFinished(implemented, readyToMerge, needsHumanReview, failed, blocked int) {
	r.runFinished = append(r.runFinished, fakeRunFinished{implemented, readyToMerge, needsHumanReview, failed, blocked})
}
func (r *fakeReporter) JudgeIntervention(_ int, _, _, _, _ string) {}
func (r *fakeReporter) PunchlistText(text string) {
	r.punchlistTexts = append(r.punchlistTexts, text)
}

// ghIssue mirrors the JSON shape for test fixtures.
type ghIssue struct {
	Number int       `json:"number"`
	Title  string    `json:"title"`
	Body   string    `json:"body"`
	Labels []ghLabel `json:"labels"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghClosedIssue struct {
	Number int `json:"number"`
}

// setupFakeGH installs a fake CommandRunner that returns different JSON
// depending on whether the caller asks for open or closed issues.
func setupFakeGH(t *testing.T, openIssues []ghIssue, closedNumbers []int) {
	t.Helper()

	closedIssues := make([]ghClosedIssue, len(closedNumbers))
	for i, n := range closedNumbers {
		closedIssues[i] = ghClosedIssue{Number: n}
	}

	orig := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = orig })

	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		// Determine if this is a closed-issue query.
		for i, a := range args {
			if a == "--state" && i+1 < len(args) && args[i+1] == "closed" {
				return json.Marshal(closedIssues)
			}
		}
		return json.Marshal(openIssues)
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Repo: "owner/repo",
	}
}

// recordingHandler is a slog.Handler that captures all log records in memory.
type recordingHandler struct {
	records []slog.Record
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler      { return h }

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	dir := t.TempDir()
	logger, err := logging.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	return logger
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 65536)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func TestDryRun_ListsIssuesInOrder(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 3, Title: "third", Labels: []ghLabel{{Name: "p2"}}},
		{Number: 1, Title: "first", Labels: []ghLabel{{Name: "p1"}}},
		{Number: 2, Title: "second", Labels: []ghLabel{{Name: "p1"}}},
	}
	setupFakeGH(t, openIssues, nil)

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	// Verify all issues appear and in priority order (p1 first).
	idx1 := strings.Index(output, "#1 first")
	idx2 := strings.Index(output, "#2 second")
	idx3 := strings.Index(output, "#3 third")
	if idx1 == -1 || idx2 == -1 || idx3 == -1 {
		t.Fatalf("expected all issues in output, got:\n%s", output)
	}
	if idx1 > idx2 || idx2 > idx3 {
		t.Errorf("issues not in priority/number order:\n%s", output)
	}
}

func TestDryRun_BlockedIssuesShownSeparately(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "unblocked", Labels: []ghLabel{}},
		{Number: 2, Title: "blocked one", Body: "**Blocked by**: #99", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil) // #99 not closed

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if !strings.Contains(output, "Processable issues:") {
		t.Error("expected 'Processable issues:' section")
	}
	if !strings.Contains(output, "Blocked issues:") {
		t.Error("expected 'Blocked issues:' section")
	}
	if !strings.Contains(output, "#2 blocked one (blocked by: #99)") {
		t.Errorf("expected blocked issue with reason, got:\n%s", output)
	}
}

func TestDryRun_SummaryCounts(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "processable", Labels: []ghLabel{}},
		{Number: 2, Title: "blocked", Body: "**Blocked by**: #99", Labels: []ghLabel{}},
		{Number: 3, Title: "also processable", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	expected := "Summary: 3 total, 1 blocked, 2 processable"
	if !strings.Contains(output, expected) {
		t.Errorf("expected summary %q, got:\n%s", expected, output)
	}
}

func TestDryRun_LogFileCreated(t *testing.T) {
	dir := t.TempDir()
	setupFakeGH(t, []ghIssue{
		{Number: 1, Title: "test", Labels: []ghLabel{}},
	}, nil)

	logger, err := logging.NewLogger(dir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	captureStdout(t, func() {
		if err := Run(context.Background(), testConfig(), logger, progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, ""); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	debugLog := filepath.Join(dir, "debug.log")
	if _, err := os.Stat(debugLog); os.IsNotExist(err) {
		t.Fatal("expected debug.log to be created")
	}
}

func TestEmptyMilestone(t *testing.T) {
	setupFakeGH(t, []ghIssue{}, nil)

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if !strings.Contains(output, "No issues found") {
		t.Errorf("expected 'No issues found' message, got:\n%s", output)
	}
}

func TestAllBlocked(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "blocked a", Body: "**Blocked by**: #90", Labels: []ghLabel{}},
		{Number: 2, Title: "blocked b", Body: "**Blocked by**: #91", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	// Stub CommandRunner so CheckWorkingTree sees a clean working tree.
	origCmd := CommandRunner
	t.Cleanup(func() { CommandRunner = origCmd })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	// Run creates a RunDataWriter when dryRun=false; disable it for this test.
	origWriter := newRunDataWriterFn
	t.Cleanup(func() { newRunDataWriterFn = origWriter })
	newRunDataWriterFn = func(repo, milestone string, issueNums []int, baseBranch string, autoMerge rundata.AutoMerge) (*rundata.Writer, error) {
		return nil, fmt.Errorf("disabled in test")
	}

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", false, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if !strings.Contains(output, "All issues are blocked") {
		t.Errorf("expected 'All issues are blocked' message, got:\n%s", output)
	}
	if !strings.Contains(output, "Summary: 2 total, 2 blocked, 0 processable") {
		t.Errorf("expected correct summary, got:\n%s", output)
	}
}

func TestDryRun_ClosedDepsUnblock(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 5, Title: "depends on closed", Body: "**Blocked by**: #3", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, []int{3}) // #3 is closed

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if strings.Contains(output, "Blocked issues:") {
		t.Error("expected no blocked issues when deps are closed")
	}
	if !strings.Contains(output, "Summary: 1 total, 0 blocked, 1 processable") {
		t.Errorf("expected all issues processable, got:\n%s", output)
	}
}

func TestFormatIssueRefs(t *testing.T) {
	tests := []struct {
		input []int
		want  string
	}{
		{[]int{1}, "#1"},
		{[]int{1, 2, 3}, "#1, #2, #3"},
	}
	for _, tt := range tests {
		got := formatIssueRefs(tt.input)
		if got != tt.want {
			t.Errorf("formatIssueRefs(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestOpenDependencies(t *testing.T) {
	closed := map[int]bool{1: true, 3: true}
	open := openDependencies([]int{1, 2, 3, 4}, closed)
	if len(open) != 2 || open[0] != 2 || open[1] != 4 {
		t.Errorf("openDependencies = %v, want [2 4]", open)
	}
}

func TestDryRun_PriorityDisplayed(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "high pri", Labels: []ghLabel{{Name: "p1"}}},
		{Number: 2, Title: "no pri", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if !strings.Contains(output, "[priority: p1]") {
		t.Error("expected priority p1 to be displayed")
	}
	if !strings.Contains(output, "[priority: none]") {
		t.Error("expected priority 'none' for unlabeled issues")
	}
}

func TestCategorizeIssues(t *testing.T) {
	closedSet := map[int]bool{10: true}
	issues := []github.Issue{
		{Number: 1, Title: "free"},
		{Number: 2, Title: "blocked by 10", Body: "**Blocked by**: #10"},
		{Number: 3, Title: "blocked by 99", Body: "**Blocked by**: #99"},
	}

	processable, blocked := categorizeIssues(issues, closedSet)
	if len(processable) != 2 {
		t.Fatalf("expected 2 processable, got %d", len(processable))
	}
	if processable[0].Number != 1 || processable[1].Number != 2 {
		t.Errorf("expected issues 1 and 2 processable, got %v", processable)
	}
	if len(blocked) != 1 || blocked[0].Issue.Number != 3 {
		t.Errorf("expected issue 3 blocked, got %v", blocked)
	}
}

// setupProcessMocks configures all the mocks needed to test processIssues.
// It returns a cleanup function. closedNumbersFn is called each time
// FetchClosedIssueNumbers is invoked (to simulate issues closing over time).
func setupProcessMocks(t *testing.T, closedNumbersFn func() []int, processFn func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome) {
	t.Helper()

	// Stub buildImageFn so tests don't require a real Docker daemon.
	origBuildImage := buildImageFn
	t.Cleanup(func() { buildImageFn = origBuildImage })
	buildImageFn = func(_ context.Context, _ sandbox.DockerConfig, _ *slog.Logger) (string, error) {
		return "test-image:latest", nil
	}

	// Mock processIssueFn.
	origProcess := processIssueFn
	t.Cleanup(func() { processIssueFn = origProcess })
	processIssueFn = processFn

	// Mock orchestrator.CommandRunner (used by PullAfterMerge).
	origCmdRunner := CommandRunner
	t.Cleanup(func() { CommandRunner = origCmdRunner })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	// Mock github.CommandRunner (used by lock, FetchClosedIssueNumbers).
	origGHRunner := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origGHRunner })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		// FetchClosedIssueNumbers: state=closed query.
		for i, a := range args {
			if a == "--state" && i+1 < len(args) && args[i+1] == "closed" {
				numbers := closedNumbersFn()
				type num struct {
					Number int `json:"number"`
				}
				items := make([]num, len(numbers))
				for j, n := range numbers {
					items[j] = num{Number: n}
				}
				return json.Marshal(items)
			}
		}
		// EnsureLabel, AddIssueLabel, RemoveIssueLabel, FindIssuesWithLabel:
		// return empty list / success.
		return []byte("[]"), nil
	}

	// Auth env vars so CollectAuthEnv doesn't fail.
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("GH_TOKEN", "test-gh-token")
}

func TestProcessIssues_MultiWaveReResolution(t *testing.T) {
	// Issue 1: no deps (processable).
	// Issue 2: blocked by #1. After #1 is "implemented" (merged & closed),
	// #2 should become processable in wave 2.
	allIssues := []github.Issue{
		{Number: 1, Title: "first task"},
		{Number: 2, Title: "second task", Body: "**Blocked by**: #1"},
	}

	// Track which issues were processed.
	var processedNumbers []int
	callCount := 0

	closedNumbers := []int{} // initially nothing closed
	setupProcessMocks(t, func() []int {
		return closedNumbers
	}, func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome {
		callCount++
		processedNumbers = append(processedNumbers, issue.Number)
		if issue.Number == 1 {
			// Simulate merge: mark #1 as closed for re-resolution.
			closedNumbers = []int{1}
			return agent.IssueOutcome{
				IssueNumber: 1,
				Status:      "implemented",
				PRNumber:    101,
			}
		}
		// Issue 2: also succeeds.
		return agent.IssueOutcome{
			IssueNumber: 2,
			Status:      "implemented",
			PRNumber:    102,
		}
	})

	closedSet := map[int]bool{} // initially empty
	cfg := testConfig()

	output := captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
		if err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	// Both issues should have been processed.
	if len(processedNumbers) != 2 {
		t.Fatalf("expected 2 issues processed, got %d: %v", len(processedNumbers), processedNumbers)
	}
	if processedNumbers[0] != 1 || processedNumbers[1] != 2 {
		t.Errorf("expected issues processed in order [1, 2], got %v", processedNumbers)
	}

	// Output should mention wave 2.
	if !strings.Contains(output, "Wave 2") {
		t.Errorf("expected 'Wave 2' in output, got:\n%s", output)
	}
	// Both implemented.
	if !strings.Contains(output, "2 implemented") {
		t.Errorf("expected '2 implemented' in output, got:\n%s", output)
	}
}

func TestProcessIssues_AllFailNoInfiniteLoop(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "will fail"},
		{Number: 2, Title: "also fails"},
	}

	var processedNumbers []int
	setupProcessMocks(t, func() []int {
		return nil
	}, func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome {
		processedNumbers = append(processedNumbers, issue.Number)
		return agent.IssueOutcome{
			IssueNumber: issue.Number,
			Status:      "failed",
			Err:         fmt.Errorf("test failure"),
		}
	})

	closedSet := map[int]bool{}
	cfg := testConfig()

	output := captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "", nil)
		if err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	// Each issue processed exactly once.
	if len(processedNumbers) != 2 {
		t.Fatalf("expected 2 issues processed, got %d: %v", len(processedNumbers), processedNumbers)
	}

	// No second wave.
	if strings.Contains(output, "Wave 2") {
		t.Errorf("expected no second wave when no merges, got:\n%s", output)
	}
	if !strings.Contains(output, "2 failed") {
		t.Errorf("expected '2 failed' in output, got:\n%s", output)
	}
}

func TestProcessIssues_FinalizeRunCalled(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 10, Title: "implemented issue"},
		{Number: 11, Title: "failed issue"},
	}

	setupProcessMocks(t, func() []int {
		return nil
	}, func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome {
		if issue.Number == 10 {
			return agent.IssueOutcome{IssueNumber: 10, Status: "implemented", PRNumber: 100}
		}
		return agent.IssueOutcome{IssueNumber: 11, Status: "failed", Err: fmt.Errorf("boom")}
	})

	// Create a real RunDataWriter in a temp HOME so we can verify FinalizeRun.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	writer, err := rundata.New("owner/repo", "test-milestone", []int{10, 11}, "", rundata.AutoMerge{})
	if err != nil {
		t.Fatalf("rundata.New: %v", err)
	}

	closedSet := map[int]bool{}
	cfg := testConfig()

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), writer, false, "", "test-milestone", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	// Read run.json directly from the writer's directory.
	data, err := os.ReadFile(filepath.Join(writer.Dir(), "run.json"))
	if err != nil {
		t.Fatalf("reading run.json: %v", err)
	}

	var meta struct {
		FinishedAt *string `json:"finished_at"`
		Summary    *struct {
			Total       int `json:"total"`
			Implemented int `json:"implemented"`
			Failed      int `json:"failed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing run.json: %v", err)
	}

	if meta.FinishedAt == nil {
		t.Error("run.json missing finished_at — FinalizeRun was not called")
	}
	if meta.Summary == nil {
		t.Fatal("run.json missing summary — FinalizeRun was not called")
	}
	if meta.Summary.Implemented != 1 {
		t.Errorf("summary.implemented = %d, want 1", meta.Summary.Implemented)
	}
	if meta.Summary.Failed != 1 {
		t.Errorf("summary.failed = %d, want 1", meta.Summary.Failed)
	}
	if meta.Summary.Total != 2 {
		t.Errorf("summary.total = %d, want 2", meta.Summary.Total)
	}
}

const testImplBody1 = "## Implementation Notes\n\n### Approach\nFirst implementation.\n\n### Key Decisions\nDecision A.\n\n### Known Limitations\nNone.\n\n### Architecture\nDomain layer.\n"
const testImplBody2 = "## Implementation Notes\n\n### Approach\nFixed implementation.\n\n### Key Decisions\nDecision B.\n\n### Known Limitations\nNone.\n\n### Architecture\nDomain layer.\n"
const testQualityBody = "## Quality Review Notes\n\n### Issues Found\nNeeds fixes.\n\n### Changes Requested\nYes.\n"
const testReviewBody = "## Review Notes\n\n### Approved\nLooks good.\n\n### Changes Requested\n\n### Architecture Compliance\nCompliant.\n"

func TestBuildDialogueEntries_SingleRound(t *testing.T) {
	bodies := []string{testImplBody1, testReviewBody}
	entries := BuildDialogueEntries(bodies)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Role != "implementer" || entries[0].Round != 1 || entries[0].Body != testImplBody1 || entries[0].Outcome != "" {
		t.Errorf("entries[0]: got {%s, %d, %q, %q}", entries[0].Role, entries[0].Round, entries[0].Body, entries[0].Outcome)
	}
	if entries[1].Role != "reviewer" || entries[1].Round != 1 || entries[1].Body != testReviewBody || entries[1].Outcome != "accepted" {
		t.Errorf("entries[1]: got {%s, %d, %q, %q}", entries[1].Role, entries[1].Round, entries[1].Body, entries[1].Outcome)
	}
}

func TestBuildDialogueEntries_MultipleRounds(t *testing.T) {
	// Quality retry: impl1 → quality(fail) → impl2 → review(pass)
	bodies := []string{testImplBody1, testQualityBody, testImplBody2, testReviewBody}
	entries := BuildDialogueEntries(bodies)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	want := []struct {
		role    string
		round   int
		body    string
		outcome string
	}{
		{"implementer", 1, testImplBody1, ""},
		{"quality_reviewer", 1, testQualityBody, "changes_requested"},
		{"implementer", 2, testImplBody2, ""},
		{"reviewer", 1, testReviewBody, "accepted"},
	}
	for i, w := range want {
		if entries[i].Role != w.role || entries[i].Round != w.round || entries[i].Body != w.body || entries[i].Outcome != w.outcome {
			t.Errorf("entries[%d]: got {%s, %d, %q, %q}, want {%s, %d, %q, %q}",
				i, entries[i].Role, entries[i].Round, entries[i].Body, entries[i].Outcome,
				w.role, w.round, w.body, w.outcome)
		}
	}
}

func TestBuildDialogueEntries_Empty(t *testing.T) {
	entries := BuildDialogueEntries(nil)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(entries))
	}
}

func TestProcessIssues_WritesDialogue(t *testing.T) {
	implComment := "## Implementation Notes\n\n### Approach\nDid the thing.\n\n### Key Decisions\nUsed X.\n\n### Known Limitations\nNone.\n\n### Architecture\nDomain layer.\n"
	reviewComment := "## Review Notes\n\n### Approved\nYes.\n\n### Changes Requested\n\n### Architecture Compliance\nGood.\n"

	allIssues := []github.Issue{
		{Number: 5, Title: "test issue"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: 5, Status: "implemented", PRNumber: 77}
		})

	// Mock fetchPRCommentBodiesFn.
	origFetch := fetchPRCommentBodiesFn
	t.Cleanup(func() { fetchPRCommentBodiesFn = origFetch })
	fetchPRCommentBodiesFn = func(repo string, prNum int) ([]string, error) {
		return []string{implComment, reviewComment}, nil
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	writer, err := rundata.New("owner/repo", "test-milestone", []int{5}, "", rundata.AutoMerge{})
	if err != nil {
		t.Fatalf("rundata.New: %v", err)
	}

	closedSet := map[int]bool{}
	cfg := testConfig()

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), writer, false, "", "test-milestone", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	// Read dialogue.json from the issue directory.
	dialoguePath := filepath.Join(writer.Dir(), "issues", "5", "dialogue.json")
	data, err := os.ReadFile(dialoguePath)
	if err != nil {
		t.Fatalf("reading dialogue.json: %v", err)
	}

	var entries []rundata.DialogueEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("parsing dialogue.json: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 dialogue entries, got %d", len(entries))
	}
	if entries[0].Role != "implementer" || entries[0].Round != 1 {
		t.Errorf("entries[0]: got role=%q round=%d, want implementer/1", entries[0].Role, entries[0].Round)
	}
	if entries[1].Role != "reviewer" || entries[1].Round != 1 {
		t.Errorf("entries[1]: got role=%q round=%d, want reviewer/1", entries[1].Role, entries[1].Round)
	}
}

func TestPunchlistEnrichmentStatus_Skipped(t *testing.T) {
	prompts := &agent.Prompts{Punchlist: ""}
	status := punchlistEnrichmentStatus(prompts, nil)
	if status != "skipped" {
		t.Errorf("expected %q, got %q", "skipped", status)
	}
}

func TestPunchlistEnrichmentStatus_Success(t *testing.T) {
	prompts := &agent.Prompts{Punchlist: "some prompt"}
	status := punchlistEnrichmentStatus(prompts, []string{"Test one"})
	if status != "success" {
		t.Errorf("expected %q, got %q", "success", status)
	}
}

func TestPunchlistEnrichmentStatus_Failed(t *testing.T) {
	prompts := &agent.Prompts{Punchlist: "some prompt"}
	status := punchlistEnrichmentStatus(prompts, nil)
	if status != "failed" {
		t.Errorf("expected %q, got %q", "failed", status)
	}
}

func TestPunchlistEnrichmentStatus_SuccessEmptySlice(t *testing.T) {
	// An empty (non-nil) slice counts as success — the LLM ran but returned
	// zero tests, which is different from a parse error (nil).
	prompts := &agent.Prompts{Punchlist: "some prompt"}
	status := punchlistEnrichmentStatus(prompts, []string{})
	if status != "success" {
		t.Errorf("expected %q for empty non-nil slice, got %q", "success", status)
	}
}

func TestCheckWorkingTree_DirtyReturnsError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(" M docs/README.md\n?? scratch.txt\n"), nil
	}

	err := CheckWorkingTree()
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
	if !strings.Contains(err.Error(), "working tree is dirty") {
		t.Errorf("error should mention 'working tree is dirty', got: %v", err)
	}
	if !strings.Contains(err.Error(), "docs/README.md") {
		t.Errorf("error should list dirty files, got: %v", err)
	}
	if !strings.Contains(err.Error(), "scratch.txt") {
		t.Errorf("error should list all dirty files, got: %v", err)
	}
}

func TestCheckWorkingTree_CleanReturnsNil(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	if err := CheckWorkingTree(); err != nil {
		t.Errorf("expected nil error for clean tree, got: %v", err)
	}
}

func TestCheckWorkingTree_StagedFilesBlocked(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	// git status --porcelain shows staged files with 'A' or 'M' in the first column.
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("A  staged-new-file.go\nM  modified-staged.go\n"), nil
	}

	err := CheckWorkingTree()
	if err == nil {
		t.Fatal("expected error for staged (uncommitted) changes")
	}
	if !strings.Contains(err.Error(), "working tree is dirty") {
		t.Errorf("expected 'working tree is dirty', got: %v", err)
	}
}

func TestCheckWorkingTree_CommandError(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("git not found")
	}

	err := CheckWorkingTree()
	if err == nil {
		t.Fatal("expected error when git command fails")
	}
	if !strings.Contains(err.Error(), "checking working tree") {
		t.Errorf("expected 'checking working tree' in error, got: %v", err)
	}
}

func TestRun_DirtyTreeBlocksRun(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "test", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(" M docs/README.md\n"), nil
	}

	captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", false, false, "")
		if err == nil {
			t.Fatal("expected error for dirty working tree")
		}
		if !strings.Contains(err.Error(), "working tree is dirty") {
			t.Errorf("expected 'working tree is dirty', got: %v", err)
		}
		if !strings.Contains(err.Error(), "docs/README.md") {
			t.Errorf("expected dirty file name in error, got: %v", err)
		}
	})
}

func TestRun_DirtyTreeErrorListsFiles(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "test", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(" M file-a.go\n?? file-b.txt\n"), nil
	}

	captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", false, false, "")
		if err == nil {
			t.Fatal("expected error for dirty working tree")
		}
		if !strings.Contains(err.Error(), "file-a.go") {
			t.Errorf("expected 'file-a.go' in error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "file-b.txt") {
			t.Errorf("expected 'file-b.txt' in error, got: %v", err)
		}
	})
}

func TestRun_DryRunSkipsDirtyCheck(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "test", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	// CommandRunner would return dirty if called, but dry-run must skip the check.
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(" M dirty.txt\n"), nil
	}

	captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("dry-run should not fail on dirty tree: %v", err)
		}
	})
}

func TestPullAfterMerge_CustomBranch(t *testing.T) {
	var capturedArgs []string
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	if err := PullAfterMerge("feature/foo", testLogger(t)); err != nil {
		t.Fatalf("PullAfterMerge() unexpected error: %v", err)
	}

	if len(capturedArgs) < 4 || capturedArgs[3] != "feature/foo" {
		t.Errorf("expected branch 'feature/foo' in git args, got %v", capturedArgs)
	}
}

func TestPullAfterMerge_MainBranch(t *testing.T) {
	var capturedArgs []string
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte(""), nil
	}

	if err := PullAfterMerge("main", testLogger(t)); err != nil {
		t.Fatalf("PullAfterMerge() unexpected error: %v", err)
	}

	if len(capturedArgs) < 4 || capturedArgs[3] != "main" {
		t.Errorf("expected branch 'main' in git args, got %v", capturedArgs)
	}
}

func TestPullAfterMerge_DirtyRepoWarning(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "pull" && args[1] == "--rebase" {
			return nil, fmt.Errorf("pull failed")
		}
		// git status --porcelain: dirty tree
		return []byte(" M some-file.go\n"), nil
	}

	h := &recordingHandler{}
	err := PullAfterMerge("feature/xyz", slog.New(h))
	if err == nil {
		t.Fatal("expected error for dirty repo")
	}
	if !strings.Contains(err.Error(), "dirty") {
		t.Errorf("expected 'dirty' in error, got: %v", err)
	}

	// Verify the warning carries "branch" as a structured attribute.
	var foundBranch bool
	for _, rec := range h.records {
		if rec.Level != slog.LevelWarn {
			continue
		}
		rec.Attrs(func(a slog.Attr) bool {
			if a.Key == "branch" && a.Value.String() == "feature/xyz" {
				foundBranch = true
				return false
			}
			return true
		})
	}
	if !foundBranch {
		t.Error("expected warning to include branch=feature/xyz structured attribute")
	}
}

func TestEnsureBaseBranch_SkipsEmptyBranch(t *testing.T) {
	// CommandRunner must not be called when branch is empty.
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	called := false
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte(""), nil
	}

	if err := EnsureBaseBranch("", testLogger(t)); err != nil {
		t.Fatalf("EnsureBaseBranch() unexpected error: %v", err)
	}
	if called {
		t.Error("expected CommandRunner not to be called for empty branch")
	}
}

func TestEnsureBaseBranch_SkipsExistingBranch(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	var pushedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) >= 1 && args[0] == "ls-remote" {
			// Branch exists on remote.
			return []byte("abc123\trefs/heads/feature/existing\n"), nil
		}
		if len(args) >= 1 && args[0] == "push" {
			pushedArgs = args
		}
		return []byte(""), nil
	}

	if err := EnsureBaseBranch("feature/existing", testLogger(t)); err != nil {
		t.Fatalf("EnsureBaseBranch() unexpected error: %v", err)
	}
	if pushedArgs != nil {
		t.Errorf("expected no git push when branch exists, got push with args %v", pushedArgs)
	}
}

func TestEnsureBaseBranch_CreatesMissingBranch(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })
	var pushedRef string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) >= 1 && args[0] == "ls-remote" {
			// Branch does not exist.
			return []byte(""), nil
		}
		if len(args) >= 1 && args[0] == "push" {
			// Capture the refspec argument.
			for _, a := range args {
				if strings.HasPrefix(a, "HEAD:") {
					pushedRef = a
				}
			}
			return []byte(""), nil
		}
		return []byte(""), nil
	}

	if err := EnsureBaseBranch("feature/new", testLogger(t)); err != nil {
		t.Fatalf("EnsureBaseBranch() unexpected error: %v", err)
	}
	if pushedRef != "HEAD:refs/heads/feature/new" {
		t.Errorf("expected push refspec 'HEAD:refs/heads/feature/new', got %q", pushedRef)
	}
}

// Suppress unused import warnings.
var _ = fmt.Sprintf

func TestProcessIssues_LifecycleLabelsEnsured(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "test issue"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: 1, Status: "implemented", PRNumber: 10}
		})

	// Wrap github.CommandRunner to capture "gh label create" calls,
	// which EnsureLabel invokes when a label is not found.
	var ensuredLabels []string
	prev := github.CommandRunner
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "label" && args[1] == "create" {
			// args: "label" "create" "--repo" "<repo>" "<name>" "--color" ...
			if len(args) >= 5 {
				ensuredLabels = append(ensuredLabels, args[4])
			}
		}
		return prev(name, args...)
	}

	closedSet := map[int]bool{}
	cfg := testConfig()

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	// All three PR lifecycle labels must have been ensured.
	wantLabels := []string{
		"godark:awaiting-human-review",
		"godark:fixing-review-feedback",
		"godark:ready-to-merge",
	}
	for _, want := range wantLabels {
		found := false
		for _, got := range ensuredLabels {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected lifecycle label %q to be ensured (created), got: %v", want, ensuredLabels)
		}
	}
}

func TestFireNotifications_SendsEvent(t *testing.T) {
	fn := &fakeNotifier{}
	event := notify.Event{Type: "run_complete", Repo: "owner/repo", Message: "1 implemented, 0 failed, 0 blocked"}

	notify.Fire(context.Background(), []notify.Notifier{fn}, event, testLogger(t))

	if len(fn.received) != 1 {
		t.Fatalf("got %d events, want 1", len(fn.received))
	}
	if fn.received[0].Type != "run_complete" {
		t.Errorf("event type = %q, want %q", fn.received[0].Type, "run_complete")
	}
	if fn.received[0].Message != event.Message {
		t.Errorf("event message = %q, want %q", fn.received[0].Message, event.Message)
	}
}

func TestFireNotifications_LogsErrorAndContinues(t *testing.T) {
	failing := &fakeNotifier{err: errors.New("network error")}
	ok := &fakeNotifier{}
	event := notify.Event{Type: "abort", Repo: "owner/repo", Message: "reason"}

	// Should not panic or return an error; the ok notifier should still receive the event.
	notify.Fire(context.Background(), []notify.Notifier{failing, ok}, event, testLogger(t))

	if len(ok.received) != 1 {
		t.Errorf("ok notifier got %d events, want 1", len(ok.received))
	}
}

func TestFireNotifications_EmptyNotifiers(t *testing.T) {
	// No notifiers — should not panic.
	notify.Fire(context.Background(), nil, notify.Event{Type: "run_complete"}, testLogger(t))
}

func TestProcessIssues_RunCompleteNotificationFired(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "first"},
		{Number: 2, Title: "second"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome {
			if issue.Number == 1 {
				return agent.IssueOutcome{IssueNumber: 1, Status: "implemented", PRNumber: 10}
			}
			return agent.IssueOutcome{IssueNumber: 2, Status: "failed", Err: fmt.Errorf("boom")}
		})

	fn := &fakeNotifier{}
	closedSet := map[int]bool{}
	cfg := testConfig()

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", []notify.Notifier{fn}); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	// Must have received exactly one run_complete event.
	var rcEvents []notify.Event
	for _, e := range fn.received {
		if e.Type == "run_complete" {
			rcEvents = append(rcEvents, e)
		}
	}
	if len(rcEvents) != 1 {
		t.Fatalf("got %d run_complete events, want 1; all events: %v", len(rcEvents), fn.received)
	}
	// Message should mention implemented and failed counts.
	msg := rcEvents[0].Message
	if !strings.Contains(msg, "implemented") {
		t.Errorf("run_complete message %q missing 'implemented'", msg)
	}
	if !strings.Contains(msg, "failed") {
		t.Errorf("run_complete message %q missing 'failed'", msg)
	}
}


// filteringNotifier only accepts events whose Type is in the allowed set,
// forwarding to the inner fakeNotifier. This simulates notify.filteredNotifier
// for use in orchestrator-layer tests without depending on unexported types.
type filteringNotifier struct {
	inner  *fakeNotifier
	events map[string]bool
}

func (f *filteringNotifier) Send(ctx context.Context, event notify.Event) error {
	if !f.events[event.Type] {
		return nil
	}
	return f.inner.Send(ctx, event)
}

func TestFireNotifications_EventFiltering(t *testing.T) {
	// Notifier subscribed to "abort" only must not receive "run_complete".
	inner := &fakeNotifier{}
	abortOnly := &filteringNotifier{inner: inner, events: map[string]bool{"abort": true}}

	notify.Fire(context.Background(), []notify.Notifier{abortOnly},
		notify.Event{Type: "run_complete", Repo: "owner/repo", Message: "done"}, testLogger(t))

	if len(inner.received) != 0 {
		t.Errorf("abort-only notifier received %d events for run_complete, want 0", len(inner.received))
	}

	// Now fire an abort — it must be received.
	notify.Fire(context.Background(), []notify.Notifier{abortOnly},
		notify.Event{Type: "abort", Repo: "owner/repo", Message: "reason"}, testLogger(t))

	if len(inner.received) != 1 {
		t.Errorf("abort-only notifier received %d events for abort, want 1", len(inner.received))
	}
}

// --- Rollup PR tests ---

// mockConfigDefaultBranch stubs config.CommandRunner so that
// EffectiveDefaultBranch returns the given branch name without calling gh.
func mockConfigDefaultBranch(t *testing.T, branch string) {
	t.Helper()
	orig := config.CommandRunner
	t.Cleanup(func() { config.CommandRunner = orig })
	config.CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(branch + "\n"), nil
	}
}

// setupRollupMocks adds stubs for upsertRollupPRFn and mergeRollupPRFn and
// returns pointers to slices that record each call for inspection.
func setupRollupMocks(t *testing.T) (created *[]string, merged *[]int) {
	t.Helper()

	var createdCalls []string
	var mergedCalls []int

	origUpsert := upsertRollupPRFn
	t.Cleanup(func() { upsertRollupPRFn = origUpsert })
	upsertRollupPRFn = func(repo, baseBranch, defaultBranch, title, body string) (int, string, error) {
		createdCalls = append(createdCalls, fmt.Sprintf("%s->%s", baseBranch, defaultBranch))
		return 999, "https://github.com/owner/repo/pull/999", nil
	}

	origMerge := mergeRollupPRFn
	t.Cleanup(func() { mergeRollupPRFn = origMerge })
	mergeRollupPRFn = func(repo string, prNum int) error {
		mergedCalls = append(mergedCalls, prNum)
		return nil
	}

	return &createdCalls, &mergedCalls
}

func TestRollup_NoneDoesNothing(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		})

	mockConfigDefaultBranch(t, "main")
	created, merged := setupRollupMocks(t)

	cfg := testConfig()
	cfg.BaseBranch = "feature-branch"
	cfg.AutoMerge.Rollup = "none"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 0 {
		t.Errorf("rollup: none should not create a PR, got %v", *created)
	}
	if len(*merged) != 0 {
		t.Errorf("rollup: none should not merge a PR, got %v", *merged)
	}
}

func TestRollup_ManualCreatesPRButDoesNotMerge(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		})

	mockConfigDefaultBranch(t, "main")
	created, merged := setupRollupMocks(t)

	cfg := testConfig()
	cfg.BaseBranch = "feature-branch"
	cfg.AutoMerge.Rollup = "manual"

	output := captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 1 {
		t.Fatalf("rollup: manual should create exactly 1 PR, got %d", len(*created))
	}
	if (*created)[0] != "feature-branch->main" {
		t.Errorf("expected rollup PR from feature-branch->main, got %q", (*created)[0])
	}
	if len(*merged) != 0 {
		t.Errorf("rollup: manual should NOT merge the PR, got %v", *merged)
	}
	if !strings.Contains(output, "Rollup PR #999") {
		t.Errorf("expected rollup PR mention in output, got:\n%s", output)
	}
}

func TestRollup_AutoCreateAndMerges(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		})

	mockConfigDefaultBranch(t, "main")
	created, merged := setupRollupMocks(t)

	cfg := testConfig()
	cfg.BaseBranch = "feature-branch"
	cfg.AutoMerge.Rollup = "auto"

	output := captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 1 {
		t.Fatalf("rollup: auto should create exactly 1 PR, got %d", len(*created))
	}
	if len(*merged) != 1 || (*merged)[0] != 999 {
		t.Errorf("rollup: auto should merge PR #999, got %v", *merged)
	}
	if !strings.Contains(output, "Rollup PR #999 merged") {
		t.Errorf("expected 'Rollup PR #999 merged' in output, got:\n%s", output)
	}
}

func TestRollup_SkipWhenBaseBranchEmpty(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		})

	mockConfigDefaultBranch(t, "main")
	created, _ := setupRollupMocks(t)

	cfg := testConfig()
	cfg.BaseBranch = "" // empty — use default branch
	cfg.AutoMerge.Rollup = "auto"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 0 {
		t.Errorf("rollup should be skipped when BaseBranch is empty, got creates: %v", *created)
	}
}

func TestRollup_SkipWhenBaseBranchEqualsDefault(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		})

	mockConfigDefaultBranch(t, "main")
	created, _ := setupRollupMocks(t)

	cfg := testConfig()
	cfg.BaseBranch = "main" // same as default branch
	cfg.AutoMerge.Rollup = "auto"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 0 {
		t.Errorf("rollup should be skipped when BaseBranch equals default branch, got creates: %v", *created)
	}
}

func TestRollup_SkipWhenBaseBranchEqualsCustomDefault(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		})

	mockConfigDefaultBranch(t, "master")
	created, _ := setupRollupMocks(t)

	cfg := testConfig()
	cfg.DefaultBranch = "master"
	cfg.BaseBranch = "master" // same as custom default branch
	cfg.AutoMerge.Rollup = "auto"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 0 {
		t.Errorf("rollup should be skipped when BaseBranch equals custom default branch, got creates: %v", *created)
	}
}

func TestRollup_UsesCustomDefaultBranch(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 10}
		})

	mockConfigDefaultBranch(t, "master")
	created, _ := setupRollupMocks(t)

	cfg := testConfig()
	cfg.DefaultBranch = "master"
	cfg.BaseBranch = "feature-branch"
	cfg.AutoMerge.Rollup = "manual"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 1 {
		t.Fatalf("rollup should create 1 PR, got %d", len(*created))
	}
	if (*created)[0] != "feature-branch->master" {
		t.Errorf("expected rollup PR from feature-branch->master, got %q", (*created)[0])
	}
}

func TestRollup_SkipWhenZeroImplemented(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "feature"},
	}
	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed", Err: fmt.Errorf("test failure")}
		})

	mockConfigDefaultBranch(t, "main")
	created, _ := setupRollupMocks(t)

	cfg := testConfig()
	cfg.BaseBranch = "feature-branch"
	cfg.AutoMerge.Rollup = "auto"

	captureStdout(t, func() {
		if err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "m1", nil); err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(*created) != 0 {
		t.Errorf("rollup should be skipped when no issues implemented, got creates: %v", *created)
	}
}


func TestBuildRollupBody_ListsIssues(t *testing.T) {
	issues := []github.Issue{
		{Number: 1, Title: "Add feature A"},
		{Number: 2, Title: "Fix bug B"},
	}
	body := buildRollupBody(issues)
	if !strings.Contains(body, "#1 Add feature A") {
		t.Errorf("expected '#1 Add feature A' in body, got:\n%s", body)
	}
	if !strings.Contains(body, "#2 Fix bug B") {
		t.Errorf("expected '#2 Fix bug B' in body, got:\n%s", body)
	}
}

// TestReporter_IssueCompleted verifies that processIssues calls IssueCompleted
// with correct parameters after processIssueFn returns an outcome.
// PRNumber is 0 to avoid triggering the punchlist enrichment goroutine in tests.
func TestReporter_IssueCompleted(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 42, Title: "add cost tracking"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			// Use PRNumber:0 to avoid triggering the punchlist goroutine.
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed", Err: fmt.Errorf("test error")}
		})

	reporter := &fakeReporter{}
	closedSet := map[int]bool{}
	cfg := testConfig()

	if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), reporter, nil, false, "", "m1", nil); err != nil {
		t.Fatalf("processIssues() error = %v", err)
	}

	if len(reporter.issueCompleted) != 1 {
		t.Fatalf("expected 1 IssueCompleted call, got %d", len(reporter.issueCompleted))
	}
	got := reporter.issueCompleted[0]
	if got.issueNumber != 42 {
		t.Errorf("IssueCompleted issueNumber = %d, want 42", got.issueNumber)
	}
	if got.title != "add cost tracking" {
		t.Errorf("IssueCompleted title = %q, want %q", got.title, "add cost tracking")
	}
	if got.status != "failed" {
		t.Errorf("IssueCompleted status = %q, want %q", got.status, "failed")
	}
	if got.errMsg != "test error" {
		t.Errorf("IssueCompleted errMsg = %q, want %q", got.errMsg, "test error")
	}
}

// TestReporter_WaveStarted verifies that a second dependency wave triggers WaveStarted.
func TestReporter_WaveStarted(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "first"},
		{Number: 2, Title: "second", Body: "**Blocked by**: #1"},
	}

	callCount := 0
	setupProcessMocks(t, func() []int {
		// After issue #1 is merged, return it as closed so #2 unblocks.
		if callCount > 0 {
			return []int{1}
		}
		return nil
	}, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		callCount++
		// PRNumber:0 avoids triggering the punchlist enrichment goroutine in tests.
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 0}
	})

	reporter := &fakeReporter{}
	closedSet := map[int]bool{}
	cfg := testConfig()

	if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), reporter, nil, false, "", "m1", nil); err != nil {
		t.Fatalf("processIssues() error = %v", err)
	}

	// Wave 2 should have been signaled.
	if len(reporter.waveStarted) != 1 {
		t.Fatalf("expected 1 WaveStarted call, got %d: %v", len(reporter.waveStarted), reporter.waveStarted)
	}
	if reporter.waveStarted[0].wave != 2 {
		t.Errorf("WaveStarted wave = %d, want 2", reporter.waveStarted[0].wave)
	}
	if reporter.waveStarted[0].count != 1 {
		t.Errorf("WaveStarted count = %d, want 1", reporter.waveStarted[0].count)
	}
}

// TestReporter_RunFinished verifies that RunFinished receives correct summary counts.
func TestReporter_RunFinished(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "implemented"},
		{Number: 2, Title: "failed"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			if issue.Number == 1 {
				// PRNumber:0 avoids triggering the punchlist enrichment goroutine in tests.
				return agent.IssueOutcome{IssueNumber: 1, Status: "implemented", PRNumber: 0}
			}
			return agent.IssueOutcome{IssueNumber: 2, Status: "failed", Err: fmt.Errorf("boom")}
		})

	reporter := &fakeReporter{}
	closedSet := map[int]bool{}
	cfg := testConfig()

	if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), reporter, nil, false, "", "m1", nil); err != nil {
		t.Fatalf("processIssues() error = %v", err)
	}

	if len(reporter.runFinished) != 1 {
		t.Fatalf("expected 1 RunFinished call, got %d", len(reporter.runFinished))
	}
	rf := reporter.runFinished[0]
	if rf.implemented != 1 {
		t.Errorf("RunFinished implemented = %d, want 1", rf.implemented)
	}
	if rf.failed != 1 {
		t.Errorf("RunFinished failed = %d, want 1", rf.failed)
	}
}

// TestReporter_DryRunDoesNotCallReporter verifies dry-run mode does not call any
// reporter methods — it uses printDryRun directly.
func TestReporter_DryRunDoesNotCallReporter(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "test issue", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	reporter := &fakeReporter{}
	cfg := testConfig()

	if err := Run(context.Background(), cfg, testLogger(t), reporter, logging.NewLogger, "Phase 1", true, false, ""); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(reporter.issueCompleted) != 0 {
		t.Errorf("dry-run: expected no IssueCompleted calls, got %d", len(reporter.issueCompleted))
	}
	if len(reporter.runFinished) != 0 {
		t.Errorf("dry-run: expected no RunFinished calls, got %d", len(reporter.runFinished))
	}
	if len(reporter.waveStarted) != 0 {
		t.Errorf("dry-run: expected no WaveStarted calls, got %d", len(reporter.waveStarted))
	}
}

// TestReporter_LoggerPreserved verifies that logger.Info is still called for issue
// outcomes alongside reporter methods.
func TestReporter_LoggerPreserved(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 5, Title: "test issue"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			// PRNumber:0 avoids triggering the punchlist enrichment goroutine in tests.
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 0}
		})

	// Use a recording logger to verify logger.Info("issue outcome", ...) is still called.
	handler := &recordingHandler{}
	logger := slog.New(handler)

	reporter := &fakeReporter{}
	closedSet := map[int]bool{}
	cfg := testConfig()

	if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, logger, reporter, nil, false, "", "m1", nil); err != nil {
		t.Fatalf("processIssues() error = %v", err)
	}

	// Reporter received IssueCompleted.
	if len(reporter.issueCompleted) != 1 {
		t.Fatalf("expected 1 IssueCompleted, got %d", len(reporter.issueCompleted))
	}

	// Logger also received the "issue outcome" log entry.
	var found bool
	for _, r := range handler.records {
		if r.Message == "issue outcome" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected logger.Info('issue outcome', ...) to be called")
	}
}

// TestReporter_AllBlockedCalled verifies that AllBlocked is called when all issues
// are blocked.
func TestReporter_AllBlockedCalled(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 1, Title: "blocked", Body: "**Blocked by**: #99"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed"}
		})

	reporter := &fakeReporter{}
	closedSet := map[int]bool{} // #99 not closed
	cfg := testConfig()

	if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), reporter, nil, false, "", "m1", nil); err != nil {
		t.Fatalf("processIssues() error = %v", err)
	}

	if len(reporter.allBlocked) != 1 {
		t.Fatalf("expected 1 AllBlocked call, got %d", len(reporter.allBlocked))
	}
	if reporter.allBlocked[0].total != 1 {
		t.Errorf("AllBlocked total = %d, want 1", reporter.allBlocked[0].total)
	}
	if reporter.allBlocked[0].blocked != 1 {
		t.Errorf("AllBlocked blocked = %d, want 1", reporter.allBlocked[0].blocked)
	}
}

// setupRollupVerifyMock replaces runRollupVerifyFn with fn and restores it
// at the end of the test.
func setupRollupVerifyMock(t *testing.T, fn func(ctx context.Context, prNum int, branch string, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, writeResult func(rundata.VerifyStepResult) error) (bool, error)) {
	t.Helper()
	orig := runRollupVerifyFn
	t.Cleanup(func() { runRollupVerifyFn = orig })
	runRollupVerifyFn = fn
}

// setupHandleRollupPRMocks installs the upsert and merge mocks used by
// tests that call handleRollupPR directly (bypassing processIssues).
func setupHandleRollupPRMocks(t *testing.T) (merged *[]int) {
	t.Helper()
	var mergedCalls []int

	origUpsert := upsertRollupPRFn
	t.Cleanup(func() { upsertRollupPRFn = origUpsert })
	upsertRollupPRFn = func(_, _, _, _, _ string) (int, string, error) {
		return 999, "https://github.com/owner/repo/pull/999", nil
	}

	origMerge := mergeRollupPRFn
	t.Cleanup(func() { mergeRollupPRFn = origMerge })
	mergeRollupPRFn = func(_ string, prNum int) error {
		mergedCalls = append(mergedCalls, prNum)
		return nil
	}

	return &mergedCalls
}

func TestRollupVerify_PassesOnFirstAttempt(t *testing.T) {
	merged := setupHandleRollupPRMocks(t)

	var verifyCalls int
	setupRollupVerifyMock(t, func(_ context.Context, _ int, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ func(rundata.VerifyStepResult) error) (bool, error) {
		verifyCalls++
		return true, nil
	})

	cfg := testConfig()
	cfg.AutoMerge.Rollup = "auto"
	reporter := &fakeReporter{}

	prNum, prURL, err := handleRollupPR(context.Background(), cfg, nil, "main", testLogger(t), reporter, nil, &agent.Prompts{}, nil)
	if err != nil {
		t.Fatalf("handleRollupPR() error = %v", err)
	}
	if prNum != 999 {
		t.Errorf("pr number = %d, want 999", prNum)
	}
	if prURL == "" {
		t.Error("pr url should not be empty")
	}
	if verifyCalls != 1 {
		t.Errorf("expected 1 verify call, got %d", verifyCalls)
	}
	if len(*merged) != 1 || (*merged)[0] != 999 {
		t.Errorf("rollup: verify pass should proceed to merge, got merges: %v", *merged)
	}
	if len(reporter.rollupCreated) != 1 || !reporter.rollupCreated[0].merged {
		t.Errorf("expected RollupCreated with merged=true, got: %v", reporter.rollupCreated)
	}
}

func TestRollupVerify_ExhaustsRetries_LeavesPROpen(t *testing.T) {
	merged := setupHandleRollupPRMocks(t)

	setupRollupVerifyMock(t, func(_ context.Context, _ int, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ func(rundata.VerifyStepResult) error) (bool, error) {
		return false, nil // exhausted retries — verify never passes
	})

	cfg := testConfig()
	cfg.AutoMerge.Rollup = "auto"
	reporter := &fakeReporter{}

	prNum, _, err := handleRollupPR(context.Background(), cfg, nil, "main", testLogger(t), reporter, nil, &agent.Prompts{}, nil)
	if err != nil {
		t.Fatalf("handleRollupPR() error = %v", err)
	}
	if prNum != 999 {
		t.Errorf("pr number = %d, want 999", prNum)
	}
	if len(*merged) != 0 {
		t.Errorf("rollup: exhausted verify retries must not merge, got merges: %v", *merged)
	}
	// PR should be reported as created but not merged (merged=false).
	if len(reporter.rollupCreated) != 1 {
		t.Fatalf("expected 1 RollupCreated call, got %d", len(reporter.rollupCreated))
	}
	if reporter.rollupCreated[0].merged {
		t.Errorf("rollup: PR should be left open when verify fails, but merged=true")
	}
}

func TestRollupVerify_WriteResultCalledWhenWriterPresent(t *testing.T) {
	setupHandleRollupPRMocks(t)

	var capturedWriteResult func(rundata.VerifyStepResult) error
	setupRollupVerifyMock(t, func(_ context.Context, _ int, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, writeResult func(rundata.VerifyStepResult) error) (bool, error) {
		capturedWriteResult = writeResult
		if writeResult != nil {
			_ = writeResult(rundata.VerifyStepResult{Attempt: 0, AllPassed: true})
		}
		return true, nil
	})

	cfg := testConfig()
	cfg.AutoMerge.Rollup = "auto"
	reporter := &fakeReporter{}

	tmpDir := t.TempDir()
	writer, err := rundata.NewWithBase(tmpDir, "owner/repo", "m1", []int{1}, "", rundata.AutoMerge{})
	if err != nil {
		t.Fatalf("NewWithBase: %v", err)
	}

	if _, _, err := handleRollupPR(context.Background(), cfg, nil, "main", testLogger(t), reporter, writer, &agent.Prompts{}, nil); err != nil {
		t.Fatalf("handleRollupPR() error = %v", err)
	}

	if capturedWriteResult == nil {
		t.Error("expected writeResult callback to be non-nil when writer is present")
	}

	// Verify the file was actually written to the rollup directory.
	runDir := writer.Dir()
	verifyFile := filepath.Join(runDir, "rollup", "verify-0.json")
	if _, err := os.Stat(verifyFile); err != nil {
		t.Errorf("expected rollup verify file at %s: %v", verifyFile, err)
	}
}

func TestRollupVerify_ManualMode_VerifyFailsLeavesOpen(t *testing.T) {
	merged := setupHandleRollupPRMocks(t)

	setupRollupVerifyMock(t, func(_ context.Context, _ int, _ string, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ func(rundata.VerifyStepResult) error) (bool, error) {
		return false, nil
	})

	cfg := testConfig()
	cfg.AutoMerge.Rollup = "manual"
	reporter := &fakeReporter{}

	if _, _, err := handleRollupPR(context.Background(), cfg, nil, "main", testLogger(t), reporter, nil, &agent.Prompts{}, nil); err != nil {
		t.Fatalf("handleRollupPR() error = %v", err)
	}

	if len(*merged) != 0 {
		t.Errorf("manual rollup: should never merge, got merges: %v", *merged)
	}
	if len(reporter.rollupCreated) != 1 || reporter.rollupCreated[0].merged {
		t.Errorf("manual rollup: PR should be left open, got rollupCreated: %v", reporter.rollupCreated)
	}
}

// TestReporter_IssueCompleted_WithCost verifies that processIssues reads per-issue
// cost from step result files written to the run data writer directory and passes
// the cost to reporter.IssueCompleted.
func TestReporter_IssueCompleted_WithCost(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 42, Title: "add cost tracking"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed", Err: fmt.Errorf("test error")}
		})

	// Create a writer with a known step cost so IssueCostUSD can find it.
	tmpDir := t.TempDir()
	writer, err := rundata.NewWithBase(tmpDir, "owner/repo", "m1", []int{42}, "", rundata.AutoMerge{})
	if err != nil {
		t.Fatalf("NewWithBase() error: %v", err)
	}
	if err := writer.WriteImplementResult(42, rundata.StepResult{CostUSD: 1.50}); err != nil {
		t.Fatalf("WriteImplementResult() error: %v", err)
	}

	reporter := &fakeReporter{}
	closedSet := map[int]bool{}
	cfg := testConfig()

	if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), reporter, writer, false, "", "m1", nil); err != nil {
		t.Fatalf("processIssues() error = %v", err)
	}

	if len(reporter.issueCompleted) != 1 {
		t.Fatalf("expected 1 IssueCompleted call, got %d", len(reporter.issueCompleted))
	}
	got := reporter.issueCompleted[0]
	if got.costUSD != 1.50 {
		t.Errorf("IssueCompleted costUSD = %f, want 1.50", got.costUSD)
	}
}

// TestReporter_IssueCompleted_ZeroCostWhenNoWriter verifies that cost is 0.0
// when the writer is nil (no run data directory).
func TestReporter_IssueCompleted_ZeroCostWhenNoWriter(t *testing.T) {
	allIssues := []github.Issue{
		{Number: 99, Title: "no cost"},
	}

	setupProcessMocks(t, func() []int { return nil },
		func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
			return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed"}
		})

	reporter := &fakeReporter{}
	closedSet := map[int]bool{}
	cfg := testConfig()

	// Pass nil writer — cost must degrade gracefully to 0.0.
	if err := processIssues(context.Background(), allIssues, closedSet, nil, cfg, testLogger(t), reporter, nil, false, "", "m1", nil); err != nil {
		t.Fatalf("processIssues() error = %v", err)
	}

	if len(reporter.issueCompleted) != 1 {
		t.Fatalf("expected 1 IssueCompleted call, got %d", len(reporter.issueCompleted))
	}
	if got := reporter.issueCompleted[0].costUSD; got != 0.0 {
		t.Errorf("IssueCompleted costUSD = %f, want 0.0", got)
	}
}

func TestFilterNoDarkIssues_RemovesNoDarkLabeled(t *testing.T) {
	logger := slog.Default()
	issues := []github.Issue{
		{Number: 1, Title: "normal issue", Labels: []string{}},
		{Number: 2, Title: "nodark issue", Labels: []string{"nodark"}},
		{Number: 3, Title: "another normal", Labels: []string{"p1"}},
		{Number: 4, Title: "nodark with other labels", Labels: []string{"p2", "nodark"}},
	}

	filtered, _ := filterNoDarkIssues(issues, logger)

	if len(filtered) != 2 {
		t.Fatalf("filterNoDarkIssues() returned %d issues, want 2", len(filtered))
	}
	if filtered[0].Number != 1 {
		t.Errorf("filtered[0].Number = %d, want 1", filtered[0].Number)
	}
	if filtered[1].Number != 3 {
		t.Errorf("filtered[1].Number = %d, want 3", filtered[1].Number)
	}
}

func TestFilterNoDarkIssues_NoNoDarkLabels(t *testing.T) {
	logger := slog.Default()
	issues := []github.Issue{
		{Number: 1, Title: "normal issue", Labels: []string{"p1"}},
		{Number: 2, Title: "another normal", Labels: []string{}},
	}

	filtered, _ := filterNoDarkIssues(issues, logger)

	if len(filtered) != 2 {
		t.Errorf("filterNoDarkIssues() returned %d issues, want 2", len(filtered))
	}
}

func TestFilterNoDarkIssues_AllNoDark(t *testing.T) {
	logger := slog.Default()
	issues := []github.Issue{
		{Number: 1, Title: "nodark 1", Labels: []string{"nodark"}},
		{Number: 2, Title: "nodark 2", Labels: []string{"nodark"}},
	}

	filtered, _ := filterNoDarkIssues(issues, logger)

	if len(filtered) != 0 {
		t.Errorf("filterNoDarkIssues() returned %d issues, want 0", len(filtered))
	}
}

func TestDryRun_NoDarkIssuesExcluded(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "normal issue", Labels: []ghLabel{}},
		{Number: 2, Title: "nodark issue", Labels: []ghLabel{{Name: "nodark"}}},
		{Number: 3, Title: "another normal", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	// The nodark issue should not appear in output.
	if strings.Contains(output, "#2 nodark issue") {
		t.Errorf("nodark issue #2 should not appear in output, got:\n%s", output)
	}
	// Normal issues should still appear.
	if !strings.Contains(output, "#1 normal issue") {
		t.Errorf("expected normal issue #1 in output, got:\n%s", output)
	}
	if !strings.Contains(output, "#3 another normal") {
		t.Errorf("expected normal issue #3 in output, got:\n%s", output)
	}
	// Summary should show only 2 total (nodark excluded).
	if !strings.Contains(output, "Summary: 2 total") {
		t.Errorf("expected summary with 2 total (nodark excluded), got:\n%s", output)
	}
}

func TestDryRun_NoDarkIssueDoesNotBlockOthers(t *testing.T) {
	// Issue 2 (nodark) is declared as a dependency of issue 3.
	// Since nodark issues are excluded before dep resolution, issue 3 should
	// NOT be treated as blocked by issue 2.
	openIssues := []ghIssue{
		{Number: 1, Title: "normal issue", Labels: []ghLabel{}},
		{Number: 2, Title: "nodark issue", Labels: []ghLabel{{Name: "nodark"}}},
		{Number: 3, Title: "depends on nodark", Body: "**Blocked by**: #2", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), progress.NewTextReporter(os.Stdout), logging.NewLogger, "Phase 1", true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	// Issue 3 should appear as processable (not blocked), because #2 is excluded.
	if strings.Contains(output, "Blocked issues:") {
		t.Errorf("expected no blocked issues (nodark should not block others), got:\n%s", output)
	}
	if !strings.Contains(output, "#3 depends on nodark") {
		t.Errorf("expected issue #3 in processable output, got:\n%s", output)
	}
}

// setupReResolveAndProcessMocks is like setupProcessMocks and preps the lock seam.
// It returns closedNumbers as a mutable slice pointer for tests to update.
func setupReResolveAndProcessMocks(t *testing.T, closedNumbers *[]int, processFn func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger, hook agent.RunDataHook, reporter progress.ProgressReporter) agent.IssueOutcome) {
	t.Helper()
	setupProcessMocks(t, func() []int { return *closedNumbers }, processFn)
}

func TestReResolveAndProcess_UnblocksAfterMerge(t *testing.T) {
	// Issue 1 (no deps) was already processed (in seen).
	// Issue 2 (blocked by #1) should become processable after #1 is closed.
	allIssues := []github.Issue{
		{Number: 1, Title: "first task"},
		{Number: 2, Title: "second task", Body: "**Blocked by**: #1"},
	}

	var processed []int
	closedNums := []int{1} // #1 already merged externally
	setupReResolveAndProcessMocks(t, &closedNums, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		processed = append(processed, issue.Number)
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 200 + issue.Number}
	})

	cfg := testConfig()

	seen := map[int]bool{1: true} // issue 1 was processed in the prior run

	captureStdout(t, func() {
		got, err := ReResolveAndProcess(context.Background(), allIssues, nil, seen, cfg, "test-milestone", testLogger(t), progress.NewTextReporter(os.Stdout), nil)
		if err != nil {
			t.Fatalf("ReResolveAndProcess() error = %v", err)
		}
		if !got {
			t.Error("expected ReResolveAndProcess to return true (issues were processed)")
		}
	})

	if len(processed) != 1 || processed[0] != 2 {
		t.Errorf("expected issue #2 to be processed, got %v", processed)
	}
	if !seen[2] {
		t.Error("expected seen to be updated with issue #2")
	}
}

func TestReResolveAndProcess_MultipleUnblocked(t *testing.T) {
	// Issue 3 blocked by #1, issue 4 blocked by #2. Both #1 and #2 closed.
	allIssues := []github.Issue{
		{Number: 1, Title: "first"},
		{Number: 2, Title: "second"},
		{Number: 3, Title: "third", Body: "**Blocked by**: #1"},
		{Number: 4, Title: "fourth", Body: "**Blocked by**: #2"},
	}

	var processed []int
	closedNums := []int{1, 2}
	setupReResolveAndProcessMocks(t, &closedNums, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		processed = append(processed, issue.Number)
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "ready-to-merge", PRNumber: 300 + issue.Number}
	})

	cfg := testConfig()

	seen := map[int]bool{1: true, 2: true}

	captureStdout(t, func() {
		got, err := ReResolveAndProcess(context.Background(), allIssues, nil, seen, cfg, "test-milestone", testLogger(t), progress.NewTextReporter(os.Stdout), nil)
		if err != nil {
			t.Fatalf("ReResolveAndProcess() error = %v", err)
		}
		if !got {
			t.Error("expected true when issues are processed")
		}
	})

	if len(processed) != 2 {
		t.Fatalf("expected 2 issues processed, got %d: %v", len(processed), processed)
	}
	for _, n := range []int{3, 4} {
		found := false
		for _, p := range processed {
			if p == n {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected issue #%d to be processed, processed=%v", n, processed)
		}
	}
}

func TestReResolveAndProcess_NoNewUnblocked(t *testing.T) {
	// Issue 2 blocked by #99 which is NOT closed yet. No processing expected.
	allIssues := []github.Issue{
		{Number: 2, Title: "blocked task", Body: "**Blocked by**: #99"},
	}

	var processed []int
	closedNums := []int{} // #99 still open
	setupReResolveAndProcessMocks(t, &closedNums, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		processed = append(processed, issue.Number)
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented"}
	})

	cfg := testConfig()

	seen := map[int]bool{}

	captureStdout(t, func() {
		got, err := ReResolveAndProcess(context.Background(), allIssues, nil, seen, cfg, "test-milestone", testLogger(t), progress.NewTextReporter(os.Stdout), nil)
		if err != nil {
			t.Fatalf("ReResolveAndProcess() error = %v", err)
		}
		if got {
			t.Error("expected false when no issues are unblocked")
		}
	})

	if len(processed) != 0 {
		t.Errorf("expected no issues processed, got %v", processed)
	}
}

func TestReResolveAndProcess_SeenSkipsAlreadyProcessed(t *testing.T) {
	// Both issues are unblocked, but issue 1 is already in seen — only issue 2 should run.
	allIssues := []github.Issue{
		{Number: 1, Title: "already done"},
		{Number: 2, Title: "fresh issue"},
	}

	var processed []int
	closedNums := []int{}
	setupReResolveAndProcessMocks(t, &closedNums, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		processed = append(processed, issue.Number)
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "ready-to-merge"}
	})

	cfg := testConfig()

	seen := map[int]bool{1: true} // issue 1 already processed

	captureStdout(t, func() {
		got, err := ReResolveAndProcess(context.Background(), allIssues, nil, seen, cfg, "test-milestone", testLogger(t), progress.NewTextReporter(os.Stdout), nil)
		if err != nil {
			t.Fatalf("ReResolveAndProcess() error = %v", err)
		}
		if !got {
			t.Error("expected true (issue 2 was processed)")
		}
	})

	if len(processed) != 1 || processed[0] != 2 {
		t.Errorf("expected only issue #2 to be processed, got %v", processed)
	}
}

// TestProcessIssues_ComposeStartsSuccessfully verifies that when
// docker_compose is configured, docker compose up -d is called with the
// correct -f and -p flags before any agent execution.
func TestProcessIssues_ComposeStartsSuccessfully(t *testing.T) {
	allIssues := []github.Issue{{Number: 1, Title: "task"}}

	var composeCalls [][]string
	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 101}
	})

	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		composeCalls = append(composeCalls, append([]string{name}, args...))
		return []byte("ok"), nil
	}

	cfg := testConfig()
	cfg.DockerCompose = &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myapp",
	}

	captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
		if err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(composeCalls) == 0 {
		t.Fatal("expected docker compose up to be called, got no calls")
	}

	var found bool
	for _, call := range composeCalls {
		if len(call) >= 2 && call[0] == "docker" && call[1] == "compose" {
			found = true
			args := call[2:]
			wantArgs := []string{"-f", "docker-compose.test.yml", "-p", "myapp", "up", "-d"}
			if len(args) != len(wantArgs) {
				t.Errorf("docker compose args = %v, want %v", args, wantArgs)
				break
			}
			for i, a := range args {
				if a != wantArgs[i] {
					t.Errorf("docker compose arg[%d] = %q, want %q", i, a, wantArgs[i])
				}
			}
			break
		}
	}
	if !found {
		t.Errorf("no docker compose call found in calls: %v", composeCalls)
	}
}

// TestProcessIssues_ComposeStartupFailure verifies that when docker compose up
// fails, the run is aborted and no agent execution occurs.
func TestProcessIssues_ComposeStartupFailure(t *testing.T) {
	allIssues := []github.Issue{{Number: 1, Title: "task"}}

	var agentCalled bool
	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		agentCalled = true
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 101}
	})

	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		if name == "docker" && len(args) > 0 && args[0] == "compose" {
			return []byte("compose startup error"), fmt.Errorf("exit status 1")
		}
		return []byte("ok"), nil
	}

	cfg := testConfig()
	cfg.DockerCompose = &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myapp",
	}

	captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
		if err == nil {
			t.Fatal("expected processIssues to return an error when compose startup fails")
		}
		if !strings.Contains(err.Error(), "starting compose services") {
			t.Errorf("error = %v, want it to mention starting compose services", err)
		}
	})

	if agentCalled {
		t.Error("expected no agent execution when compose startup fails")
	}
}

// TestProcessIssues_ComposeSkippedWhenNotConfigured verifies that when
// docker_compose is not set, no docker compose commands are executed and
// agent execution proceeds normally.
func TestProcessIssues_ComposeSkippedWhenNotConfigured(t *testing.T) {
	allIssues := []github.Issue{{Number: 1, Title: "task"}}

	var agentCalled bool
	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		agentCalled = true
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 101}
	})

	var composeCalled bool
	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		if name == "docker" && len(args) > 0 && args[0] == "compose" {
			composeCalled = true
		}
		return []byte("ok"), nil
	}

	cfg := testConfig()
	// cfg.DockerCompose is nil — compose not configured.

	captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
		if err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if composeCalled {
		t.Error("expected no docker compose call when docker_compose is not configured")
	}
	if !agentCalled {
		t.Error("expected agent to be called when compose is not configured")
	}
}

// TestProcessIssues_ComposeProjectNamePassed verifies that the -p flag with
// the configured project name is included in the docker compose command.
func TestProcessIssues_ComposeProjectNamePassed(t *testing.T) {
	allIssues := []github.Issue{{Number: 42, Title: "task"}}

	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 200}
	})

	var capturedArgs []string
	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		if name == "docker" && len(args) > 0 && args[0] == "compose" {
			capturedArgs = append([]string{name}, args...)
		}
		return []byte("ok"), nil
	}

	cfg := testConfig()
	cfg.DockerCompose = &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myapp",
	}

	captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
		if err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if len(capturedArgs) == 0 {
		t.Fatal("expected docker compose to be called, got no calls")
	}

	var foundP bool
	for i, arg := range capturedArgs {
		if arg == "-p" && i+1 < len(capturedArgs) {
			if capturedArgs[i+1] != "myapp" {
				t.Errorf("-p value = %q, want %q", capturedArgs[i+1], "myapp")
			}
			foundP = true
			break
		}
	}
	if !foundP {
		t.Errorf("expected -p flag in compose args, got: %v", capturedArgs)
	}
}

// hasComposeDown returns true if any recorded call is `docker compose ... down
// --volumes`.
func hasComposeDown(calls [][]string) bool {
	for _, call := range calls {
		if len(call) < 2 || call[0] != "docker" || call[1] != "compose" {
			continue
		}
		var foundDown, foundVolumes bool
		for _, a := range call[2:] {
			if a == "down" {
				foundDown = true
			}
			if a == "--volumes" {
				foundVolumes = true
			}
		}
		if foundDown && foundVolumes {
			return true
		}
	}
	return false
}

// TestProcessIssues_ComposeDownCalledAfterNormalRun verifies that
// docker compose down --volumes is called after a successful run.
func TestProcessIssues_ComposeDownCalledAfterNormalRun(t *testing.T) {
	allIssues := []github.Issue{{Number: 1, Title: "task"}}

	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 101}
	})

	var composeCalls [][]string
	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		composeCalls = append(composeCalls, append([]string{name}, args...))
		return []byte("ok"), nil
	}

	cfg := testConfig()
	cfg.DockerCompose = &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myapp",
	}

	captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
		if err != nil {
			t.Fatalf("processIssues() error = %v", err)
		}
	})

	if !hasComposeDown(composeCalls) {
		t.Errorf("expected docker compose down --volumes to be called, got calls: %v", composeCalls)
	}
}

// TestProcessIssues_ComposeDownCalledAfterRunFailure verifies that
// docker compose down --volumes is called even when the run encounters an
// agent failure (deferred cleanup runs regardless of exit path).
func TestProcessIssues_ComposeDownCalledAfterRunFailure(t *testing.T) {
	allIssues := []github.Issue{{Number: 1, Title: "task"}}

	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "failed"}
	})

	var composeCalls [][]string
	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		composeCalls = append(composeCalls, append([]string{name}, args...))
		return []byte("ok"), nil
	}

	cfg := testConfig()
	cfg.DockerCompose = &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myapp",
	}

	captureStdout(t, func() {
		// The run may return nil even on agent failures (failing is a valid
		// outcome, not a processIssues error).
		_ = processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
	})

	if !hasComposeDown(composeCalls) {
		t.Errorf("expected docker compose down --volumes to be called after agent failure, got calls: %v", composeCalls)
	}
}

// TestProcessIssues_ComposeDownCalledAfterContextCancellation verifies that
// docker compose down --volumes is called when the context is cancelled before
// the run processes any issues.
func TestProcessIssues_ComposeDownCalledAfterContextCancellation(t *testing.T) {
	allIssues := []github.Issue{{Number: 1, Title: "task"}}

	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 101}
	})

	var composeCalls [][]string
	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		composeCalls = append(composeCalls, append([]string{name}, args...))
		return []byte("ok"), nil
	}

	cfg := testConfig()
	cfg.DockerCompose = &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myapp",
	}

	// Use a pre-cancelled context. ComposeUp ignores the context, so it will
	// succeed. The inner issue loop will detect the cancellation and exit via
	// goto done, after which the deferred ComposeDown fires.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	captureStdout(t, func() {
		_ = processIssues(ctx, allIssues, map[int]bool{}, nil, cfg, testLogger(t), progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
	})

	if !hasComposeDown(composeCalls) {
		t.Errorf("expected docker compose down --volumes to be called after context cancellation, got calls: %v", composeCalls)
	}
}

// TestProcessIssues_ComposeDownFailureLogsWarning verifies that when
// docker compose down fails, the error is logged as a warning and the run
// exit status is not changed (processIssues still returns nil).
func TestProcessIssues_ComposeDownFailureLogsWarning(t *testing.T) {
	allIssues := []github.Issue{{Number: 1, Title: "task"}}

	setupProcessMocks(t, func() []int { return nil }, func(_ context.Context, issue github.Issue, _ *config.Config, _ *agent.Prompts, _ map[string]string, _ *slog.Logger, _ agent.RunDataHook, _ progress.ProgressReporter) agent.IssueOutcome {
		return agent.IssueOutcome{IssueNumber: issue.Number, Status: "implemented", PRNumber: 101}
	})

	origSandboxRunner := sandbox.CommandRunner
	t.Cleanup(func() { sandbox.CommandRunner = origSandboxRunner })
	sandbox.CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "compose" {
			for _, a := range args {
				if a == "down" {
					return []byte("down failed"), fmt.Errorf("exit status 1")
				}
			}
		}
		return []byte("ok"), nil
	}

	cfg := testConfig()
	cfg.DockerCompose = &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myapp",
	}

	handler := &recordingHandler{}
	logger := slog.New(handler)

	var runErr error
	captureStdout(t, func() {
		runErr = processIssues(context.Background(), allIssues, map[int]bool{}, nil, cfg, logger, progress.NewTextReporter(os.Stdout), nil, false, "", "test-milestone", nil)
	})

	// The run must still succeed despite the compose down failure.
	if runErr != nil {
		t.Errorf("expected processIssues to return nil when compose down fails, got: %v", runErr)
	}

	// A warning must have been logged.
	var foundWarn bool
	for _, r := range handler.records {
		if r.Level == slog.LevelWarn {
			msg := r.Message
			if strings.Contains(msg, "compose") || strings.Contains(msg, "down") {
				foundWarn = true
				break
			}
		}
	}
	if !foundWarn {
		t.Error("expected a warning log record about compose down failure")
	}
}

func TestRefreshAndCategorize_ReturnsCorrectSplit(t *testing.T) {
	// Issue 3 is blocked by #1. After #1 closes, #3 becomes processable.
	allIssues := []github.Issue{
		{Number: 2, Title: "free issue"},
		{Number: 3, Title: "blocked by 1", Body: "**Blocked by**: #1"},
	}

	origRunner := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origRunner })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		// gh issue list --state closed → return #1 as closed
		if len(args) > 0 && args[0] == "issue" {
			for i, a := range args {
				if a == "--state" && i+1 < len(args) && args[i+1] == "closed" {
					return json.Marshal([]struct{ Number int }{{Number: 1}})
				}
			}
		}
		// gh pr list --state merged → no merged PRs
		return []byte("[]"), nil
	}

	processable, blocked, err := refreshAndCategorize("owner/repo", allIssues, nil, nil, slog.Default())
	if err != nil {
		t.Fatalf("refreshAndCategorize() error = %v", err)
	}
	if len(processable) != 2 {
		t.Fatalf("expected 2 processable (2 and 3), got %d: %v", len(processable), processable)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked, got %d: %v", len(blocked), blocked)
	}
}

func TestRefreshAndCategorize_ForcedClosedUnblocksDependent(t *testing.T) {
	// Issue 5 is blocked by #4. Issue 4 was just merged but CloseIssue failed
	// (issue still open on GitHub). forcedClosed=[4] must unblock issue 5.
	allIssues := []github.Issue{
		{Number: 5, Title: "blocked by 4", Body: "**Blocked by**: #4"},
	}

	origRunner := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origRunner })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		// GitHub reports no closed issues (CloseIssue failed).
		if len(args) > 0 && args[0] == "issue" {
			return []byte("[]"), nil
		}
		// No merged PRs with closing keywords.
		return []byte("[]"), nil
	}

	processable, blocked, err := refreshAndCategorize("owner/repo", allIssues, nil, []int{4}, slog.Default())
	if err != nil {
		t.Fatalf("refreshAndCategorize() error = %v", err)
	}
	if len(processable) != 1 || processable[0].Number != 5 {
		t.Fatalf("expected issue 5 to be processable, got processable=%v blocked=%v", processable, blocked)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked, got %d", len(blocked))
	}
}

func TestRefreshAndCategorize_MergedPRUnblocksDependent(t *testing.T) {
	// Issue 7 is blocked by #6. Issue 6's PR was merged with "Closes #6" in
	// the body, but CloseIssue was never called (watch-mode path).
	allIssues := []github.Issue{
		{Number: 7, Title: "blocked by 6", Body: "**Blocked by**: #6"},
	}

	origRunner := github.CommandRunner
	t.Cleanup(func() { github.CommandRunner = origRunner })
	github.CommandRunner = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "issue" {
			// GitHub still shows issue 6 as open.
			return []byte("[]"), nil
		}
		// gh pr list --state merged returns a PR whose body closes #6.
		if len(args) > 0 && args[0] == "pr" {
			return []byte(`[{"body":"Closes #6\n\nImplementation notes here."}]`), nil
		}
		return []byte("[]"), nil
	}

	processable, blocked, err := refreshAndCategorize("owner/repo", allIssues, nil, nil, slog.Default())
	if err != nil {
		t.Fatalf("refreshAndCategorize() error = %v", err)
	}
	if len(processable) != 1 || processable[0].Number != 7 {
		t.Fatalf("expected issue 7 to be processable, got processable=%v blocked=%v", processable, blocked)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked, got %d", len(blocked))
	}
}
