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
three-agent loop: one agent implements, a second audits code quality, and
a third reviews against scenario specs. Approved PRs are squash-merged
automatically.`,
}

func Execute() error {
	return rootCmd.Execute()
}
