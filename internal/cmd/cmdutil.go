package cmd

import (
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/spf13/cobra"
)

// parseCLIFlags reads the 7 flag-overridable fields from cmd and returns a
// partially-populated CLIFlags. Only flags that were explicitly set by the
// caller (Changed == true) are written; unset flags leave their pointer fields
// nil. The Config field is not handled here — callers set it separately via
// cmd.Flags().GetString("config").
func parseCLIFlags(cmd *cobra.Command) config.CLIFlags {
	var flags config.CLIFlags

	if cmd.Flags().Changed("repo") {
		v, _ := cmd.Flags().GetString("repo")
		flags.Repo = &v
	}
	if cmd.Flags().Changed("max-retries") {
		v, _ := cmd.Flags().GetInt("max-retries")
		flags.MaxRetries = &v
	}
	if cmd.Flags().Changed("auto-merge-feature") {
		v, _ := cmd.Flags().GetString("auto-merge-feature")
		flags.AutoMergeFeature = &v
	}
	if cmd.Flags().Changed("auto-merge-rollup") {
		v, _ := cmd.Flags().GetString("auto-merge-rollup")
		flags.AutoMergeRollup = &v
	}
	if cmd.Flags().Changed("base-branch") {
		v, _ := cmd.Flags().GetString("base-branch")
		flags.BaseBranch = &v
	}
	if cmd.Flags().Changed("default-branch") {
		v, _ := cmd.Flags().GetString("default-branch")
		flags.DefaultBranch = &v
	}
	if cmd.Flags().Changed("no-judge") {
		v, _ := cmd.Flags().GetBool("no-judge")
		flags.NoJudge = &v
	}
	if cmd.Flags().Changed("model") {
		v, _ := cmd.Flags().GetString("model")
		flags.Model = &v
	}
	if cmd.Flags().Changed("integration") {
		v, _ := cmd.Flags().GetBool("integration")
		flags.Integration = &v
	}
	if cmd.Flags().Changed("workers") {
		v, _ := cmd.Flags().GetInt("workers")
		flags.Workers = &v
	}

	return flags
}
