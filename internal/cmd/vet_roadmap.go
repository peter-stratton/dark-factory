package cmd

import (
	"fmt"

	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/vet"
	"github.com/spf13/cobra"
)

var vetRoadmapCmd = &cobra.Command{
	Use:   "roadmap",
	Short: "Validate planning docs against milestone issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		milestone, _ := cmd.Flags().GetString("milestone")
		asJSON, _ := cmd.Flags().GetBool("json")
		planningDir, _ := cmd.Flags().GetString("planning-dir")

		if repo == "" || milestone == "" {
			return fmt.Errorf("--repo and --milestone are required")
		}

		issues, err := github.FetchMilestoneIssues(repo, milestone)
		if err != nil {
			return fmt.Errorf("fetching milestone issues: %w", err)
		}

		allNums, err := github.FetchAllIssueNumbers(repo)
		if err != nil {
			return fmt.Errorf("fetching all issue numbers: %w", err)
		}

		report := vet.ValidateRoadmap(planningDir, issues, milestone, allNums)

		printReport(cmd, report, asJSON)
		return nil
	},
}

func init() {
	f := vetRoadmapCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to validate")
	f.Bool("json", false, "Output findings as JSON")
	f.String("planning-dir", "docs/planning/", "Path to planning docs directory")

	vetCmd.AddCommand(vetRoadmapCmd)
}
