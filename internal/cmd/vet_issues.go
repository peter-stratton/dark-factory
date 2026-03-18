package cmd

import (
	"fmt"

	"github.com/peter-stratton/dark-factory/internal/vet"
	"github.com/spf13/cobra"
)

var vetIssuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "Validate GitHub issue structure for agent consumption",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := resolveRepo(cmd)
		asJSON, _ := cmd.Flags().GetBool("json")

		milestone, err := resolveTag(cmd)
		if err != nil {
			return err
		}
		if repo == "" || milestone == "" {
			return fmt.Errorf("--repo and either --milestone or --tag are required")
		}

		issues, allNums, err := fetchVetData(repo, milestone)
		if err != nil {
			return err
		}
		if len(issues) == 0 {
			return fmt.Errorf("no open issues found for milestone %q — check that the milestone exists and has open issues", milestone)
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
	f.String("milestone", "", "GitHub milestone to validate (exact title)")
	f.String("tag", "", "Milestone tag (e.g., phase-3) — resolved to full milestone title")
	f.Bool("json", false, "Output findings as JSON")

	vetCmd.AddCommand(vetIssuesCmd)
}
