package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the development loop for a milestone or single issue",
	Long: `Fetch issues from a GitHub milestone, resolve dependencies, and process
each unblocked issue through the implement → review → merge loop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("run: not implemented yet (Phase 2)")
		return nil
	},
}

func init() {
	f := runCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to process")
	f.Int("issue", 0, "Single issue number to process (instead of milestone)")
	f.Int("max-retries", 2, "Maximum review/fix retry cycles per issue")
	f.Bool("dry-run", false, "Print execution plan without taking action")
	f.Bool("no-sandbox", false, "Run agents on host instead of in Docker")
	f.String("config", "godark.yaml", "Path to configuration file")

	rootCmd.AddCommand(runCmd)
}
