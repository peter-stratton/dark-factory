package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peter-stratton/dark-factory/internal/vet"
	"github.com/spf13/cobra"
)

var vetScenariosCmd = &cobra.Command{
	Use:   "scenarios",
	Short: "Validate scenario spec files",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := resolveRepo(cmd)
		asJSON, _ := cmd.Flags().GetBool("json")
		scenarioDir, _ := cmd.Flags().GetString("scenario-dir")

		milestone, err := resolveTag(cmd)
		if err != nil {
			return err
		}

		if repo == "" || milestone == "" {
			return fmt.Errorf("--repo and either --milestone or --tag are required")
		}

		phaseLabel := milestoneToLabel(milestone)
		scopedDir := filepath.Join(scenarioDir, phaseLabel)
		if _, err := os.Stat(scopedDir); os.IsNotExist(err) {
			return fmt.Errorf("no scenario directory found for %s: %s", phaseLabel, scopedDir+"/")
		}

		issues, allNums, err := fetchVetData(repo, milestone)
		if err != nil {
			return err
		}

		report := vet.ValidateScenarios(scopedDir, issues, allNums)

		printReport(cmd, report, asJSON)
		return nil
	},
}

func init() {
	f := vetScenariosCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to validate (exact title)")
	f.String("tag", "", "Milestone tag (e.g., phase-3) — resolved to full milestone title")
	f.Bool("json", false, "Output findings as JSON")
	f.String("scenario-dir", "tests/scenarios/", "Path to scenario spec directory")

	vetCmd.AddCommand(vetScenariosCmd)
}
