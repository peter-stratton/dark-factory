package agent

import (
	"os"
	"path/filepath"
	"strings"
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

func TestImplementerPrompt_NoGoSpecificNaming(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		IssueTitle:     "Test Issue",
		Repo:           "owner/repo",
		BuildCommand:   "make build",
		TestCommand:    "make test",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
		Slug:           "test-issue",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "foo_test.go") {
		t.Error("implementer prompt should not contain 'foo_test.go'")
	}
	if strings.Contains(rendered, "foo.go") {
		t.Error("implementer prompt should not contain 'foo.go'")
	}
}

func TestReviewerPrompt_NoGoTestReferences(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		IssueTitle:     "Test Issue",
		Repo:           "owner/repo",
		PRNumber:       10,
		BuildCommand:   "make build",
		TestCommand:    "make test",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "go test") {
		t.Error("reviewer prompt should not contain hardcoded 'go test'")
	}
	if strings.Contains(rendered, "Generate Go integration") {
		t.Error("reviewer prompt should not contain 'Generate Go integration'")
	}
}

func TestReviewerPrompt_StillReferencesReviewDir(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		Repo:           "owner/repo",
		PRNumber:       10,
		TestCommand:    "make test",
		BuildCommand:   "make build",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "tests/review/") {
		t.Error("reviewer prompt should still reference the ReviewDir path")
	}
}

func TestReviewerPrompt_ExpandsProtectedPaths(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		Repo:           "owner/repo",
		PRNumber:       10,
		TestCommand:    "make test",
		BuildCommand:   "make build",
		ProtectedPaths: "CLAUDE.md,tests/scenarios/",
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "CLAUDE.md,tests/scenarios/") {
		t.Error("reviewer prompt must contain ProtectedPaths \"CLAUDE.md,tests/scenarios/\"")
	}
}

func TestRetryPrompt_ExpandsTestCommandNoGoReferences(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		PRNumber:       10,
		Repo:           "owner/repo",
		TestCommand:    "pytest -v",
		BuildCommand:   "pip install -e .",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
	}
	rendered, err := RenderPrompt(p.ImplementerRetry, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "pytest -v") {
		t.Error("retry prompt should expand TestCommand")
	}
	if strings.Contains(rendered, "foo_test.go") || strings.Contains(rendered, "foo.go") {
		t.Error("retry prompt should not contain Go-specific file naming")
	}
}

func TestPromptTemplateVarsPreserved(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    42,
		PRNumber:       7,
		Repo:           "owner/repo",
		BuildCommand:   "make build",
		TestCommand:    "make test",
		ScenarioDir:    "tests/scenarios/",
		ReviewDir:      "tests/review/",
		ProtectedPaths: "CLAUDE.md",
		Slug:           "test-feature",
	}

	// Implementer: BuildCommand and TestCommand expand correctly.
	implRendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("implementer RenderPrompt() error = %v", err)
	}
	if !strings.Contains(implRendered, "make build") {
		t.Error("implementer prompt should expand BuildCommand")
	}
	if !strings.Contains(implRendered, "make test") {
		t.Error("implementer prompt should expand TestCommand")
	}

	// Reviewer: ReviewDir and TestCommand expand correctly.
	revRendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("reviewer RenderPrompt() error = %v", err)
	}
	if !strings.Contains(revRendered, "tests/review/") {
		t.Error("reviewer prompt should expand ReviewDir")
	}
	if !strings.Contains(revRendered, "make test") {
		t.Error("reviewer prompt should expand TestCommand")
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
