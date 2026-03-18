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
		SharedRules:    buildSharedRules("CLAUDE.md,tests/scenarios/", "tests/scenarios/"),
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

func TestLoadPrompts_ReconLoadedFromEmbedded(t *testing.T) {
	cfg := &config.Config{}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.Recon == "" {
		t.Error("Recon should be loaded from embedded default")
	}
}

func TestReconPrompt_RendersWithIssueTitleAndBody(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber: 42,
		IssueTitle:  "Add recon agent",
		IssueBody:   "The recon agent explores the codebase before implementation.",
	}
	rendered, err := RenderPrompt(p.Recon, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "Add recon agent") {
		t.Error("recon prompt should contain the issue title")
	}
	if !strings.Contains(rendered, "The recon agent explores the codebase before implementation.") {
		t.Error("recon prompt should contain the issue body")
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

func TestReviewerPrompt_IncludesArchitectureJSONBlock(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	jsonContent := `{"layers":[{"name":"foundation","may_depend_on":[]}]}`
	data := PromptData{
		IssueNumber:      1,
		Repo:             "owner/repo",
		PRNumber:         10,
		TestCommand:      "make test",
		BuildCommand:     "make build",
		ProtectedPaths:   "CLAUDE.md",
		ScenarioDir:      "tests/scenarios/",
		ReviewDir:        "tests/review/",
		ArchitectureJSON: jsonContent,
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, jsonContent) {
		t.Error("reviewer prompt should include ArchitectureJSON content when present")
	}
	if !strings.Contains(rendered, "may_depend_on") {
		t.Error("reviewer prompt should reference may_depend_on when ArchitectureJSON is present")
	}
}

func TestReviewerPrompt_EnforceArchitectureOn_IncludesBlockingDirective(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	jsonContent := `{"layers":[{"name":"foundation","may_depend_on":[]}]}`
	data := PromptData{
		IssueNumber:         1,
		Repo:                "owner/repo",
		PRNumber:            10,
		TestCommand:         "make test",
		BuildCommand:        "make build",
		ProtectedPaths:      "CLAUDE.md",
		ScenarioDir:         "tests/scenarios/",
		ReviewDir:           "tests/review/",
		ArchitectureJSON:    jsonContent,
		EnforceArchitecture: true,
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "MUST result in CHANGES_REQUESTED") {
		t.Error("reviewer prompt should include blocking directive when EnforceArchitecture is true")
	}
	if strings.Contains(rendered, "do not block approval") {
		t.Error("reviewer prompt should not include informational directive when EnforceArchitecture is true")
	}
}

func TestReviewerPrompt_EnforceArchitectureOff_IncludesInformationalDirective(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	jsonContent := `{"layers":[{"name":"foundation","may_depend_on":[]}]}`
	data := PromptData{
		IssueNumber:         1,
		Repo:                "owner/repo",
		PRNumber:            10,
		TestCommand:         "make test",
		BuildCommand:        "make build",
		ProtectedPaths:      "CLAUDE.md",
		ScenarioDir:         "tests/scenarios/",
		ReviewDir:           "tests/review/",
		ArchitectureJSON:    jsonContent,
		EnforceArchitecture: false,
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "do not block approval") {
		t.Error("reviewer prompt should include informational directive when EnforceArchitecture is false")
	}
	if strings.Contains(rendered, "MUST result in CHANGES_REQUESTED") {
		t.Error("reviewer prompt should not include blocking directive when EnforceArchitecture is false")
	}
}

func TestReviewerPrompt_NoArchitectureJSON_OmitsBothDirectives(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:         1,
		Repo:                "owner/repo",
		PRNumber:            10,
		TestCommand:         "make test",
		BuildCommand:        "make build",
		ProtectedPaths:      "CLAUDE.md",
		ScenarioDir:         "tests/scenarios/",
		ReviewDir:           "tests/review/",
		ArchitectureJSON:    "",
		EnforceArchitecture: true,
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "MUST result in CHANGES_REQUESTED") {
		t.Error("reviewer prompt should not include blocking directive when ArchitectureJSON is empty")
	}
	if strings.Contains(rendered, "do not block approval") {
		t.Error("reviewer prompt should not include informational directive when ArchitectureJSON is empty")
	}
}

func TestReviewerPrompt_OmitsArchitectureJSONBlockWhenEmpty(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:      1,
		Repo:             "owner/repo",
		PRNumber:         10,
		TestCommand:      "make test",
		BuildCommand:     "make build",
		ProtectedPaths:   "CLAUDE.md",
		ScenarioDir:      "tests/scenarios/",
		ReviewDir:        "tests/review/",
		ArchitectureJSON: "",
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "must_not_depend_on") {
		t.Error("reviewer prompt should not include must_not_depend_on when ArchitectureJSON is absent")
	}
}

func TestLoadPrompts_VerifyFixLoadedFromEmbedded(t *testing.T) {
	cfg := &config.Config{}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.VerifyFix == "" {
		t.Error("VerifyFix should be loaded from embedded default")
	}
}

func TestLoadPrompts_VerifyFixLoadedFromConfigPath(t *testing.T) {
	dir := t.TempDir()
	content := "verify fix custom template {{.VerifyErrors}}"
	path := filepath.Join(dir, "vf.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Prompts: config.Prompts{
			VerifyFix: path,
		},
	}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	if p.VerifyFix != content {
		t.Errorf("VerifyFix = %q, want %q", p.VerifyFix, content)
	}
}

func TestLoadPrompts_VerifyFixMissingFileIsEmpty(t *testing.T) {
	cfg := &config.Config{
		Prompts: config.Prompts{
			VerifyFix: "/nonexistent/verify_fix.txt",
		},
	}

	p, err := LoadPrompts(cfg)
	if err != nil {
		t.Fatalf("LoadPrompts() should not error for missing verify_fix, got: %v", err)
	}
	if p.VerifyFix != "" {
		t.Errorf("VerifyFix = %q, want empty for missing file", p.VerifyFix)
	}
}

func TestVerifyFixPrompt_RendersWithVerifyErrors(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	errText := "build: exit status 1\nundefined: foo"
	data := PromptData{
		IssueNumber:    5,
		IssueTitle:     "Add feature",
		Repo:           "owner/repo",
		PRNumber:       12,
		BuildCommand:   "go build ./...",
		TestCommand:    "go test ./...",
		ProtectedPaths: "CLAUDE.md",
		VerifyErrors:   errText,
	}
	rendered, err := RenderPrompt(p.VerifyFix, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, errText) {
		t.Error("verify_fix prompt should contain the verify error text")
	}
	if !strings.Contains(rendered, "12") {
		t.Error("verify_fix prompt should contain the PR number")
	}
}

func TestVerifyFixPrompt_RendersWithEmptyVerifyErrors(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    5,
		IssueTitle:     "Add feature",
		Repo:           "owner/repo",
		PRNumber:       12,
		BuildCommand:   "go build ./...",
		TestCommand:    "go test ./...",
		ProtectedPaths: "CLAUDE.md",
		VerifyErrors:   "",
	}
	_, err = RenderPrompt(p.VerifyFix, data)
	if err != nil {
		t.Errorf("RenderPrompt() with empty VerifyErrors should not error, got: %v", err)
	}
}

func TestRenderPrompt_VerifyErrorsField(t *testing.T) {
	tmpl := "errors: {{.VerifyErrors}}"
	errText := "lint: line 42: unused variable"
	data := PromptData{VerifyErrors: errText}
	result, err := RenderPrompt(tmpl, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if result != "errors: "+errText {
		t.Errorf("RenderPrompt() = %q, want %q", result, "errors: "+errText)
	}
}

func TestImplementerPrompt_BaseBranchSet_IncludesBaseFlag(t *testing.T) {
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
		BaseBranch:     "feature/foo",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "--base feature/foo") {
		t.Error("implementer prompt should contain '--base feature/foo' when BaseBranch is set")
	}
}

func TestImplementerPrompt_BaseBranchEmpty_NoBaseFlag(t *testing.T) {
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
		BaseBranch:     "",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "--base") {
		t.Error("implementer prompt should not contain '--base' when BaseBranch is empty")
	}
}

func TestImplementerPrompt_BaseBranchSet_CheckoutFromOrigin(t *testing.T) {
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
		BaseBranch:     "feature/foo",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "git checkout -b 1-test-issue origin/feature/foo") {
		t.Error("implementer prompt should contain 'git checkout -b <branch> origin/feature/foo' when BaseBranch is set")
	}
}

func TestImplementerPrompt_BaseBranchEmpty_CheckoutWithoutOrigin(t *testing.T) {
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
		BaseBranch:     "",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "git checkout -b 1-test-issue") {
		t.Error("implementer prompt should contain 'git checkout -b <branch>' when BaseBranch is empty")
	}
	if strings.Contains(rendered, "origin/") {
		t.Error("implementer prompt should not reference 'origin/' when BaseBranch is empty")
	}
}

func TestSpecGeneratorPrompt_BaseBranchSet_CheckoutFromOrigin(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		IssueTitle:     "Test Issue",
		Repo:           "owner/repo",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		Slug:           "test-issue",
		BaseBranch:     "feature/foo",
	}
	rendered, err := RenderPrompt(p.SpecGenerator, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "git checkout -b 1-test-issue origin/feature/foo") {
		t.Error("spec_generator prompt should contain 'git checkout -b <branch> origin/feature/foo' when BaseBranch is set")
	}
}

func TestSpecGeneratorPrompt_BaseBranchEmpty_CheckoutWithoutOrigin(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		IssueTitle:     "Test Issue",
		Repo:           "owner/repo",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		Slug:           "test-issue",
		BaseBranch:     "",
	}
	rendered, err := RenderPrompt(p.SpecGenerator, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "git checkout -b 1-test-issue") {
		t.Error("spec_generator prompt should contain 'git checkout -b <branch>' when BaseBranch is empty")
	}
	if strings.Contains(rendered, "origin/") {
		t.Error("spec_generator prompt should not reference 'origin/' when BaseBranch is empty")
	}
}

func TestImplementerPrompt_BaseBranchSet_NeverCommitToBaseBranch(t *testing.T) {
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
		BaseBranch:     "feature/foo",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "Never commit directly to feature/foo") {
		t.Error("implementer prompt should contain 'Never commit directly to feature/foo' when BaseBranch is set")
	}
}

func TestImplementerPrompt_BaseBranchEmpty_NeverCommitToMain(t *testing.T) {
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
		BaseBranch:     "",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "Never commit directly to main") {
		t.Error("implementer prompt should contain 'Never commit directly to main' when BaseBranch is empty")
	}
}

func TestSpecGeneratorPrompt_BaseBranchSet_NeverCommitToBaseBranch(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		IssueTitle:     "Test Issue",
		Repo:           "owner/repo",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		Slug:           "test-issue",
		BaseBranch:     "feature/foo",
	}
	rendered, err := RenderPrompt(p.SpecGenerator, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "Never commit directly to feature/foo") {
		t.Error("spec_generator prompt should contain 'Never commit directly to feature/foo' when BaseBranch is set")
	}
}

func TestSpecGeneratorPrompt_BaseBranchEmpty_NeverCommitToMain(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		IssueTitle:     "Test Issue",
		Repo:           "owner/repo",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		Slug:           "test-issue",
		BaseBranch:     "",
	}
	rendered, err := RenderPrompt(p.SpecGenerator, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "Never commit directly to main") {
		t.Error("spec_generator prompt should contain 'Never commit directly to main' when BaseBranch is empty")
	}
}

func TestImplementerPrompt_WithReconBrief_IncludesBrief(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	brief := "## Relevant files\n- internal/config/config.go: Prompts struct at line 206"
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
		ReconBrief:     brief,
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, brief) {
		t.Error("implementer prompt should contain the recon brief when ReconBrief is set")
	}
	if !strings.Contains(rendered, "recon brief") {
		t.Error("implementer prompt should contain 'recon brief' heading when ReconBrief is set")
	}
}

func TestImplementerPrompt_WithoutReconBrief_OmitsReconSection(t *testing.T) {
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
		ReconBrief:     "",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "recon brief") {
		t.Error("implementer prompt should not contain 'recon brief' when ReconBrief is empty")
	}
}

func TestRenderPrompt_HandoffContextRendered(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	handoff := "## Implementation Notes\nPrevious attempt failed due to X."
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
		HandoffContext: handoff,
	}
	rendered, err := RenderPrompt(p.ImplementerRetry, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "fresh session") {
		t.Error("implementer_retry prompt should include fresh session preamble when HandoffContext is set")
	}
	if !strings.Contains(rendered, handoff) {
		t.Errorf("implementer_retry prompt should include handoff content, got: %s", rendered)
	}
}

func TestRenderPrompt_HandoffContextNotRenderedWhenEmpty(t *testing.T) {
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
		HandoffContext: "",
	}
	rendered, err := RenderPrompt(p.ImplementerRetry, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "fresh session") {
		t.Error("implementer_retry prompt should not include fresh session preamble when HandoffContext is empty")
	}
}

func TestImplementerPrompt_RendersSharedRulesAndBranchAndGeneratedPaths(t *testing.T) {
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
		GeneratedPaths: "gen/",
		Slug:           "test-issue",
		SharedRules:    buildSharedRules("CLAUDE.md", "tests/scenarios/"),
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	// Shared rules present.
	if !strings.Contains(rendered, "Do NOT modify any protected paths: CLAUDE.md") {
		t.Error("implementer prompt should contain shared protected paths rule")
	}
	if !strings.Contains(rendered, "Do NOT modify files in tests/scenarios/") {
		t.Error("implementer prompt should contain shared scenario dir rule (via SharedRules)")
	}
	// Agent-specific rules present.
	if !strings.Contains(rendered, "git checkout") {
		t.Error("implementer prompt should contain branch creation rule")
	}
	if !strings.Contains(rendered, "Do NOT modify files in tests/review/") {
		t.Error("implementer prompt should contain agent-specific ReviewDir rule")
	}
	if !strings.Contains(rendered, "Do NOT edit generated files directly") {
		t.Error("implementer prompt should contain agent-specific generated paths rule")
	}
}

func TestSpecGeneratorPrompt_KeepsNarrowScenarioDirRule(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:    1,
		IssueTitle:     "Test Issue",
		Repo:           "owner/repo",
		ProtectedPaths: "CLAUDE.md",
		ScenarioDir:    "tests/scenarios/",
		Slug:           "test-issue",
		// SharedRules excludes ScenarioDir for spec_generator (narrow wording kept agent-side).
		SharedRules: buildSharedRules("CLAUDE.md", ""),
	}
	rendered, err := RenderPrompt(p.SpecGenerator, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	// Shared protected paths rule present.
	if !strings.Contains(rendered, "Do NOT modify any protected paths: CLAUDE.md") {
		t.Error("spec_generator prompt should contain shared protected paths rule")
	}
	// Narrow agent-specific ScenarioDir rule with "existing files" wording present.
	if !strings.Contains(rendered, "Do NOT modify existing files in tests/scenarios/") {
		t.Error("spec_generator prompt should retain the narrower 'existing files' ScenarioDir rule")
	}
	// Broad ScenarioDir rule must NOT appear — it would contradict spec_generator's task of creating files.
	if strings.Contains(rendered, "Do NOT modify files in tests/scenarios/") {
		t.Error("spec_generator prompt must not contain the broad 'Do NOT modify files in' ScenarioDir rule from SharedRules")
	}
}

func TestImplementerPrompt_ComposeServicesPresent(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:     1,
		IssueTitle:      "Test Issue",
		Repo:            "owner/repo",
		BuildCommand:    "make build",
		TestCommand:     "make test",
		ProtectedPaths:  "CLAUDE.md",
		ScenarioDir:     "tests/scenarios/",
		ReviewDir:       "tests/review/",
		Slug:            "test-issue",
		ComposeServices: "- postgres: PostgreSQL on localhost:5432\n- redis: Redis on localhost:6379\n",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "postgres") {
		t.Error("implementer prompt should contain service name 'postgres' when ComposeServices is set")
	}
	if !strings.Contains(rendered, "PostgreSQL on localhost:5432") {
		t.Error("implementer prompt should contain service description when ComposeServices is set")
	}
	if !strings.Contains(rendered, "Docker Compose services") {
		t.Error("implementer prompt should contain 'Docker Compose services' heading when ComposeServices is set")
	}
}

func TestImplementerPrompt_ComposeServicesAbsent(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:     1,
		IssueTitle:      "Test Issue",
		Repo:            "owner/repo",
		BuildCommand:    "make build",
		TestCommand:     "make test",
		ProtectedPaths:  "CLAUDE.md",
		ScenarioDir:     "tests/scenarios/",
		ReviewDir:       "tests/review/",
		Slug:            "test-issue",
		ComposeServices: "",
	}
	rendered, err := RenderPrompt(p.Implementer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "Docker Compose services") {
		t.Error("implementer prompt should not contain 'Docker Compose services' heading when ComposeServices is empty")
	}
}

func TestReviewerPrompt_ComposeServicesPresent(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:     1,
		IssueTitle:      "Test Issue",
		Repo:            "owner/repo",
		PRNumber:        10,
		BuildCommand:    "make build",
		TestCommand:     "make test",
		ProtectedPaths:  "CLAUDE.md",
		ScenarioDir:     "tests/scenarios/",
		ReviewDir:       "tests/review/",
		ComposeServices: "- traefik: Local reverse proxy — API at http://api.localhost\n",
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if !strings.Contains(rendered, "traefik") {
		t.Error("reviewer prompt should contain service name 'traefik' when ComposeServices is set")
	}
	if !strings.Contains(rendered, "Docker Compose services") {
		t.Error("reviewer prompt should contain 'Docker Compose services' heading when ComposeServices is set")
	}
}

func TestReviewerPrompt_ComposeServicesAbsent(t *testing.T) {
	p, err := LoadPrompts(&config.Config{})
	if err != nil {
		t.Fatalf("LoadPrompts() error = %v", err)
	}
	data := PromptData{
		IssueNumber:     1,
		IssueTitle:      "Test Issue",
		Repo:            "owner/repo",
		PRNumber:        10,
		BuildCommand:    "make build",
		TestCommand:     "make test",
		ProtectedPaths:  "CLAUDE.md",
		ScenarioDir:     "tests/scenarios/",
		ReviewDir:       "tests/review/",
		ComposeServices: "",
	}
	rendered, err := RenderPrompt(p.Reviewer, data)
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if strings.Contains(rendered, "Docker Compose services") {
		t.Error("reviewer prompt should not contain 'Docker Compose services' heading when ComposeServices is empty")
	}
}
