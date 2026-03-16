package cmd

import (
	"testing"
)

func TestWatchCmd_Registered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "watch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("watch command not registered on rootCmd")
	}
}

func TestWatchCmd_HasHelpOutput(t *testing.T) {
	if watchCmd.Use != "watch" {
		t.Errorf("watchCmd.Use: got %q, want %q", watchCmd.Use, "watch")
	}
	if watchCmd.Short == "" {
		t.Error("watchCmd.Short must not be empty")
	}
}
