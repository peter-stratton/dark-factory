package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/phs/dark-factory/prompts"
	"github.com/spf13/cobra"
)

// harnessDocFiles is the canonical list of harness documentation templates
// written by both "init" and "new". Entries map embedded template names to
// destination paths relative to the project root.
var harnessDocFiles = []struct{ src, dest string }{
	{"architecture.md", "docs/architecture.md"},
	{"architecture.json", "docs/architecture.json"},
	{"conventions.md", "docs/conventions.md"},
	{"roadmap.md", "docs/ROADMAP.md"},
}

// harnessPromptFiles is the canonical list of prompt template files written
// by both "init" and "new". Entries map embedded prompt names to destination
// paths relative to the project root.
var harnessPromptFiles = []struct{ name, dest string }{
	{"implementer.txt", "prompts/implementer.txt"},
	{"implementer_retry.txt", "prompts/implementer_retry.txt"},
	{"reviewer.txt", "prompts/reviewer.txt"},
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

// writeHarnessPrompts writes all harness prompt files to disk using
// writeFileWithDirs. Prompt files are always overwritten (they are managed by
// godark, like skills).
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
	return nil
}
