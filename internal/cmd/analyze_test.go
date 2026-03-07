package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/analysis"
	"github.com/phs/dark-factory/internal/rundata"
)

// writeJSONFile marshals v and writes it to path, creating parent dirs as needed.
func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling JSON: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}

// writeRunDir creates a run directory with run.json and per-issue outcome files.
func writeRunDir(t *testing.T, base, owner, repo string, meta rundata.RunMeta, outcomes []rundata.Outcome) {
	t.Helper()
	timestamp := meta.StartedAt.UTC().Format("20060102-150405")
	runDir := filepath.Join(base, owner, repo, timestamp)
	writeJSONFile(t, filepath.Join(runDir, "run.json"), meta)
	for _, o := range outcomes {
		issueDir := filepath.Join(runDir, "issues", fmt.Sprintf("%d", o.IssueNumber))
		writeJSONFile(t, filepath.Join(issueDir, "outcome.json"), o)
	}
}

// analyzeWithReader calls runAnalyze with a reader rooted at base and returns stdout.
func analyzeWithReader(t *testing.T, base, repo, milestone, sinceStr, untilStr string, jsonOut bool) (string, error) {
	t.Helper()
	reader := rundata.NewReaderWithBase(base, nil)
	since, until, err := parseDateRange(sinceStr, untilStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = runAnalyze(&buf, reader, nil, repo, milestone, since, until, jsonOut)
	return buf.String(), err
}

// TestAnalyzeCommandRegistered checks the analyze command is wired to root.
func TestAnalyzeCommandRegistered(t *testing.T) {
	names := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["analyze"] {
		t.Error("analyze subcommand not registered on rootCmd")
	}
}

// TestAnalyzeCommandFlags verifies all required flags are present.
func TestAnalyzeCommandFlags(t *testing.T) {
	for _, name := range []string{"repo", "milestone", "since", "until", "json"} {
		if analyzeCmd.Flags().Lookup(name) == nil {
			t.Errorf("analyze command missing flag --%s", name)
		}
	}
}

// TestAnalyzeEmptyState: no run data directory prints message and exits 0.
func TestAnalyzeEmptyState(t *testing.T) {
	base := t.TempDir()
	out, err := analyzeWithReader(t, base, "", "", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No matching runs found") {
		t.Errorf("output = %q, want 'No matching runs found'", out)
	}
}

// TestAnalyzeFullReport: given run data, command prints all report sections.
func TestAnalyzeFullReport(t *testing.T) {
	base := t.TempDir()
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	meta := rundata.RunMeta{
		Repo:         "owner/repo",
		Milestone:    "Phase 1",
		IssueNumbers: []int{1, 2},
		StartedAt:    ts,
	}
	writeRunDir(t, base, "owner", "repo", meta, []rundata.Outcome{
		{IssueNumber: 1, Status: "implemented"},
		{IssueNumber: 2, Status: "failed"},
	})

	out, err := analyzeWithReader(t, base, "", "", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		"Analyzed 1 runs, 2 issues",
		"Outcomes",
		"Flag Frequencies",
		"Retry Stats",
		"Cost Stats",
		"Prompt Gaps",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

// TestAnalyzeRepoFilter: --repo owner/other excludes non-matching runs.
func TestAnalyzeRepoFilter(t *testing.T) {
	base := t.TempDir()
	ts1 := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 1, 11, 10, 0, 0, 0, time.UTC)

	writeRunDir(t, base, "owner", "repo", rundata.RunMeta{
		Repo: "owner/repo", Milestone: "m1", IssueNumbers: []int{1}, StartedAt: ts1,
	}, []rundata.Outcome{{IssueNumber: 1, Status: "implemented"}})

	writeRunDir(t, base, "owner", "other", rundata.RunMeta{
		Repo: "owner/other", Milestone: "m1", IssueNumbers: []int{2}, StartedAt: ts2,
	}, []rundata.Outcome{{IssueNumber: 2, Status: "implemented"}})

	out, err := analyzeWithReader(t, base, "owner/other", "", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Analyzed 1 runs, 1 issues") {
		t.Errorf("expected 1 run after repo filter, got:\n%s", out)
	}
}

// TestAnalyzeMilestoneFilter: --milestone "Phase 7" includes only Phase 7 runs.
func TestAnalyzeMilestoneFilter(t *testing.T) {
	base := t.TempDir()
	ts1 := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)

	writeRunDir(t, base, "owner", "repo", rundata.RunMeta{
		Repo: "owner/repo", Milestone: "Phase 7", IssueNumbers: []int{1}, StartedAt: ts1,
	}, []rundata.Outcome{{IssueNumber: 1, Status: "implemented"}})

	writeRunDir(t, base, "owner", "repo", rundata.RunMeta{
		Repo: "owner/repo", Milestone: "Phase 8", IssueNumbers: []int{2}, StartedAt: ts2,
	}, []rundata.Outcome{{IssueNumber: 2, Status: "failed"}})

	out, err := analyzeWithReader(t, base, "", "Phase 7", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Analyzed 1 runs, 1 issues") {
		t.Errorf("expected 1 run after milestone filter, got:\n%s", out)
	}
}

// TestAnalyzeDateFilter: --since 2026-01-01 --until 2026-02-01 includes only January runs.
func TestAnalyzeDateFilter(t *testing.T) {
	base := t.TempDir()

	dec := time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC)
	jan := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)

	for i, ts := range []time.Time{dec, jan, feb} {
		issueNum := i + 1
		writeRunDir(t, base, "owner", "repo", rundata.RunMeta{
			Repo: "owner/repo", Milestone: "m1", IssueNumbers: []int{issueNum}, StartedAt: ts,
		}, []rundata.Outcome{{IssueNumber: issueNum, Status: "implemented"}})
	}

	out, err := analyzeWithReader(t, base, "", "", "2026-01-01", "2026-02-01", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Analyzed 1 runs, 1 issues") {
		t.Errorf("expected 1 run (January only) after date filter, got:\n%s", out)
	}
}

// TestAnalyzeJSONOutput: --json produces valid JSON with report and gaps fields.
func TestAnalyzeJSONOutput(t *testing.T) {
	base := t.TempDir()
	ts := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)

	writeRunDir(t, base, "owner", "repo", rundata.RunMeta{
		Repo: "owner/repo", Milestone: "m1", IssueNumbers: []int{1}, StartedAt: ts,
	}, []rundata.Outcome{{IssueNumber: 1, Status: "implemented"}})

	out, err := analyzeWithReader(t, base, "", "", "", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result struct {
		Report analysis.Report      `json:"report"`
		Gaps   []analysis.PromptGap `json:"gaps"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput:\n%s", err, out)
	}

	if result.Report.RunCount != 1 {
		t.Errorf("report.run_count = %d, want 1", result.Report.RunCount)
	}
	if result.Report.IssueCount != 1 {
		t.Errorf("report.issue_count = %d, want 1", result.Report.IssueCount)
	}
}

// TestAnalyzeNoMatchesRepoFilter: filters that match no runs print message and exit 0.
func TestAnalyzeNoMatchesRepoFilter(t *testing.T) {
	base := t.TempDir()
	ts := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)

	writeRunDir(t, base, "owner", "repo", rundata.RunMeta{
		Repo: "owner/repo", Milestone: "m1", IssueNumbers: []int{1}, StartedAt: ts,
	}, []rundata.Outcome{{IssueNumber: 1, Status: "implemented"}})

	out, err := analyzeWithReader(t, base, "owner/nonexistent", "", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No matching runs found") {
		t.Errorf("output = %q, want 'No matching runs found'", out)
	}
}

// TestParseDateRange covers valid and invalid inputs.
func TestParseDateRange(t *testing.T) {
	tests := []struct {
		name    string
		since   string
		until   string
		wantErr bool
	}{
		{"empty", "", "", false},
		{"rfc3339", "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", false},
		{"date only", "2026-01-01", "2026-02-01", false},
		{"invalid since", "not-a-date", "", true},
		{"invalid until", "", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			since, until, err := parseDateRange(tt.since, tt.until)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.since == "" && since != nil {
				t.Error("since should be nil for empty string")
			}
			if tt.until == "" && until != nil {
				t.Error("until should be nil for empty string")
			}
		})
	}
}

// TestFilterRunMetas checks all filter combinations.
func TestFilterRunMetas(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	metas := []rundata.RunMeta{
		{Repo: "owner/a", Milestone: "m1", StartedAt: t1},
		{Repo: "owner/b", Milestone: "m1", StartedAt: t2},
		{Repo: "owner/a", Milestone: "m2", StartedAt: t3},
	}

	tests := []struct {
		name      string
		repo      string
		milestone string
		since     *time.Time
		until     *time.Time
		wantCount int
	}{
		{"no filter", "", "", nil, nil, 3},
		{"repo filter", "owner/a", "", nil, nil, 2},
		{"milestone filter", "", "m1", nil, nil, 2},
		{"since filter (inclusive)", "", "", &t2, nil, 2},
		{"until filter (inclusive)", "", "", nil, &t2, 2},
		{"since+until same day", "", "", &t2, &t2, 1},
		{"repo+milestone", "owner/a", "m1", nil, nil, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterRunMetas(metas, tt.repo, tt.milestone, tt.since, tt.until)
			if len(got) != tt.wantCount {
				t.Errorf("filterRunMetas() = %d results, want %d", len(got), tt.wantCount)
			}
		})
	}
}

// TestSplitRepo covers valid and invalid repo strings.
func TestSplitRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"owner/repo/extra", "owner", "repo/extra", false},
		{"noslash", "", "", true},
		{"", "", "", true},
		{"/noowner", "", "", true},
		{"owner/", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, name, err := splitRepo(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}
