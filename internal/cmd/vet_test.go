package cmd

import "testing"

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
