package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/peter-stratton/dark-factory/internal/harness/layers"
	"github.com/peter-stratton/dark-factory/internal/vet"
	"github.com/spf13/cobra"
)

var vetArchitectureCmd = &cobra.Command{
	Use:   "architecture",
	Short: "Validate architecture layer definitions for DAG correctness",
	RunE: func(cmd *cobra.Command, args []string) error {
		archFile, _ := cmd.Flags().GetString("architecture-file")
		asJSON, _ := cmd.Flags().GetBool("json")

		def, err := layers.ParseFile(archFile)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(cmd.OutOrStdout(), "info: architecture file %q not found — skipping (harnesses are opt-in)\n", archFile)
				return nil
			}
			return fmt.Errorf("parsing architecture file: %w", err)
		}

		report := vet.ValidateArchitecture(def)
		printReport(cmd, report, asJSON)
		return nil
	},
}

func init() {
	f := vetArchitectureCmd.Flags()
	f.String("architecture-file", "docs/architecture.json", "Path to architecture JSON file")
	f.Bool("json", false, "Output findings as JSON")
	vetCmd.AddCommand(vetArchitectureCmd)
}
