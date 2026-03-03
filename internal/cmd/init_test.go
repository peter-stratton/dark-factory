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

	skillPath := filepath.Join(".claude", "skills", "godark-create-scenario", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("skill file not created: %v", err)
	}

	if !strings.Contains(string(data), "name: godark-create-scenario") {
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

	skillPath := filepath.Join(".claude", "skills", "godark-create-scenario", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Error("skill file missing after second init")
	}
}
