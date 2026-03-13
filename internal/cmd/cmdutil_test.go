package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// newTestCmd returns a cobra.Command with all 7 flags registered, mirroring
// the flag definitions used by runCmd and implementCmd.
func newTestCmd() *cobra.Command {
	c := &cobra.Command{Use: "test"}
	f := c.Flags()
	f.String("repo", "", "")
	f.Int("max-retries", 3, "")
	f.Bool("no-sandbox", false, "")
	f.String("auto-merge-feature", "none", "")
	f.String("auto-merge-rollup", "none", "")
	f.String("base-branch", "", "")
	f.String("default-branch", "", "")
	return c
}

func TestParseCLIFlags_NoFlagsChanged(t *testing.T) {
	t.Helper()
	cmd := newTestCmd()
	flags := parseCLIFlags(cmd)

	if flags.Repo != nil {
		t.Errorf("Repo: want nil, got %v", flags.Repo)
	}
	if flags.MaxRetries != nil {
		t.Errorf("MaxRetries: want nil, got %v", flags.MaxRetries)
	}
	if flags.NoSandbox != nil {
		t.Errorf("NoSandbox: want nil, got %v", flags.NoSandbox)
	}
	if flags.AutoMergeFeature != nil {
		t.Errorf("AutoMergeFeature: want nil, got %v", flags.AutoMergeFeature)
	}
	if flags.AutoMergeRollup != nil {
		t.Errorf("AutoMergeRollup: want nil, got %v", flags.AutoMergeRollup)
	}
	if flags.BaseBranch != nil {
		t.Errorf("BaseBranch: want nil, got %v", flags.BaseBranch)
	}
	if flags.DefaultBranch != nil {
		t.Errorf("DefaultBranch: want nil, got %v", flags.DefaultBranch)
	}
	if flags.Config != "" {
		t.Errorf("Config: want empty, got %q", flags.Config)
	}
}

func TestParseCLIFlags_AllFlagsSet(t *testing.T) {
	t.Helper()
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{
		"--repo", "owner/repo",
		"--max-retries", "5",
		"--no-sandbox",
		"--auto-merge-feature", "all",
		"--auto-merge-rollup", "auto",
		"--base-branch", "main",
		"--default-branch", "main",
	}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	flags := parseCLIFlags(cmd)

	if flags.Repo == nil || *flags.Repo != "owner/repo" {
		t.Errorf("Repo: want %q, got %v", "owner/repo", flags.Repo)
	}
	if flags.MaxRetries == nil || *flags.MaxRetries != 5 {
		t.Errorf("MaxRetries: want 5, got %v", flags.MaxRetries)
	}
	if flags.NoSandbox == nil || !*flags.NoSandbox {
		t.Errorf("NoSandbox: want true, got %v", flags.NoSandbox)
	}
	if flags.AutoMergeFeature == nil || *flags.AutoMergeFeature != "all" {
		t.Errorf("AutoMergeFeature: want %q, got %v", "all", flags.AutoMergeFeature)
	}
	if flags.AutoMergeRollup == nil || *flags.AutoMergeRollup != "auto" {
		t.Errorf("AutoMergeRollup: want %q, got %v", "auto", flags.AutoMergeRollup)
	}
	if flags.BaseBranch == nil || *flags.BaseBranch != "main" {
		t.Errorf("BaseBranch: want %q, got %v", "main", flags.BaseBranch)
	}
	if flags.DefaultBranch == nil || *flags.DefaultBranch != "main" {
		t.Errorf("DefaultBranch: want %q, got %v", "main", flags.DefaultBranch)
	}
}

func TestParseCLIFlags_PartialFlags(t *testing.T) {
	t.Helper()
	cmd := newTestCmd()
	if err := cmd.ParseFlags([]string{
		"--repo", "owner/repo",
		"--no-sandbox",
	}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	flags := parseCLIFlags(cmd)

	if flags.Repo == nil || *flags.Repo != "owner/repo" {
		t.Errorf("Repo: want %q, got %v", "owner/repo", flags.Repo)
	}
	if flags.NoSandbox == nil || !*flags.NoSandbox {
		t.Errorf("NoSandbox: want true, got %v", flags.NoSandbox)
	}
	// Unset fields must remain nil.
	if flags.MaxRetries != nil {
		t.Errorf("MaxRetries: want nil, got %v", flags.MaxRetries)
	}
	if flags.AutoMergeFeature != nil {
		t.Errorf("AutoMergeFeature: want nil, got %v", flags.AutoMergeFeature)
	}
	if flags.AutoMergeRollup != nil {
		t.Errorf("AutoMergeRollup: want nil, got %v", flags.AutoMergeRollup)
	}
	if flags.BaseBranch != nil {
		t.Errorf("BaseBranch: want nil, got %v", flags.BaseBranch)
	}
	if flags.DefaultBranch != nil {
		t.Errorf("DefaultBranch: want nil, got %v", flags.DefaultBranch)
	}
}
