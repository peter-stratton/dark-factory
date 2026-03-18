package cmd

import (
	"fmt"

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

		if milestone != "" && repo == "" {
			return fmt.Errorf("--repo is required when --milestone or --tag is specified")
		}

		issues, allNums, err := fetchVetData(repo, milestone)
		if err != nil {
			return err
		}

		report := vet.ValidateScenarios(scenarioDir, issues, allNums)

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
