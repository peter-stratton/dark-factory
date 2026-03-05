package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/logging"
)

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
		Repo:   "owner/repo",
		LogDir: "",
	}
}

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
		err := Run(context.Background(), testConfig(), testLogger(t), "Phase 1", 0, true, false, "")
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
		err := Run(context.Background(), testConfig(), testLogger(t), "Phase 1", 0, true, false, "")
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
		err := Run(context.Background(), testConfig(), testLogger(t), "Phase 1", 0, true, false, "")
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
		if err := Run(context.Background(), testConfig(), logger, "Phase 1", 0, true, false, ""); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("expected log file to be created")
	}
	if !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Errorf("expected JSON log file, got %s", entries[0].Name())
	}
}

func TestEmptyMilestone(t *testing.T) {
	setupFakeGH(t, []ghIssue{}, nil)

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), "Phase 1", 0, true, false, "")
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

	output := captureStdout(t, func() {
		err := Run(context.Background(), testConfig(), testLogger(t), "Phase 1", 0, false, false, "")
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
		err := Run(context.Background(), testConfig(), testLogger(t), "Phase 1", 0, true, false, "")
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
		err := Run(context.Background(), testConfig(), testLogger(t), "Phase 1", 0, true, false, "")
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

func TestSingleIssueMode_FiltersCorrectly(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "first", Labels: []ghLabel{}},
		{Number: 2, Title: "second", Labels: []ghLabel{}},
		{Number: 3, Title: "third", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	cfg := testConfig()

	output := captureStdout(t, func() {
		err := Run(context.Background(), cfg, testLogger(t), "Phase 1", 2, true, false, "")
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if !strings.Contains(output, "#2 second") {
		t.Errorf("expected issue #2 in output, got:\n%s", output)
	}
	// Should not contain the other issues in processable section
	if strings.Contains(output, "#1 first") {
		t.Errorf("expected issue #1 filtered out, got:\n%s", output)
	}
}

func TestSingleIssueMode_ErrorWhenBlocked(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "blocked", Body: "**Blocked by**: #99", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	cfg := testConfig()

	captureStdout(t, func() {
		err := Run(context.Background(), cfg, testLogger(t), "Phase 1", 1, true, false, "")
		if err == nil {
			t.Fatal("expected error for blocked single issue")
		}
		if !strings.Contains(err.Error(), "blocked by") {
			t.Errorf("expected 'blocked by' in error, got: %v", err)
		}
	})
}

func TestSingleIssueMode_ErrorWhenNotFound(t *testing.T) {
	openIssues := []ghIssue{
		{Number: 1, Title: "first", Labels: []ghLabel{}},
	}
	setupFakeGH(t, openIssues, nil)

	cfg := testConfig()

	captureStdout(t, func() {
		err := Run(context.Background(), cfg, testLogger(t), "Phase 1", 999, true, false, "")
		if err == nil {
			t.Fatal("expected error for missing single issue")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' in error, got: %v", err)
		}
	})
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
func setupProcessMocks(t *testing.T, closedNumbersFn func() []int, processFn func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) agent.IssueOutcome) {
	t.Helper()

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
	}, func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) agent.IssueOutcome {
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
	cfg.NoSandbox = true

	output := captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, closedSet, cfg, testLogger(t), false, "")
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
	}, func(ctx context.Context, issue github.Issue, cfg *config.Config, prompts *agent.Prompts, authEnv map[string]string, logger *slog.Logger) agent.IssueOutcome {
		processedNumbers = append(processedNumbers, issue.Number)
		return agent.IssueOutcome{
			IssueNumber: issue.Number,
			Status:      "failed",
			Err:         fmt.Errorf("test failure"),
		}
	})

	closedSet := map[int]bool{}
	cfg := testConfig()
	cfg.NoSandbox = true

	output := captureStdout(t, func() {
		err := processIssues(context.Background(), allIssues, closedSet, cfg, testLogger(t), false, "")
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

// Suppress unused import warnings.
var _ = fmt.Sprintf
