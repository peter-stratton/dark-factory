package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/phs/dark-factory/internal/harness/templates"
	"github.com/spf13/cobra"
)

// gitRunner executes a command and returns its combined output. Replaceable for testing.
var gitRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

var newCmd = &cobra.Command{
	Use:   "new <project-name>",
	Short: "Create a new project with all harness files",
	Long: `Create a new directory with all harness files for a greenfield project.

Creates the directory, writes CLAUDE.md and .gitignore, runs git init,
then scaffolds skills, godark.yaml, and harness documentation files.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		repo, _ := cmd.Flags().GetString("repo")
		return runNew(cmd, projectName, repo)
	},
}

func init() {
	newCmd.Flags().String("repo", "", "GitHub repository (owner/repo) — pre-populates repo: in godark.yaml")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, projectName, repo string) error {
	// Check if directory already exists.
	if _, err := os.Stat(projectName); err == nil {
		return fmt.Errorf("directory %q already exists", projectName)
	}

	// Create the project directory.
	if err := os.MkdirAll(projectName, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", projectName, err)
	}

	absProjectDir, err := filepath.Abs(projectName)
	if err != nil {
		return fmt.Errorf("resolving project directory: %w", err)
	}

	// Write CLAUDE.md from embedded templates.
	claudeMDData, err := templates.FS.ReadFile("claude.md")
	if err != nil {
		return fmt.Errorf("reading embedded CLAUDE.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(absProjectDir, "CLAUDE.md"), claudeMDData, 0o644); err != nil {
		return fmt.Errorf("writing CLAUDE.md: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s/CLAUDE.md\n", projectName)

	// Write .gitignore from embedded templates.
	gitignoreData, err := templates.FS.ReadFile("gitignore")
	if err != nil {
		return fmt.Errorf("reading embedded .gitignore: %w", err)
	}
	if err := os.WriteFile(filepath.Join(absProjectDir, ".gitignore"), gitignoreData, 0o644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s/.gitignore\n", projectName)

	// Run git init in the new directory.
	if _, err := gitRunner("git", "-C", absProjectDir, "init"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "initialized git repository in %s/\n", projectName)

	// Switch to the project directory so init-like helpers work relative to it.
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	if err := os.Chdir(absProjectDir); err != nil {
		return fmt.Errorf("changing to project directory: %w", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not restore working directory: %v\n", err)
		}
	}()

	// Write skill files (reuses the shared helper from init.go).
	if err := writeSkillFiles(cmd); err != nil {
		return err
	}

	// Write .claude/godark.md (reuses the shared helper from init.go).
	if err := writeGodarkDoc(cmd); err != nil {
		return err
	}

	// Write godark.yaml with optional repo pre-populated.
	if err := writeNewProjectConfig(cmd, repo); err != nil {
		return err
	}

	// Write harness documentation files.
	if err := writeNewProjectHarnessDocs(cmd); err != nil {
		return err
	}

	// Ensure godark labels exist on the repo (no-op when --repo is not set).
	if err := ensureLabels(cmd); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nProject %q created.\n", projectName)
	fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  1. Fill in the language-specific sections of CLAUDE.md\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  2. Run /godark-create-milestone to define phases\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  3. Use `godark vet` to validate before execution\n")

	return nil
}

// writeNewProjectConfig writes godark.yaml into the current directory, with the
// repo field pre-populated if repo is non-empty.
func writeNewProjectConfig(cmd *cobra.Command, repo string) error {
	// New projects have no go.mod yet, so use generic (empty) command examples.
	config := buildDefaultConfig("", "")
	if repo != "" {
		safe := strings.ReplaceAll(repo, `"`, `\"`)
		config = strings.Replace(config, `repo: ""`, fmt.Sprintf(`repo: "%s"`, safe), 1)
	}
	if err := os.WriteFile("godark.yaml", []byte(config), 0o644); err != nil {
		return fmt.Errorf("writing godark.yaml: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote godark.yaml\n")
	return nil
}

// writeNewProjectHarnessDocs scaffolds harness documentation files into the
// current directory. Unlike writeHarnessDocs, it does not check --reset-claude-md
// or print a CLAUDE.md guidance hint (CLAUDE.md is already written by runNew).
func writeNewProjectHarnessDocs(cmd *cobra.Command) error {
	for _, dir := range []string{"docs/planning", "tests/scenarios"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	for _, f := range harnessDocFiles {
		written, err := templates.WriteIfNotExists(f.src, f.dest)
		if err != nil {
			return err
		}
		if written {
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", f.dest)
		}
	}

	// Prompt templates are always overwritten from prompts.FS (single source
	// of truth), matching the behavior of godark init.
	return writeHarnessPrompts(cmd)
}
