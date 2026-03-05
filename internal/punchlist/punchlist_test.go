package punchlist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_EmptyEntries(t *testing.T) {
	result := Generate(nil)
	if result != "" {
		t.Errorf("expected empty string for no entries, got %q", result)
	}
}

func TestGenerate_EmptySlice(t *testing.T) {
	result := Generate([]Entry{})
	if result != "" {
		t.Errorf("expected empty string for empty slice, got %q", result)
	}
}

func TestGenerate_SingleEntry(t *testing.T) {
	entries := []Entry{
		{
			IssueNumber:  42,
			IssueTitle:   "Add widget",
			IssueBody:    "Some description.",
			PRNumber:     99,
			Repo:         "owner/repo",
			ChangedFiles: []string{"internal/widget.go", "web/widget.html"},
		},
	}

	result := Generate(entries)

	if !strings.Contains(result, "=== Manual Testing Punchlist ===") {
		t.Errorf("missing header in output: %q", result)
	}
	if !strings.Contains(result, "1. Issue #42: Add widget (PR #99)") {
		t.Errorf("missing issue/PR header in output: %q", result)
	}
	if !strings.Contains(result, "https://github.com/owner/repo/pull/99") {
		t.Errorf("missing PR URL in output: %q", result)
	}
	if !strings.Contains(result, "internal/widget.go") {
		t.Errorf("missing changed file in output: %q", result)
	}
	if !strings.Contains(result, "web/widget.html") {
		t.Errorf("missing changed file in output: %q", result)
	}
}

func TestGenerate_WithCheckboxItems(t *testing.T) {
	entries := []Entry{
		{
			IssueNumber: 1,
			IssueTitle:  "Test issue",
			IssueBody:   "Description.\n- [ ] Verify login works\n- [ ] Check logout",
			PRNumber:    10,
			Repo:        "owner/repo",
		},
	}

	result := Generate(entries)

	if !strings.Contains(result, "Verification steps (from issue):") {
		t.Errorf("missing verification steps header: %q", result)
	}
	if !strings.Contains(result, "- [ ] Verify login works") {
		t.Errorf("missing checkbox item in output: %q", result)
	}
	if !strings.Contains(result, "- [ ] Check logout") {
		t.Errorf("missing checkbox item in output: %q", result)
	}
}

func TestGenerate_WithScenarioSpec(t *testing.T) {
	spec := "# Scenario: Widget\n\nRelates to: Issue #42\n\n## Cases\n\n### Happy path\nDo the thing.\n\n### Error path\nBreak the thing.\n"
	entries := []Entry{
		{
			IssueNumber:  42,
			IssueTitle:   "Add widget",
			PRNumber:     99,
			Repo:         "owner/repo",
			ScenarioSpec: spec,
		},
	}

	result := Generate(entries)

	if !strings.Contains(result, "Scenario test cases:") {
		t.Errorf("missing scenario test cases header: %q", result)
	}
	if !strings.Contains(result, "Happy path") {
		t.Errorf("expected scenario case 'Happy path' in output: %q", result)
	}
	if !strings.Contains(result, "Error path") {
		t.Errorf("expected scenario case 'Error path' in output: %q", result)
	}
}

func TestGenerate_MultipleEntries(t *testing.T) {
	entries := []Entry{
		{IssueNumber: 1, IssueTitle: "First", PRNumber: 10, Repo: "owner/repo"},
		{IssueNumber: 2, IssueTitle: "Second", PRNumber: 11, Repo: "owner/repo"},
	}

	result := Generate(entries)

	if !strings.Contains(result, "1. Issue #1: First (PR #10)") {
		t.Errorf("missing first entry in output: %q", result)
	}
	if !strings.Contains(result, "2. Issue #2: Second (PR #11)") {
		t.Errorf("missing second entry in output: %q", result)
	}
}

func TestGenerate_NoChangedFilesSection(t *testing.T) {
	entries := []Entry{
		{IssueNumber: 1, IssueTitle: "No files", PRNumber: 5, Repo: "owner/repo"},
	}
	result := Generate(entries)
	if strings.Contains(result, "Changed files:") {
		t.Errorf("should not have 'Changed files' section when none: %q", result)
	}
}

func TestGenerate_NoScenarioSection(t *testing.T) {
	entries := []Entry{
		{IssueNumber: 1, IssueTitle: "No spec", PRNumber: 5, Repo: "owner/repo"},
	}
	result := Generate(entries)
	if strings.Contains(result, "Scenario test cases:") {
		t.Errorf("should not have 'Scenario test cases' section when empty: %q", result)
	}
}

func TestExtractVerificationSteps_Checkboxes(t *testing.T) {
	body := "Some intro.\n- [ ] Step one\n- [ ] Step two\n"
	items := extractVerificationSteps(body)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != "Step one" {
		t.Errorf("item 0 = %q, want %q", items[0], "Step one")
	}
	if items[1] != "Step two" {
		t.Errorf("item 1 = %q, want %q", items[1], "Step two")
	}
}

func TestExtractVerificationSteps_AsteriskCheckboxes(t *testing.T) {
	body := "* [ ] Check foo\n* [ ] Check bar\n"
	items := extractVerificationSteps(body)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %v", len(items), items)
	}
	if items[0] != "Check foo" {
		t.Errorf("item 0 = %q, want %q", items[0], "Check foo")
	}
}

func TestExtractVerificationSteps_TestSection(t *testing.T) {
	body := "## Test cases\n- Verify foo\n- Check bar\n\n## Other\n- Skip this\n"
	items := extractVerificationSteps(body)
	if len(items) != 2 {
		t.Fatalf("expected 2 items from test section, got %d: %v", len(items), items)
	}
}

func TestExtractVerificationSteps_AcceptanceCriteriaSection(t *testing.T) {
	body := "## Acceptance criteria\n- Feature works\n- No errors\n"
	items := extractVerificationSteps(body)
	if len(items) != 2 {
		t.Fatalf("expected 2 items from acceptance section, got %d: %v", len(items), items)
	}
}

func TestExtractVerificationSteps_VerificationSection(t *testing.T) {
	body := "## Verification steps\n- Check UI\n"
	items := extractVerificationSteps(body)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d: %v", len(items), items)
	}
}

func TestExtractVerificationSteps_NoItems(t *testing.T) {
	body := "Just a plain description with no checkboxes or test sections."
	items := extractVerificationSteps(body)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d: %v", len(items), items)
	}
}

func TestExtractVerificationSteps_MixedCheckboxAndSection(t *testing.T) {
	body := "- [ ] Standalone checkbox\n## Test cases\n- Bullet in section\n"
	items := extractVerificationSteps(body)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %v", len(items), items)
	}
}

func TestExtractScenarioCases_Basic(t *testing.T) {
	spec := "# Scenario\n\n## Setup\n...\n\n## Cases\n\n### Happy path\n...\n\n### Error path\n...\n"
	cases := extractScenarioCases(spec)
	if len(cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(cases))
	}
	if cases[0] != "Happy path" {
		t.Errorf("case 0 = %q, want %q", cases[0], "Happy path")
	}
	if cases[1] != "Error path" {
		t.Errorf("case 1 = %q, want %q", cases[1], "Error path")
	}
}

func TestExtractScenarioCases_Empty(t *testing.T) {
	spec := "# Scenario\nNo cases here.\n"
	cases := extractScenarioCases(spec)
	if len(cases) != 0 {
		t.Errorf("expected 0 cases, got %d: %v", len(cases), cases)
	}
}

func TestReadScenarioSpec_Found(t *testing.T) {
	dir := t.TempDir()
	specContent := "# Scenario: Test\n\nRelates to: Issue #42\n\n## Cases\n### Case 1\n..."
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ReadScenarioSpec(dir, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != specContent {
		t.Errorf("ReadScenarioSpec = %q, want %q", result, specContent)
	}
}

func TestReadScenarioSpec_NotFound(t *testing.T) {
	dir := t.TempDir()
	result, err := ReadScenarioSpec(dir, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string when no spec found, got %q", result)
	}
}

func TestReadScenarioSpec_NonExistentDir(t *testing.T) {
	_, err := ReadScenarioSpec("/nonexistent-dir-xyz-punchlist", 42)
	if err == nil {
		t.Error("expected error for nonexistent dir, got nil")
	}
}

func TestReadScenarioSpec_MultipleFiles_ReturnsFirst(t *testing.T) {
	dir := t.TempDir()
	spec1 := "# Scenario: First\n\nRelates to: Issue #5\n"
	spec2 := "# Scenario: Second\n\nRelates to: Issue #5\n"
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(spec1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte(spec2), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ReadScenarioSpec(dir, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result when specs exist")
	}
}

func TestFetchChangedFiles_Success(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("internal/foo.go\nweb/bar.html\n"), nil
	}

	files, err := FetchChangedFiles("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0] != "internal/foo.go" {
		t.Errorf("file 0 = %q, want %q", files[0], "internal/foo.go")
	}
	if files[1] != "web/bar.html" {
		t.Errorf("file 1 = %q, want %q", files[1], "web/bar.html")
	}
}

func TestFetchChangedFiles_PassesCorrectArgs(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var capturedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(""), nil
	}

	_, _ = FetchChangedFiles("owner/repo", 42)

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "gh") {
		t.Errorf("expected 'gh' command, got: %v", capturedArgs)
	}
	if !strings.Contains(joined, "pr diff") {
		t.Errorf("expected 'pr diff' in args, got: %v", capturedArgs)
	}
	if !strings.Contains(joined, "--name-only") {
		t.Errorf("expected '--name-only' flag, got: %v", capturedArgs)
	}
	if !strings.Contains(joined, "42") {
		t.Errorf("expected PR number '42' in args, got: %v", capturedArgs)
	}
}

func TestFetchChangedFiles_Error(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh error")
	}

	_, err := FetchChangedFiles("owner/repo", 42)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchChangedFiles_EmptyOutput(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}

	files, err := FetchChangedFiles("owner/repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty slice for empty output, got %v", files)
	}
}

func TestGenerate_WithAcceptanceTests(t *testing.T) {
	entries := []Entry{
		{
			IssueNumber: 42,
			IssueTitle:  "Add widget",
			PRNumber:    99,
			Repo:        "owner/repo",
			AcceptanceTests: []string{
				"Run godark with --no-sandbox, verify widget appears",
				"Set widget_count to 0 in config, verify graceful handling",
			},
		},
	}

	result := Generate(entries)

	if !strings.Contains(result, "Suggested acceptance tests:") {
		t.Errorf("missing acceptance tests header: %q", result)
	}
	if !strings.Contains(result, "- [ ] Run godark with --no-sandbox") {
		t.Errorf("missing acceptance test checkbox: %q", result)
	}
	if !strings.Contains(result, "- [ ] Set widget_count to 0") {
		t.Errorf("missing second acceptance test: %q", result)
	}
}

func TestGenerate_NoAcceptanceTestsSection(t *testing.T) {
	entries := []Entry{
		{IssueNumber: 1, IssueTitle: "No tests", PRNumber: 5, Repo: "owner/repo"},
	}
	result := Generate(entries)
	if strings.Contains(result, "Suggested acceptance tests:") {
		t.Errorf("should not have acceptance tests section when empty: %q", result)
	}
}

func TestFetchPRDiff_Success(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("diff --git a/foo.go b/foo.go\n+added line\n"), nil
	}

	diff, err := FetchPRDiff("owner/repo", 42, 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(diff, "+added line") {
		t.Errorf("expected diff content, got %q", diff)
	}
}

func TestFetchPRDiff_Error(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("gh error")
	}

	_, err := FetchPRDiff("owner/repo", 42, 10000)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPRDiff_Truncation(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()
	longDiff := strings.Repeat("x", 1000)
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(longDiff), nil
	}

	diff, err := FetchPRDiff("owner/repo", 42, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diff) != 100 {
		t.Errorf("expected truncated diff of 100 bytes, got %d", len(diff))
	}
}

func TestFetchPRDiff_NoTruncationWhenUnderLimit(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte("short"), nil
	}

	diff, err := FetchPRDiff("owner/repo", 42, 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "short" {
		t.Errorf("expected %q, got %q", "short", diff)
	}
}

func TestFetchPRDiff_PassesCorrectArgs(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var capturedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte(""), nil
	}

	_, _ = FetchPRDiff("owner/repo", 42, 1000)

	joined := strings.Join(capturedArgs, " ")
	if strings.Contains(joined, "--name-only") {
		t.Errorf("FetchPRDiff should NOT pass --name-only, got: %v", capturedArgs)
	}
	if !strings.Contains(joined, "42") {
		t.Errorf("expected PR number in args, got: %v", capturedArgs)
	}
}

func TestWrite_EmptyText(t *testing.T) {
	err := Write("", "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWrite_ToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "punchlist.txt")
	content := "test punchlist content\n"

	// Redirect stdout to avoid test noise.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Write(content, path)

	w.Close()
	os.Stdout = origStdout
	r.Close()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content = %q, want %q", string(data), content)
	}
}

func TestWrite_FileError(t *testing.T) {
	err := Write("content", "/nonexistent-dir-xyz/punchlist.txt")
	if err == nil {
		t.Fatal("expected error writing to nonexistent directory")
	}
}
