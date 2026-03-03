package cmd

import (
	"testing"
)

func TestRootCommandHasSubcommands(t *testing.T) {
	names := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"init", "run", "status", "vet"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestRunCommandFlags(t *testing.T) {
	flags := []string{"repo", "milestone", "issue", "max-retries", "dry-run", "no-sandbox", "config"}
	for _, name := range flags {
		if runCmd.Flags().Lookup(name) == nil {
			t.Errorf("run command missing flag --%s", name)
		}
	}
}

func TestConfigFlagDefault(t *testing.T) {
	f := runCmd.Flags().Lookup("config")
	if f.DefValue != "godark.yaml" {
		t.Errorf("config default = %q, want %q", f.DefValue, "godark.yaml")
	}
}
