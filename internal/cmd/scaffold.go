package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peter-stratton/dark-factory/internal/harness/templates"
	"github.com/peter-stratton/dark-factory/prompts"
	"github.com/spf13/cobra"
)

// harnessDocFiles is the canonical list of harness documentation templates
// written by both "init" and "new". Entries map embedded template names to
// destination paths relative to the project root.
var harnessDocFiles = []struct{ src, dest string }{
	{"architecture.md", "docs/architecture.md"},
	{"architecture.json", "docs/architecture.json"},
	{"conventions.md", "docs/conventions.md"},
	{"roadmap.md", "docs/roadmap/README.md"},
}

// harnessPromptFiles is the canonical list of prompt template files written
// by both "init" and "new". Entries map embedded prompt names to destination
// paths relative to the project root.
var harnessPromptFiles = []struct{ name, dest string }{
	{"implementer.txt", "prompts/implementer.txt"},
	{"implementer_retry.txt", "prompts/implementer_retry.txt"},
	{"reviewer.txt", "prompts/reviewer.txt"},
	{"quality_reviewer.txt", "prompts/quality_reviewer.txt"},
	{"spec_generator.txt", "prompts/spec_generator.txt"},
	{"punchlist.txt", "prompts/punchlist.txt"},
	{"verify_fix.txt", "prompts/verify_fix.txt"},
	{"recon.txt", "prompts/recon.txt"},
	{"planner.txt", "prompts/planner.txt"},
	{"merge_coordinator.txt", "prompts/merge_coordinator.txt"},
	{"reviewer_semiformal.txt", "prompts/reviewer_semiformal.txt"},
}

// writeFileWithDirs creates parent directories for path and writes data to it,
// always overwriting any existing file.
func writeFileWithDirs(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// writeHarnessPrompts writes all harness prompt files and the prompts README
// to disk using writeFileWithDirs. Prompt files are always overwritten (they
// are managed by godark, like skills).
func writeHarnessPrompts(cmd *cobra.Command) error {
	for _, f := range harnessPromptFiles {
		data, err := prompts.FS.ReadFile(f.name)
		if err != nil {
			return fmt.Errorf("reading embedded prompt %s: %w", f.name, err)
		}
		if err := writeFileWithDirs(f.dest, data); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", f.dest)
	}

	// Write the prompts README from harness templates.
	readme, err := templates.FS.ReadFile("prompts-readme.md")
	if err != nil {
		return fmt.Errorf("reading embedded prompts README: %w", err)
	}
	if err := writeFileWithDirs("prompts/README.md", readme); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote prompts/README.md\n")

	return nil
}
