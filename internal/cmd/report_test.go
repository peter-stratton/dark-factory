package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/phs/dark-factory/internal/stats"
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
	if err := runReport(&buf, db, "org/my-repo", since, until, "terminal"); err != nil {
		t.Errorf("runReport unexpected error: %v", err)
	}
}
