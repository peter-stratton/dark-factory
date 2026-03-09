package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCommandRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "new" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new command not registered on rootCmd")
	}
}

// stubGitRunner replaces gitRunner with a stub that creates the .git directory
// at the path given by the -C argument. Returns a cleanup function.
func stubGitRunner(t *testing.T) func() {
	t.Helper()
	orig := gitRunner
	gitRunner = func(name string, args ...string) ([]byte, error) {
		for i, arg := range args {
			if arg == "-C" && i+1 < len(args) {
				dir := args[i+1]
				if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Errorf("stub gitRunner: creating .git: %v", err)
				}
				return nil, nil
			}
		}
		return nil, nil
	}
	return func() { gitRunner = orig }
}

func runNewCmd(t *testing.T, args []string) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	newCmd.SetOut(buf)
	defer newCmd.SetOut(nil)

	if err := newCmd.RunE(newCmd, args); err != nil {
		t.Fatalf("new command failed: %v", err)
	}
	return buf
}

func TestNewCreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	restore := stubGitRunner(t)
	defer restore()

	runNewCmd(t, []string{"testproject"})

	for _, path := range []string{
		"testproject/CLAUDE.md",
		"testproject/.gitignore",
		"testproject/.git",
		"testproject/godark.yaml",
		"testproject/docs/architecture.md",
		"testproject/docs/architecture.json",
		"testproject/docs/conventions.md",
		"testproject/docs/ROADMAP.md",
		"testproject/prompts/implementer.txt",
		"testproject/prompts/implementer_retry.txt",
		"testproject/prompts/reviewer.txt",
		"testproject/.claude/godark.md",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}

	for _, d := range []string{"testproject/docs/planning", "testproject/tests/scenarios"} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}
}

func TestNewClaudeMDHeaders(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	restore := stubGitRunner(t)
	defer restore()

	runNewCmd(t, []string{"testproject"})

	data, err := os.ReadFile("testproject/CLAUDE.md")
	if err != nil {
		t.Fatalf("reading testproject/CLAUDE.md: %v", err)
	}
	content := string(data)

	for _, header := range []string{
		"# CLAUDE.md",
		"## Where to look",
	} {
		if !strings.Contains(content, header) {
			t.Errorf("CLAUDE.md missing expected header %q", header)
		}
	}
}

func TestNewGitInitialized(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	restore := stubGitRunner(t)
	defer restore()

	runNewCmd(t, []string{"testproject"})

	info, err := os.Stat("testproject/.git")
	if err != nil {
		t.Fatalf("testproject/.git not found: %v", err)
	}
	if !info.IsDir() {
		t.Error("testproject/.git should be a directory")
	}
}

func TestNewRepoFlag(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	restore := stubGitRunner(t)
	defer restore()

	if err := newCmd.Flags().Set("repo", "owner/repo"); err != nil {
		t.Fatalf("setting --repo flag: %v", err)
	}
	defer newCmd.Flags().Set("repo", "") //nolint:errcheck

	runNewCmd(t, []string{"testproject"})

	data, err := os.ReadFile("testproject/godark.yaml")
	if err != nil {
		t.Fatalf("reading testproject/godark.yaml: %v", err)
	}

	if !strings.Contains(string(data), `repo: "owner/repo"`) {
		t.Errorf(`godark.yaml missing repo: "owner/repo"; got:\n%s`, data)
	}
}

func TestNewExistingDirError(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	if err := os.MkdirAll("testproject", 0o755); err != nil {
		t.Fatal(err)
	}

	err := newCmd.RunE(newCmd, []string{"testproject"})
	if err == nil {
		t.Fatal("expected error when directory already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error message should mention 'already exists', got: %v", err)
	}
}

func TestNewNoArgError(t *testing.T) {
	err := newCmd.Args(newCmd, []string{})
	if err == nil {
		t.Error("expected error when no project name argument given")
	}
}
