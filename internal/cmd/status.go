package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show summary of the most recent run",
	Long:  `Parse the latest run log and display a summary of issues processed, PRs opened, and outcomes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("status: not implemented yet (Phase 5)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
