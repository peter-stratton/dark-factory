package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phs/dark-factory/internal/config"
)

func TestLoadPrompts_Success(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"impl.txt":  "implement {{.IssueNumber}}",
		"retry.txt": "retry {{.PRNumber}}",
		"rev.txt":   "review {{.PRNumber}}",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Prompts: config.Prompts{
			Implementer:      filepath.Join(dir, "impl.txt"),
			ImplementerRetry: filepath.Join(dir, "retry.txt"),
			Reviewer:         filepath.Join(dir, "rev.txt"),
		},
	}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.Implementer != files["impl.txt"] {
		t.Errorf("Implementer = %q, want %q", p.Implementer, files["impl.txt"])
	}
	if p.ImplementerRetry != files["retry.txt"] {
		t.Errorf("ImplementerRetry = %q, want %q", p.ImplementerRetry, files["retry.txt"])
	}
	if p.Reviewer != files["rev.txt"] {
		t.Errorf("Reviewer = %q, want %q", p.Reviewer, files["rev.txt"])
	}
}

func TestLoadPrompts_MissingFile(t *testing.T) {
	cfg := &config.Config{
		Prompts: config.Prompts{
			Implementer:      "/nonexistent/implementer.txt",
			ImplementerRetry: "/nonexistent/retry.txt",
			Reviewer:         "/nonexistent/reviewer.txt",
		},
	}

	_, err := LoadPrompts(cfg)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadPrompts_EmbeddedDefaults(t *testing.T) {
	// No prompt paths configured — should load from embedded defaults.
	cfg := &config.Config{}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.Implementer == "" {
		t.Error("Implementer should be loaded from embedded default")
	}
	if p.ImplementerRetry == "" {
		t.Error("ImplementerRetry should be loaded from embedded default")
	}
	if p.Reviewer == "" {
		t.Error("Reviewer should be loaded from embedded default")
	}
	if p.SpecGenerator == "" {
		t.Error("SpecGenerator should be loaded from embedded default")
	}
}

func TestLoadPrompts_SpecGeneratorDefaultsToEmbedded(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"impl.txt", "retry.txt", "rev.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("template"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Prompts: config.Prompts{
			Implementer:      filepath.Join(dir, "impl.txt"),
			ImplementerRetry: filepath.Join(dir, "retry.txt"),
			Reviewer:         filepath.Join(dir, "rev.txt"),
			// SpecGenerator intentionally empty — should load embedded default.
		},
	}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.SpecGenerator == "" {
		t.Error("SpecGenerator should be loaded from embedded default when config path is empty")
	}
}

func TestLoadPrompts_SpecGeneratorLoaded(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"impl.txt", "retry.txt", "rev.txt", "specgen.txt"} {
		content := "template"
		if name == "specgen.txt" {
			content = "specgen template"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Prompts: config.Prompts{
			Implementer:      filepath.Join(dir, "impl.txt"),
			ImplementerRetry: filepath.Join(dir, "retry.txt"),
			Reviewer:         filepath.Join(dir, "rev.txt"),
			SpecGenerator:    filepath.Join(dir, "specgen.txt"),
		},
	}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.SpecGenerator != "specgen template" {
		t.Errorf("SpecGenerator = %q, want %q", p.SpecGenerator, "specgen template")
	}
}

func TestLoadPrompts_SpecGeneratorMissingFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"impl.txt", "retry.txt", "rev.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("template"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.Config{
		Prompts: config.Prompts{
			Implementer:      filepath.Join(dir, "impl.txt"),
			ImplementerRetry: filepath.Join(dir, "retry.txt"),
			Reviewer:         filepath.Join(dir, "rev.txt"),
			SpecGenerator:    filepath.Join(dir, "nonexistent.txt"),
		},
	}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() should not error for missing spec_generator, got: %v", err)
	}
	if p.SpecGenerator != "" {
		t.Errorf("SpecGenerator = %q, want empty for missing file", p.SpecGenerator)
	}
}

func TestRenderPrompt_BranchExistsField(t *testing.T) {
	tmpl := "{{if .BranchExists}}existing{{else}}new{{end}}"
	data := PromptData{BranchExists: true}
	result, err := RenderPrompt(tmpl, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if result != "existing" {
		t.Errorf("RenderPrompt() = %q, want %q", result, "existing")
	}
}

func TestRenderPrompt_SubstitutesAllFields(t *testing.T) {
	tmpl := "Issue #{{.IssueNumber}} {{.IssueTitle}} repo={{.Repo}} PR={{.PRNumber}} build={{.BuildCommand}} test={{.TestCommand}} protected={{.ProtectedPaths}} scenario={{.ScenarioDir}} review={{.ReviewDir}} slug={{.Slug}}"

	data := PromptData{
		IssueNumber:    42,
		IssueTitle:     "Add feature",
		Repo:           "owner/repo",
		PRNumber:       7,
		BuildCommand:   "go build",
		TestCommand:    "go test ./...",
		ProtectedPaths: "CLAUDE.md, tests/scenarios/",
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
		Slug:           "add-feature",
	}

	result, err := RenderPrompt(tmpl, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}

	expected := "Issue #42 Add feature repo=owner/repo PR=7 build=go build test=go test ./... protected=CLAUDE.md, tests/scenarios/ scenario=tests/scenarios/ review=tests/review/ slug=add-feature"
	if result != expected {
		t.Errorf("RenderPrompt() = %q, want %q", result, expected)
	}
}

func TestRenderPrompt_InvalidTemplate(t *testing.T) {
	_, err := RenderPrompt("{{.Invalid", PromptData{})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Add feature", "add-feature"},
		{"Fix bug #42", "fix-bug-42"},
		{"Hello World!", "hello-world"},
		{"  spaces  ", "spaces"},
		{"UPPERCASE", "uppercase"},
		{"multi---dashes", "multi-dashes"},
		{"special@chars&here", "special-chars-here"},
	}

	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
