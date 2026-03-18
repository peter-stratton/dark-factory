package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/peter-stratton/dark-factory/internal/lock"
	"github.com/spf13/cobra"
)

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Clear a stale run lock left by a crashed godark instance",
	Long: `Remove the godark-in-progress label from all open issues in the repo
and delete the local .godark/lock.json file.

Use this command when a previous godark run crashed mid-execution and left
the lock label on issues, preventing new runs from starting.

Alternatively, pass --force to godark run to automatically clear the stale
lock before starting a new run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		if repo == "" {
			return fmt.Errorf("--repo is required")
		}

		logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		locker := lock.New(repo, logger)
		count, err := locker.ReleaseAll()
		if err != nil {
			return fmt.Errorf("releasing lock: %w", err)
		}

		if count == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No lock found — nothing to clear.")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Cleared lock from %d issue(s).\n", count)
		}
		return nil
	},
}

func init() {
	unlockCmd.Flags().String("repo", "", "GitHub repository (owner/repo)")
	rootCmd.AddCommand(unlockCmd)
}
