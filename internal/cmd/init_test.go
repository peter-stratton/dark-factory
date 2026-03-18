package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommandRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("init command not registered on rootCmd")
	}
}

func runInit(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	initCmd.SetOut(buf)
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init command failed: %v", err)
	}
	return buf
}

func TestInitWritesSkillFiles(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	skillPath := filepath.Join(".claude", "skills", "godark-create-scenarios", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("skill file not created: %v", err)
	}

	if !strings.Contains(string(data), "name: godark-create-scenarios") {
		t.Error("SKILL.md missing expected frontmatter")
	}
}

func TestInitWritesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	data, err := os.ReadFile("godark.yaml")
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	if !strings.Contains(string(data), "repo:") {
		t.Error("godark.yaml missing repo field")
	}
}

func TestInitSkipsExistingConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	existing := "repo: my-org/my-repo\n"
	os.WriteFile("godark.yaml", []byte(existing), 0o644)

	buf := runInit(t)

	data, _ := os.ReadFile("godark.yaml")
	if string(data) != existing {
		t.Error("existing godark.yaml was overwritten")
	}

	if !strings.Contains(buf.String(), "skipped godark.yaml") {
		t.Error("expected skip message for existing config")
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)
	runInit(t)

	skillPath := filepath.Join(".claude", "skills", "godark-create-scenarios", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Error("skill file missing after second init")
	}
}

func TestInitWritesConfigureProjectSkill(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	skillPath := filepath.Join(".claude", "skills", "godark-configure-project", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("godark-configure-project skill not created: %v", err)
	}

	if !strings.Contains(string(data), "name: godark-configure-project") {
		t.Error("SKILL.md missing expected frontmatter name")
	}
}

func TestInitConfigureProjectSkillIdempotent(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)
	runInit(t)

	skillPath := filepath.Join(".claude", "skills", "godark-configure-project", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Error("godark-configure-project skill file missing after second init")
	}
}

func TestInitWritesGodarkDoc(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	docPath := filepath.Join(".claude", "godark.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf(".claude/godark.md not created: %v", err)
	}

	if !strings.Contains(string(data), "godark.yaml") {
		t.Error("godark.md missing expected config reference content")
	}
}

func TestInitGodarkDocAlwaysOverwritten(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	// Write custom content to simulate a stale version.
	docPath := filepath.Join(".claude", "godark.md")
	os.WriteFile(docPath, []byte("stale"), 0o644)

	runInit(t)

	data, _ := os.ReadFile(docPath)
	if string(data) == "stale" {
		t.Error(".claude/godark.md should be overwritten on re-init")
	}
}

func TestInitWritesHarnessDocs(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	for _, path := range []string{
		"docs/architecture.md",
		"docs/architecture.json",
		"docs/conventions.md",
		"docs/ROADMAP.md",
		"prompts/implementer.txt",
		"prompts/implementer_retry.txt",
		"prompts/reviewer.txt",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		} else if len(data) == 0 {
			t.Errorf("expected %s to have non-empty content", path)
		}
	}

	for _, d := range []string{"docs/planning", "tests/scenarios"} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}

	if _, err := os.Stat("CLAUDE.md"); !os.IsNotExist(err) {
		t.Error("CLAUDE.md should not be written without --reset-claude-md")
	}
}

func TestInitHarnessDocsIdempotent(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	data1, err := os.ReadFile("docs/architecture.md")
	if err != nil {
		t.Fatalf("reading docs/architecture.md after first init: %v", err)
	}

	buf := runInit(t)

	data2, _ := os.ReadFile("docs/architecture.md")
	if string(data1) != string(data2) {
		t.Error("docs/architecture.md was overwritten on second init")
	}

	if !strings.Contains(buf.String(), "skipped docs/architecture.md (already exists)") {
		t.Error("expected skip message for docs/architecture.md on second init")
	}
}

func TestInitPartialState(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.MkdirAll("docs", 0o755)
	existing := "pre-existing content"
	os.WriteFile("docs/architecture.md", []byte(existing), 0o644)

	buf := runInit(t)

	data, _ := os.ReadFile("docs/architecture.md")
	if string(data) != existing {
		t.Error("docs/architecture.md was overwritten")
	}

	if _, err := os.Stat("docs/conventions.md"); err != nil {
		t.Error("docs/conventions.md should have been created")
	}

	if !strings.Contains(buf.String(), "skipped docs/architecture.md (already exists)") {
		t.Error("expected skip message for pre-existing docs/architecture.md")
	}

	if !strings.Contains(buf.String(), "wrote docs/conventions.md") {
		t.Error("expected write message for docs/conventions.md")
	}
}

func TestInitPromptsAlwaysOverwritten(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	// Write stale content to simulate an older scaffolded version.
	stale := "stale prompt content"
	os.WriteFile("prompts/implementer.txt", []byte(stale), 0o644)

	buf := runInit(t)

	data, _ := os.ReadFile("prompts/implementer.txt")
	if string(data) == stale {
		t.Error("prompts/implementer.txt should be overwritten on re-init")
	}

	if !strings.Contains(buf.String(), "wrote prompts/implementer.txt") {
		t.Error("expected write message for prompts/implementer.txt on re-init")
	}
}

func TestInitResetClaudeMD(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := initCmd.Flags().Set("reset-claude-md", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	defer initCmd.Flags().Set("reset-claude-md", "false") //nolint:errcheck

	buf := runInit(t)

	if _, err := os.Stat("CLAUDE.md"); err != nil {
		t.Error("CLAUDE.md should be written with --reset-claude-md")
	}

	if !strings.Contains(buf.String(), "wrote CLAUDE.md") {
		t.Error("expected write message for CLAUDE.md")
	}
}

func TestInitResetClaudeMDReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	existing := "# Custom CLAUDE.md\n"
	os.WriteFile("CLAUDE.md", []byte(existing), 0o644)

	errBuf := new(bytes.Buffer)
	initCmd.SetErr(errBuf)
	defer initCmd.SetErr(nil) // reset to default os.Stderr

	if err := initCmd.Flags().Set("reset-claude-md", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	defer initCmd.Flags().Set("reset-claude-md", "false") //nolint:errcheck

	runInit(t)

	data, _ := os.ReadFile("CLAUDE.md")
	if string(data) == existing {
		t.Error("CLAUDE.md was not replaced by --reset-claude-md")
	}

	if !strings.Contains(errBuf.String(), "replacing existing CLAUDE.md with harness template") {
		t.Error("expected warning about replacing existing CLAUDE.md on stderr")
	}
}

func TestInitGuidanceWithoutResetFlag(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	buf := runInit(t)

	if !strings.Contains(buf.String(), "hint: review your CLAUDE.md") {
		t.Error("expected CLAUDE.md guidance hint in output when --reset-claude-md not used")
	}
}

func TestInitConfigIncludesFormatCommand(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	data, err := os.ReadFile("godark.yaml")
	if err != nil {
		t.Fatalf("godark.yaml not created: %v", err)
	}

	if !strings.Contains(string(data), "format_command") {
		t.Error("godark.yaml missing format_command field")
	}
}

func TestInitConfigIncludesLintCommand(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	runInit(t)

	data, err := os.ReadFile("godark.yaml")
	if err != nil {
		t.Fatalf("godark.yaml not created: %v", err)
	}

	if !strings.Contains(string(data), "lint_command") {
		t.Error("godark.yaml missing lint_command field")
	}
}

func TestInitGoProjectWritesActiveCommands(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Simulate a Go project by writing a minimal go.mod.
	if err := os.WriteFile("go.mod", []byte("module example.com/test\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	runInit(t)

	data, err := os.ReadFile("godark.yaml")
	if err != nil {
		t.Fatalf("godark.yaml not created: %v", err)
	}

	content := string(data)

	// Commands should be active (uncommented), not just present as examples.
	for _, want := range []string{
		"build_command: \"go build ./...\"",
		"test_command: \"go test ./...\"",
		"format_command: \"gofmt -l -d .\"",
		"lint_command: \"go vet ./...\"",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("godark.yaml missing active command line: %s", want)
		}
	}

	// Should include the auto-detected comment.
	if !strings.Contains(content, "auto-detected") {
		t.Error("godark.yaml should note commands were auto-detected")
	}
}

func TestInitRepoFlagUpdatesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Write a config with an empty repo field.
	os.WriteFile("godark.yaml", []byte("repo: \"\"\nauto_merge:\n  feature: none\n"), 0o644)

	if err := initCmd.Flags().Set("repo", "owner/repo-name"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	defer initCmd.Flags().Set("repo", "") //nolint:errcheck

	buf := runInit(t)

	data, _ := os.ReadFile("godark.yaml")
	if !strings.Contains(string(data), `repo: "owner/repo-name"`) {
		t.Errorf("expected repo field to be updated, got: %s", data)
	}

	if !strings.Contains(buf.String(), "updated repo in godark.yaml") {
		t.Error("expected update message for repo field")
	}
}

func TestInitRepoFlagSkipsAlreadySetConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Write a config with repo already set.
	existing := "repo: \"owner/existing-repo\"\nauto_merge:\n  feature: none\n"
	os.WriteFile("godark.yaml", []byte(existing), 0o644)

	if err := initCmd.Flags().Set("repo", "owner/new-repo"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	defer initCmd.Flags().Set("repo", "") //nolint:errcheck

	buf := runInit(t)

	data, _ := os.ReadFile("godark.yaml")
	if string(data) != existing {
		t.Error("config with existing repo should not be modified")
	}

	if !strings.Contains(buf.String(), "skipped godark.yaml (repo already set)") {
		t.Error("expected skip message for already-set repo")
	}
}

func TestInitRepoFlagPopulatesNewConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := initCmd.Flags().Set("repo", "owner/new-project"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	defer initCmd.Flags().Set("repo", "") //nolint:errcheck

	runInit(t)

	data, err := os.ReadFile("godark.yaml")
	if err != nil {
		t.Fatalf("godark.yaml not created: %v", err)
	}

	if !strings.Contains(string(data), `repo: "owner/new-project"`) {
		t.Errorf("expected repo field to be populated, got: %s", data)
	}
}

func TestInitNonGoProjectUsesGenericExamples(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// No go.mod — non-Go project uses generic (empty-string) examples.
	runInit(t)

	data, err := os.ReadFile("godark.yaml")
	if err != nil {
		t.Fatalf("godark.yaml not created: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "golangci-lint") {
		t.Error("non-Go project should not suggest golangci-lint")
	}
	if strings.Contains(content, "gofmt") {
		t.Error("non-Go project should not suggest gofmt")
	}
}
