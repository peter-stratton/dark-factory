package templates_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/peter-stratton/dark-factory/internal/harness/templates"
	"github.com/peter-stratton/dark-factory/prompts"
)

func TestWriteIfNotExists_FileWritten(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "architecture.md")

	written, err := templates.WriteIfNotExists("architecture.md", dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !written {
		t.Fatal("expected written=true for a new file")
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}

	want, err := templates.FS.ReadFile("architecture.md")
	if err != nil {
		t.Fatalf("reading embedded file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("file content mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestWriteIfNotExists_FileSkipped(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "conventions.md")

	existing := "pre-existing content"
	if err := os.WriteFile(dest, []byte(existing), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	written, err := templates.WriteIfNotExists("conventions.md", dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written {
		t.Fatal("expected written=false for an existing file")
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(got) != existing {
		t.Errorf("file was overwritten; got %q, want %q", got, existing)
	}
}

func TestWriteIfNotExists_ParentDirsCreated(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "a", "b", "c", "roadmap.md")

	written, err := templates.WriteIfNotExists("roadmap.md", dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !written {
		t.Fatal("expected written=true for a new file in nested dirs")
	}

	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("file not found after write: %v", err)
	}
}

func TestAllTemplatesAccessible(t *testing.T) {
	files := []string{
		"architecture.md",
		"architecture.json",
		"conventions.md",
		"roadmap.md",
		"claude.md",
		"godark.md",
		"gitignore",
	}

	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			data, err := templates.FS.ReadFile(f)
			if err != nil {
				t.Fatalf("FS.ReadFile(%q): %v", f, err)
			}
			if len(data) == 0 {
				t.Errorf("embedded file %q is empty", f)
			}
		})
	}
}

func TestImplementerPromptHasImplementationNotes(t *testing.T) {
	data, err := prompts.FS.ReadFile("implementer.txt")
	if err != nil {
		t.Fatalf("reading implementer prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## Implementation Notes") {
		t.Error("implementer prompt does not contain '## Implementation Notes' section format")
	}
}

func TestImplementerPromptHasArchitectureReference(t *testing.T) {
	data, err := prompts.FS.ReadFile("implementer.txt")
	if err != nil {
		t.Fatalf("reading implementer prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "{{.ArchitectureDocContent}}") {
		t.Error("implementer prompt does not reference architecture doc via {{.ArchitectureDocContent}} template variable")
	}
}

func TestImplementerPromptHasConventionsReference(t *testing.T) {
	data, err := prompts.FS.ReadFile("implementer.txt")
	if err != nil {
		t.Fatalf("reading implementer prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "{{.ConventionsDocContent}}") {
		t.Error("implementer prompt does not reference conventions doc via {{.ConventionsDocContent}} template variable")
	}
}

func TestImplementerPromptHasArchitectureConditional(t *testing.T) {
	data, err := prompts.FS.ReadFile("implementer.txt")
	if err != nil {
		t.Fatalf("reading implementer prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "{{if .ArchitectureDocContent}}") && !strings.Contains(content, "{{- if .ArchitectureDocContent}}") {
		t.Error("implementer prompt does not guard {{.ArchitectureDocContent}} with a conditional")
	}
}

func TestRetryPromptReferencesImplementationNotes(t *testing.T) {
	data, err := prompts.FS.ReadFile("implementer_retry.txt")
	if err != nil {
		t.Fatalf("reading retry prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Implementation Notes") {
		t.Error("retry prompt does not reference prior Implementation Notes")
	}
}

func TestRetryPromptReferencesReviewerChallenges(t *testing.T) {
	data, err := prompts.FS.ReadFile("implementer_retry.txt")
	if err != nil {
		t.Fatalf("reading retry prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Review Notes") {
		t.Error("retry prompt does not reference reviewer challenges (Review Notes)")
	}
}

func TestReviewerPromptHasReviewNotes(t *testing.T) {
	data, err := prompts.FS.ReadFile("reviewer.txt")
	if err != nil {
		t.Fatalf("reading reviewer prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## Review Notes") {
		t.Error("reviewer prompt does not contain '## Review Notes' section format")
	}
}

func TestReviewerPromptHasArchitectureComplianceCheck(t *testing.T) {
	data, err := prompts.FS.ReadFile("reviewer.txt")
	if err != nil {
		t.Fatalf("reading reviewer prompt: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "architecture compliance") && !strings.Contains(content, "{{.ArchitectureDoc}}") {
		t.Error("reviewer prompt does not include architecture compliance check instruction")
	}
}

func TestGitignoreIncludesRequiredEntries(t *testing.T) {
	data, err := templates.FS.ReadFile("gitignore")
	if err != nil {
		t.Fatalf("reading gitignore: %v", err)
	}
	content := string(data)
	for _, entry := range []string{"tests/review/", "logs/"} {
		if !strings.Contains(content, entry) {
			t.Errorf("gitignore does not contain %q", entry)
		}
	}
}

func TestSemiformalReviewerPromptHasFiveSections(t *testing.T) {
	data, err := prompts.FS.ReadFile("reviewer_semiformal.txt")
	if err != nil {
		t.Fatalf("reading reviewer_semiformal prompt: %v", err)
	}
	content := string(data)
	for _, section := range []string{
		"### PREMISES",
		"### ACCEPTANCE TRACE",
		"### REGRESSION TRACE",
		"### UNCOVERED PATHS",
		"### FORMAL CONCLUSION",
	} {
		if !strings.Contains(content, section) {
			t.Errorf("reviewer_semiformal prompt missing section %q", section)
		}
	}
}

func TestSemiformalReviewerPromptSemiformalPresentWithoutSpec(t *testing.T) {
	data, err := prompts.FS.ReadFile("reviewer_semiformal.txt")
	if err != nil {
		t.Fatalf("reading reviewer_semiformal prompt: %v", err)
	}

	tmpl, err := template.New("reviewer_semiformal").Parse(string(data))
	if err != nil {
		t.Fatalf("parsing reviewer_semiformal prompt: %v", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, map[string]any{
		"HasScenarioSpec": false,
	}); err != nil {
		t.Fatalf("executing reviewer_semiformal prompt: %v", err)
	}

	output := buf.String()
	for _, section := range []string{
		"### PREMISES",
		"### ACCEPTANCE TRACE",
		"### REGRESSION TRACE",
		"### UNCOVERED PATHS",
		"### FORMAL CONCLUSION",
	} {
		if !strings.Contains(output, section) {
			t.Errorf("reviewer_semiformal output missing %q with HasScenarioSpec=false", section)
		}
	}
	if strings.Contains(output, "MANDATORY: Generate integration tests") {
		t.Error("reviewer_semiformal output should not contain integration test step when HasScenarioSpec=false")
	}
}

func TestSemiformalReviewerPromptSemiformalAnalysisPresentTwice(t *testing.T) {
	data, err := prompts.FS.ReadFile("reviewer_semiformal.txt")
	if err != nil {
		t.Fatalf("reading reviewer_semiformal prompt: %v", err)
	}
	content := string(data)
	count := strings.Count(content, "### Semi-Formal Analysis")
	if count < 2 {
		t.Errorf("reviewer_semiformal prompt should contain '### Semi-Formal Analysis' at least twice (once per comment template), got %d", count)
	}
}
