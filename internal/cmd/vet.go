package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var vetCmd = &cobra.Command{
	Use:   "vet",
	Short: "Validate roadmap docs and issue quality for agent consumption",
	Long: `Check that issues have clear acceptance criteria, correct blocker
notations, and are fully actionable by agents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("vet: not implemented yet (Phase 3)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(vetCmd)
}
