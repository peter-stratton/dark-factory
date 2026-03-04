package cmd

import (
	"fmt"

	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/vet"
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

		var issues []github.Issue
		var allNums map[int]bool

		if milestone != "" {
			if repo == "" {
				return fmt.Errorf("--repo is required when --milestone or --tag is specified")
			}
			var err error
			issues, err = github.FetchMilestoneIssues(repo, milestone)
			if err != nil {
				return fmt.Errorf("fetching milestone issues: %w", err)
			}
			allNums, err = github.FetchAllIssueNumbers(repo)
			if err != nil {
				return fmt.Errorf("fetching all issue numbers: %w", err)
			}
		} else if repo != "" {
			var err error
			allNums, err = github.FetchAllIssueNumbers(repo)
			if err != nil {
				return fmt.Errorf("fetching all issue numbers: %w", err)
			}
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
