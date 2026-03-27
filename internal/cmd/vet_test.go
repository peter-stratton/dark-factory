package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVetCommandHasSubcommands(t *testing.T) {
	names := make(map[string]bool)
	for _, sub := range vetCmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"issues", "scenarios", "roadmap", "architecture"} {
		if !names[want] {
			t.Errorf("vet missing subcommand %q", want)
		}
	}
}

func TestVetIssuesFlags(t *testing.T) {
	for _, name := range []string{"repo", "milestone", "json"} {
		if vetIssuesCmd.Flags().Lookup(name) == nil {
			t.Errorf("vet issues missing flag --%s", name)
		}
	}
}

func TestVetScenariosFlags(t *testing.T) {
	for _, name := range []string{"repo", "milestone", "json", "scenario-dir"} {
		if vetScenariosCmd.Flags().Lookup(name) == nil {
			t.Errorf("vet scenarios missing flag --%s", name)
		}
	}
}

func TestVetRoadmapFlags(t *testing.T) {
	for _, name := range []string{"repo", "milestone", "json", "planning-dir"} {
		if vetRoadmapCmd.Flags().Lookup(name) == nil {
			t.Errorf("vet roadmap missing flag --%s", name)
		}
	}
}

func TestVetArchitectureFlags(t *testing.T) {
	for _, name := range []string{"architecture-file", "json"} {
		if vetArchitectureCmd.Flags().Lookup(name) == nil {
			t.Errorf("vet architecture missing flag --%s", name)
		}
	}
}

func TestVetScenariosRequiresRepoAndMilestone(t *testing.T) {
	cmd := vetScenariosCmd
	cmd.ResetFlags()
	f := cmd.Flags()
	f.String("repo", "", "")
	f.String("milestone", "", "")
	f.String("tag", "", "")
	f.Bool("json", false, "")
	f.String("scenario-dir", "tests/scenarios/", "")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when --repo and --milestone are missing, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error to contain 'required', got: %q", err.Error())
	}
}

func TestVetScenariosRequiresMilestone(t *testing.T) {
	cmd := vetScenariosCmd
	cmd.ResetFlags()
	f := cmd.Flags()
	f.String("repo", "", "")
	f.String("milestone", "", "")
	f.String("tag", "", "")
	f.Bool("json", false, "")
	f.String("scenario-dir", "tests/scenarios/", "")
	if err := f.Set("repo", "owner/repo"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when --milestone/--tag is missing, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error to contain 'required', got: %q", err.Error())
	}
}

func TestVetScenariosMissingSubdir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "phase-1"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := vetScenariosCmd
	cmd.ResetFlags()
	f := cmd.Flags()
	f.String("repo", "", "")
	f.String("milestone", "", "")
	f.String("tag", "", "")
	f.Bool("json", false, "")
	f.String("scenario-dir", tmp, "")
	if err := f.Set("repo", "owner/repo"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("milestone", "Phase 99"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing phase-99 subdir, got nil")
	}
	if !strings.Contains(err.Error(), "phase-99") {
		t.Errorf("expected error to name the missing phase, got: %q", err.Error())
	}
}

func TestVetScenariosScopedDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "phase-1"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := vetScenariosCmd
	cmd.ResetFlags()
	f := cmd.Flags()
	f.String("repo", "", "")
	f.String("milestone", "", "")
	f.String("tag", "", "")
	f.Bool("json", false, "")
	f.String("scenario-dir", tmp, "")
	if err := f.Set("repo", "owner/repo"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("milestone", "Phase 1"); err != nil {
		t.Fatal(err)
	}

	err := cmd.RunE(cmd, nil)
	// The command may fail (e.g., GitHub API unavailable in tests), but it must
	// NOT return the missing-subdir error since phase-1/ exists and is the
	// correct scoped directory.
	if err != nil && strings.Contains(err.Error(), "no scenario directory found") {
		t.Errorf("expected phase-1/ to be found and used, got: %q", err.Error())
	}
}

func TestMilestoneToLabel(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Phase 2", "phase-2"},
		{"Phase 10", "phase-10"},
		{"Phase 2: Vault Reader + Foundation", "phase-2"},
		{"phase 3: Something", "phase-3"},
		{"My Milestone", "my-milestone"},
	}
	for _, tt := range tests {
		if got := milestoneToLabel(tt.in); got != tt.want {
			t.Errorf("milestoneToLabel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
