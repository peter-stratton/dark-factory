package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/phs/dark-factory/internal/lock"
	"github.com/phs/dark-factory/internal/skills"
	"github.com/spf13/cobra"
)

const defaultConfig = `# godark.yaml — Configuration for godark
repo: ""              # GitHub repository (owner/repo)

# Paths (defaults shown — override to customize)
# roadmap_path: docs/ROADMAP.md
# planning_dir: docs/planning/
# scenario_dir: tests/scenarios/

# Prompt templates
prompts:
  implementer: prompts/implementer.txt
  implementer_retry: prompts/implementer_retry.txt
  reviewer: prompts/reviewer.txt

# Agent timeout (Go duration format: "30m", "1h", etc.)
# agent_timeout: "30m"
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a project with godark skills and default config",
	Long: `Write Claude Code skill files and a default godark.yaml to the current
directory. Skills are always overwritten (they are managed by godark).
The config file is only created if it does not already exist.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := writeSkillFiles(cmd); err != nil {
			return err
		}
		if err := writeDefaultConfig(cmd); err != nil {
			return err
		}
		return createLockLabel(cmd)
	},
}

func init() {
	initCmd.Flags().String("repo", "", "GitHub repository (owner/repo) — used to create the godark-in-progress label")
	rootCmd.AddCommand(initCmd)
}

func writeSkillFiles(cmd *cobra.Command) error {
	return fs.WalkDir(skills.SkillFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		dest := filepath.Join(".claude", "skills", path)

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		data, err := fs.ReadFile(skills.SkillFiles, path)
		if err != nil {
			return fmt.Errorf("reading embedded file %s: %w", path, err)
		}

		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", dest)
		return nil
	})
}

func writeDefaultConfig(cmd *cobra.Command) error {
	const configPath = "godark.yaml"

	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "skipped %s (already exists)\n", configPath)
		return nil
	}

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", configPath)
	return nil
}

// createLockLabel creates the godark-in-progress label in the repo if --repo
// is provided. If no repo is available, this step is silently skipped.
func createLockLabel(cmd *cobra.Command) error {
	repo, _ := cmd.Flags().GetString("repo")
	if repo == "" {
		return nil
	}
	if err := lock.EnsureLabelExists(repo); err != nil {
		// Warn but do not fail init — the label will be created lazily on first run.
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create lock label: %v\n", err)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ensured label %q in %s\n", lock.LockLabel, repo)
	return nil
}
