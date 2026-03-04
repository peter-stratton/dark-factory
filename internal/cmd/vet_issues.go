package cmd

import (
	"fmt"

	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/vet"
	"github.com/spf13/cobra"
)

var vetIssuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "Validate GitHub issue structure for agent consumption",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		milestone, _ := cmd.Flags().GetString("milestone")
		asJSON, _ := cmd.Flags().GetBool("json")

		if repo == "" || milestone == "" {
			return fmt.Errorf("--repo and --milestone are required")
		}

		issues, err := github.FetchMilestoneIssues(repo, milestone)
		if err != nil {
			return fmt.Errorf("fetching milestone issues: %w", err)
		}
		if len(issues) == 0 {
			return fmt.Errorf("no open issues found for milestone %q — check that the milestone exists and has open issues", milestone)
		}

		allNums, err := github.FetchAllIssueNumbers(repo)
		if err != nil {
			return fmt.Errorf("fetching all issue numbers: %w", err)
		}

		phaseLabel := milestoneToLabel(milestone)
		report := vet.ValidateIssues(issues, allNums, phaseLabel)

		printReport(cmd, report, asJSON)
		return nil
	},
}

func init() {
	f := vetIssuesCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to validate")
	f.Bool("json", false, "Output findings as JSON")

	vetCmd.AddCommand(vetIssuesCmd)
}
