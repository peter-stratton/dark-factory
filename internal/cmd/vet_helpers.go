package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/patterns"
	"github.com/phs/dark-factory/internal/vet"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// printReport outputs the report in table or JSON format and exits non-zero
// if any errors were found.
func printReport(cmd *cobra.Command, report *vet.Report, asJSON bool) {
	if asJSON {
		_ = report.PrintJSON(cmd.OutOrStdout())
	} else {
		report.Print(cmd.OutOrStdout())
	}
	if code := report.ExitCode(); code != 0 {
		os.Exit(code)
	}
}

// milestoneToLabel extracts the phase label from a milestone title.
// "Phase 2" and "Phase 2: Vault Reader + Foundation" both become "phase-2".
// Falls back to full lowercase-hyphenated slug if no "Phase N" prefix is found.
func milestoneToLabel(milestone string) string {
	if m := patterns.Phase.FindStringSubmatch(milestone); m != nil {
		return "phase-" + m[1]
	}
	return strings.ReplaceAll(strings.ToLower(milestone), " ", "-")
}

// resolveRepo returns the repo from --repo flag if explicitly set, otherwise
// falls back to reading the config file's repo field.
func resolveRepo(cmd *cobra.Command) string {
	if cmd.Flags().Changed("repo") {
		repo, _ := cmd.Flags().GetString("repo")
		return repo
	}
	cfgPath, _ := cmd.Flags().GetString("config")
	if cfgPath == "" {
		cfgPath = "godark.yaml"
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return ""
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.Repo
}

// resolveTag resolves --milestone and --tag flags into a milestone title.
// --milestone takes precedence. If --tag is provided, it looks up the
// matching milestone from the repo. Returns ("", nil) if neither is set.
func resolveTag(cmd *cobra.Command) (string, error) {
	milestone, _ := cmd.Flags().GetString("milestone")
	tag, _ := cmd.Flags().GetString("tag")
	repo := resolveRepo(cmd)

	if milestone != "" && tag != "" {
		return "", fmt.Errorf("--milestone and --tag are mutually exclusive")
	}
	if tag != "" {
		if repo == "" {
			return "", fmt.Errorf("--repo is required when using --tag")
		}
		title, err := github.ResolveMilestoneByTag(repo, tag)
		if err != nil {
			return "", err
		}
		return title, nil
	}
	return milestone, nil
}
