package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "godark",
	Short: "Autonomous AI agent orchestrator for GitHub issues",
	Long: `godark orchestrates autonomous AI agents to implement GitHub issues,
review their own work, and merge — without human intervention.

It fetches issues from a milestone, resolves dependencies, and runs a
two-agent loop: one agent implements, another reviews. Approved PRs are
squash-merged automatically.`,
}

func Execute() error {
	return rootCmd.Execute()
}
