package cmd

import (
	"testing"
)

func TestRootCommandHasSubcommands(t *testing.T) {
	names := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}

	for _, want := range []string{"init", "report", "run", "status", "vet"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestRunCommandFlags(t *testing.T) {
	flags := []string{"repo", "milestone", "max-retries", "dry-run", "config"}
	for _, name := range flags {
		if runCmd.Flags().Lookup(name) == nil {
			t.Errorf("run command missing flag --%s", name)
		}
	}
	// no-sandbox flag must not be registered.
	if runCmd.Flags().Lookup("no-sandbox") != nil {
		t.Error("run command must not have --no-sandbox flag")
	}
}

func TestConfigFlagDefault(t *testing.T) {
	f := runCmd.Flags().Lookup("config")
	if f.DefValue != "godark.yaml" {
		t.Errorf("config default = %q, want %q", f.DefValue, "godark.yaml")
	}
}
