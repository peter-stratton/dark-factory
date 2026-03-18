package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/peter-stratton/dark-factory/internal/stats"
)

// TestReportCommandRegistered checks the report command is wired to root.
func TestReportCommandRegistered(t *testing.T) {
	names := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}
	if !names["report"] {
		t.Error("report subcommand not registered on rootCmd")
	}
}

// TestReportCommandFlags verifies all required flags are present with correct defaults.
func TestReportCommandFlags(t *testing.T) {
	for _, name := range []string{"since", "until", "repo", "format"} {
		if reportCmd.Flags().Lookup(name) == nil {
			t.Errorf("report command missing flag --%s", name)
		}
	}

	since := reportCmd.Flags().Lookup("since")
	if since.DefValue != "2w" {
		t.Errorf("--since default = %q, want %q", since.DefValue, "2w")
	}

	format := reportCmd.Flags().Lookup("format")
	if format.DefValue != "terminal" {
		t.Errorf("--format default = %q, want %q", format.DefValue, "terminal")
	}
}

// TestParseSinceDuration covers valid and invalid duration strings.
func TestParseSinceDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"2w", 14 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"4w", 28 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"abc", 0, true},
		{"d", 0, true},
		{"w", 0, true},
		{"0d", 0, true},
		{"0w", 0, true},
		{"-1d", 0, true},
		{"2m", 0, true},
		{"", 0, true},
		{"2", 0, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			got, err := parseSinceDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseSinceDuration(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSinceDuration(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseSinceDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestOpenStatsDBAtMissing exercises the real production error message when the DB file is absent.
func TestOpenStatsDBAtMissing(t *testing.T) {
	_, err := openStatsDBAt("/tmp/this-path-does-not-exist-godark-stats-test.db")
	if err == nil {
		t.Fatal("expected error for missing database, got nil")
	}
	if !strings.Contains(err.Error(), "no stats database found") {
		t.Errorf("error = %q, want to contain 'no stats database found'", err.Error())
	}
	if !strings.Contains(err.Error(), "godark run") && !strings.Contains(err.Error(), "godark implement") {
		t.Errorf("error = %q, want to mention 'godark run' or 'godark implement'", err.Error())
	}
}

// TestReportMissingDatabase verifies the error propagates through RunE when stats.db does not exist.
func TestReportMissingDatabase(t *testing.T) {
	orig := newReportDB
	newReportDB = func() (*stats.DB, error) {
		return openStatsDBAt("/tmp/this-path-does-not-exist-godark-stats-test.db")
	}
	defer func() { newReportDB = orig }()

	cmd := reportCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	_ = cmd.Flags().Set("since", "2w")
	_ = cmd.Flags().Set("until", "")
	_ = cmd.Flags().Set("repo", "")
	_ = cmd.Flags().Set("format", "terminal")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing database, got nil")
	}
	if !strings.Contains(err.Error(), "no stats database found") {
		t.Errorf("error = %q, want to contain 'no stats database found'", err.Error())
	}
	if !strings.Contains(err.Error(), "godark run") && !strings.Contains(err.Error(), "godark implement") {
		t.Errorf("error = %q, want to mention 'godark run' or 'godark implement'", err.Error())
	}
}

// TestReportInvalidFormat verifies that an unsupported --format value returns an error.
func TestReportInvalidFormat(t *testing.T) {
	orig := newReportDB
	newReportDB = func() (*stats.DB, error) {
		return stats.Open(":memory:")
	}
	defer func() { newReportDB = orig }()

	cmd := reportCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	_ = cmd.Flags().Set("since", "2w")
	_ = cmd.Flags().Set("until", "")
	_ = cmd.Flags().Set("repo", "")
	_ = cmd.Flags().Set("format", "pdf")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --format") {
		t.Errorf("error = %q, want to contain 'invalid --format'", err.Error())
	}
}

// TestReportValidFormats verifies all supported --format values are accepted without error.
func TestReportValidFormats(t *testing.T) {
	for _, format := range []string{"terminal", "markdown", "html"} {
		t.Run(format, func(t *testing.T) {
			orig := newReportDB
			newReportDB = func() (*stats.DB, error) {
				return stats.Open(":memory:")
			}
			defer func() { newReportDB = orig }()

			cmd := reportCmd
			var buf bytes.Buffer
			cmd.SetOut(&buf)

			_ = cmd.Flags().Set("since", "2w")
			_ = cmd.Flags().Set("until", "")
			_ = cmd.Flags().Set("repo", "")
			_ = cmd.Flags().Set("format", format)

			if err := cmd.RunE(cmd, nil); err != nil {
				t.Errorf("--format %s unexpected error: %v", format, err)
			}
		})
	}
}

// TestReportRepoFilter verifies that --repo is passed through to the stats query without error.
func TestReportRepoFilter(t *testing.T) {
	orig := newReportDB
	newReportDB = func() (*stats.DB, error) {
		return stats.Open(":memory:")
	}
	defer func() { newReportDB = orig }()

	cmd := reportCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	_ = cmd.Flags().Set("since", "2w")
	_ = cmd.Flags().Set("until", "")
	_ = cmd.Flags().Set("repo", "org/my-repo")
	_ = cmd.Flags().Set("format", "terminal")

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("unexpected error with --repo: %v", err)
	}
}

// TestRunReportFilter verifies that runReport passes the repo filter to the DB query.
func TestRunReportFilter(t *testing.T) {
	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("stats.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var buf bytes.Buffer
	since := time.Now().Add(-14 * 24 * time.Hour)
	until := time.Now()

	// runReport should not error on an empty DB with a repo filter.
	// noSummary=true to skip the API call in the test environment.
	if err := runReport(&buf, db, "org/my-repo", since, until, "terminal", true); err != nil {
		t.Errorf("runReport unexpected error: %v", err)
	}
}

// TestReportNoSummaryFlag verifies the --no-summary flag is registered with correct defaults.
func TestReportNoSummaryFlag(t *testing.T) {
	f := reportCmd.Flags().Lookup("no-summary")
	if f == nil {
		t.Fatal("report command missing flag --no-summary")
	}
	if f.DefValue != "false" {
		t.Errorf("--no-summary default = %q, want %q", f.DefValue, "false")
	}
}

// TestReportNoSummarySkipsAPI verifies that --no-summary=true results in no
// generateSummary call.
func TestReportNoSummarySkipsAPI(t *testing.T) {
	orig := generateSummary
	called := false
	generateSummary = func(_ context.Context, _ string) (string, error) {
		called = true
		return "summary", nil
	}
	defer func() { generateSummary = orig }()

	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("stats.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var buf bytes.Buffer
	since := time.Now().Add(-7 * 24 * time.Hour)
	until := time.Now()

	if err := runReport(&buf, db, "", since, until, "terminal", true); err != nil {
		t.Fatalf("runReport unexpected error: %v", err)
	}
	if called {
		t.Error("generateSummary was called despite noSummary=true")
	}
}

// TestReportAPIFailureGraceful verifies that a generateSummary error does not
// prevent the report from rendering.
func TestReportAPIFailureGraceful(t *testing.T) {
	orig := generateSummary
	generateSummary = func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("API unreachable")
	}
	defer func() { generateSummary = orig }()

	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("stats.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	since := time.Now().Add(-7 * 24 * time.Hour)
	until := time.Now()

	// Insert a run and outcome so the API call is triggered (len(outcomes) > 0).
	ctx := context.Background()
	run := stats.RunRecord{ID: "run-graceful", Repo: "org/repo", StartedAt: time.Now().Add(-1 * time.Hour), FinishedAt: time.Now()}
	if err := stats.WriteRun(ctx, db, run); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	outcome := stats.IssueOutcomeRecord{RunID: "run-graceful", IssueNumber: 1, Title: "Test issue", Status: "implemented"}
	if err := stats.WriteIssueOutcome(ctx, db, outcome); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}

	var buf bytes.Buffer
	// Should not return an error even though generateSummary failed.
	if err := runReport(&buf, db, "", since, until, "terminal", false); err != nil {
		t.Errorf("runReport returned unexpected error on API failure: %v", err)
	}
}

// TestReportSummaryInjected verifies that a successful generateSummary call
// results in the summary text appearing in the rendered output.
func TestReportSummaryInjected(t *testing.T) {
	const wantSummary = "This sprint delivered major improvements to the pipeline."

	orig := generateSummary
	generateSummary = func(_ context.Context, _ string) (string, error) {
		return wantSummary, nil
	}
	defer func() { generateSummary = orig }()

	db, err := stats.Open(":memory:")
	if err != nil {
		t.Fatalf("stats.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	since := time.Now().Add(-7 * 24 * time.Hour)
	until := time.Now()

	// Insert a run and outcome so len(outcomes) > 0 and the summary path is exercised.
	ctx := context.Background()
	run := stats.RunRecord{ID: "run-summary", Repo: "org/repo", StartedAt: time.Now().Add(-1 * time.Hour), FinishedAt: time.Now()}
	if err := stats.WriteRun(ctx, db, run); err != nil {
		t.Fatalf("WriteRun: %v", err)
	}
	outcome := stats.IssueOutcomeRecord{RunID: "run-summary", IssueNumber: 42, Title: "Add dashboard widget", Status: "implemented"}
	if err := stats.WriteIssueOutcome(ctx, db, outcome); err != nil {
		t.Fatalf("WriteIssueOutcome: %v", err)
	}

	var buf bytes.Buffer
	if err := runReport(&buf, db, "", since, until, "terminal", false); err != nil {
		t.Fatalf("runReport unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, wantSummary) {
		t.Errorf("report output missing injected summary text;\ngot: %s", out)
	}
	if !strings.Contains(out, "Executive Summary") {
		t.Errorf("report output missing 'Executive Summary' header;\ngot: %s", out)
	}
}
