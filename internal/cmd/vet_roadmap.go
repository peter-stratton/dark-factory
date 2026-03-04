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
		repo := resolveRepo(cmd)
		asJSON, _ := cmd.Flags().GetBool("json")
		planningDir, _ := cmd.Flags().GetString("planning-dir")

		milestone, err := resolveTag(cmd)
		if err != nil {
			return err
		}
		if repo == "" || milestone == "" {
			return fmt.Errorf("--repo and either --milestone or --tag are required")
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
	f.String("milestone", "", "GitHub milestone to validate (exact title)")
	f.String("tag", "", "Milestone tag (e.g., phase-3) — resolved to full milestone title")
	f.Bool("json", false, "Output findings as JSON")
	f.String("planning-dir", "docs/planning/", "Path to planning docs directory")

	vetCmd.AddCommand(vetRoadmapCmd)
}
