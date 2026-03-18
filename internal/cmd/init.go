package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/peter-stratton/dark-factory/internal/detect"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/harness/templates"
	"github.com/peter-stratton/dark-factory/internal/label"
	"github.com/peter-stratton/dark-factory/internal/lock"
	"github.com/peter-stratton/dark-factory/internal/skills"
	"github.com/spf13/cobra"
)

// detectedCommands holds auto-detected build/test commands for config generation.
type detectedCommands struct {
	build  string
	test   string
	format string
	lint   string
}

// buildDefaultConfig constructs the default godark.yaml config string.
// When detected is non-nil, build/test/format/lint commands are written as
// active (uncommented) fields. Otherwise they remain commented-out placeholders.
func buildDefaultConfig(detected *detectedCommands) string {
	var commandBlock string
	if detected != nil && (detected.build != "" || detected.test != "") {
		commandBlock = "# Build, test, format, and lint commands (auto-detected — verify for your project)\n"
		if detected.build != "" {
			commandBlock += fmt.Sprintf("build_command: %q\n", detected.build)
		}
		if detected.test != "" {
			commandBlock += fmt.Sprintf("test_command: %q\n", detected.test)
		}
		if detected.format != "" {
			commandBlock += fmt.Sprintf("format_command: %q\n", detected.format)
		}
		if detected.lint != "" {
			commandBlock += fmt.Sprintf("lint_command: %q\n", detected.lint)
		}
	} else {
		commandBlock = `# Build, test, format, and lint commands (all optional)
# build_command: ""  # run before the verify step
# test_command: ""   # run by the verify step
# format_command: "" # run during verify step
# lint_command: ""   # run during verify step
`
	}

	return `# godark.yaml — Configuration for godark
repo: ""              # GitHub repository (owner/repo)

# Merge behavior — two-tier config:
#   feature: controls how feature PRs are merged after both reviewers approve
#     none     — label PR awaiting-human-review (default, safest)
#     low_risk — auto-merge small/safe PRs, label others for human review
#     all      — auto-merge everything
#   rollup: controls rollup PR behavior (only relevant when base_branch ≠ default_branch)
#     none   — godark stops after feature PRs; human opens and merges rollup PR
#     manual — godark opens rollup PR; human reviews and merges
#     auto   — godark opens and merges rollup PR
auto_merge:
  feature: none
  rollup: none

` + commandBlock + `
# Paths (defaults shown — override to customize)
# roadmap_path: docs/ROADMAP.md
# planning_dir: docs/planning/
# scenario_dir: tests/scenarios/

# Patch coverage enforcement (optional)
# When set, the verify step computes patch coverage after tests pass and fails
# if the percentage of covered changed lines is below patch_target.
# coverage:
#   patch_target: 80          # 1-100, percentage of changed lines that must be covered
#   test_command: ""           # optional override (must write -coverprofile=/tmp/cov.out)

# Prompt templates
prompts:
  implementer: prompts/implementer.txt
  implementer_retry: prompts/implementer_retry.txt
  reviewer: prompts/reviewer.txt

# Default branch of the repository (auto-detected from GitHub if omitted)
# default_branch: main

# Docker Compose integration (optional)
# When set, godark manages compose services alongside the sandbox.
# docker_compose:
#   file: docker-compose.test.yml     # path to compose file (required)
#   project_name: ""                  # optional prefix (auto-generated if empty)
#   services:                         # optional: describe services for agent context
#     - name: postgres
#       description: "PostgreSQL 16 test database on port 5432"
#     - name: redis
#       description: "Redis 7 cache on port 6379"

# Agent timeout (Go duration format: "30m", "1h", etc.)
# agent_timeout: "30m"
`
}

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
		if err := writeGodarkDoc(cmd); err != nil {
			return err
		}
		if err := writeDefaultConfig(cmd); err != nil {
			return err
		}
		if err := writeHarnessDocs(cmd); err != nil {
			return err
		}
		return ensureLabels(cmd)
	},
}

func init() {
	initCmd.Flags().String("repo", "", "GitHub repository (owner/repo) — used to create the godark-in-progress label")
	initCmd.Flags().Bool("reset-claude-md", false, "replace existing CLAUDE.md with the harness template")
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

// writeGodarkDoc writes .claude/godark.md from the embedded template. This file
// is always overwritten (like skills) since it describes godark's pipeline and
// config, and should stay current with the installed version of godark.
func writeGodarkDoc(cmd *cobra.Command) error {
	const dest = ".claude/godark.md"

	data, err := templates.FS.ReadFile("godark.md")
	if err != nil {
		return fmt.Errorf("reading embedded godark.md: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", dest, err)
	}

	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", dest)
	return nil
}

func writeDefaultConfig(cmd *cobra.Command) error {
	const configPath = "godark.yaml"

	repo, _ := cmd.Flags().GetString("repo")

	if _, err := os.Stat(configPath); err == nil {
		// File exists — update the repo field if --repo was provided.
		if repo != "" {
			if err := patchConfigRepo(cmd, repo); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "skipped %s (already exists)\n", configPath)
		}
		return nil
	}

	detected := detectCommandsForDir(".")
	content := buildDefaultConfig(detected)
	if repo != "" {
		safe := strings.ReplaceAll(repo, `"`, `\"`)
		content = strings.Replace(content, `repo: ""`, fmt.Sprintf(`repo: "%s"`, safe), 1)
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", configPath)
	return nil
}

// patchConfigRepo updates the repo field in an existing godark.yaml file.
func patchConfigRepo(cmd *cobra.Command, repo string) error {
	const path = "godark.yaml"

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	safe := strings.ReplaceAll(repo, `"`, `\"`)
	old := string(data)
	updated := strings.Replace(old, `repo: ""`, fmt.Sprintf(`repo: "%s"`, safe), 1)

	if old == updated {
		fmt.Fprintf(cmd.OutOrStdout(), "skipped %s (repo already set)\n", path)
		return nil
	}

	if err := os.WriteFile(filepath.Clean(path), []byte(updated), 0o644); err != nil { //nolint:gosec // path is a hardcoded constant
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "updated repo in %s\n", path)
	return nil
}

// detectCommandsForDir runs runtime detection and returns commands to write
// into godark.yaml. Returns nil if no runtime is detected.
func detectCommandsForDir(dir string) *detectedCommands {
	dp, err := detect.DetectRuntime(dir)
	if err != nil {
		return nil
	}
	dc := &detectedCommands{
		build: dp.BuildCommand,
		test:  dp.TestCommand,
	}
	// Add language-specific format/lint suggestions.
	if dp.Runtime.Name == "go" {
		dc.format = "gofmt -l -d ."
		dc.lint = "golangci-lint run ./..."
	}
	return dc
}

// writeHarnessDocs scaffolds harness documentation files into the current
// directory. Files are created only if they do not already exist, except for
// CLAUDE.md which is only written when --reset-claude-md is set.
func writeHarnessDocs(cmd *cobra.Command) error {
	resetClaudeMD, _ := cmd.Flags().GetBool("reset-claude-md")

	// Create directories unconditionally; MkdirAll is idempotent.
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
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "skipped %s (already exists)\n", f.dest)
		}
	}

	// Prompt templates are always overwritten (managed by godark, like skills).
	// Read from prompts.FS (single source of truth) rather than maintaining a
	// separate copy in the harness templates.
	if err := writeHarnessPrompts(cmd); err != nil {
		return err
	}

	claudeMDWritten := false
	if resetClaudeMD {
		if _, err := os.Stat("CLAUDE.md"); err == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "replacing existing CLAUDE.md with harness template\n")
		}
		data, err := templates.FS.ReadFile("claude.md")
		if err != nil {
			return fmt.Errorf("reading embedded CLAUDE.md: %w", err)
		}
		if err := os.WriteFile("CLAUDE.md", data, 0o644); err != nil {
			return fmt.Errorf("writing CLAUDE.md: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote CLAUDE.md\n")
		claudeMDWritten = true
	}

	if !claudeMDWritten {
		fmt.Fprintf(cmd.OutOrStdout(), "hint: review your CLAUDE.md against the harness principles in docs/architecture.md (use --reset-claude-md to replace it)\n")
	}

	return nil
}

// ensureLabels creates all godark labels in the repo if --repo is provided.
// This includes the lock label (godark-in-progress) and PR lifecycle labels
// (godark:awaiting-human-review, godark:fixing-review-feedback, godark:ready-to-merge).
// If no repo is available, this step is silently skipped.
func ensureLabels(cmd *cobra.Command) error {
	repo, _ := cmd.Flags().GetString("repo")
	if repo == "" {
		return nil
	}

	// Lock label.
	if err := lock.EnsureLabelExists(repo); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create lock label: %v\n", err)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ensured label %q in %s\n", label.InProgress, repo)

	// PR lifecycle labels.
	for _, spec := range label.Specs {
		if err := github.EnsureLabel(repo, spec.Name, spec.Color, spec.Description); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create label %q: %v\n", spec.Name, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ensured label %q in %s\n", spec.Name, repo)
	}

	return nil
}
