package cmd

import (
	"github.com/spf13/cobra"
)

var vetCmd = &cobra.Command{
	Use:   "vet",
	Short: "Validate roadmap docs and issue quality for agent consumption",
	Long: `Check that issues have clear acceptance criteria, correct blocker
notations, and are fully actionable by agents.

Use a subcommand to validate a specific aspect:
  godark vet issues     Validate GitHub issue structure
  godark vet scenarios  Validate scenario spec files
  godark vet roadmap    Validate planning docs`,
}

func init() {
	rootCmd.AddCommand(vetCmd)
}
