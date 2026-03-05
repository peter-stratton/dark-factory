package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/skills"
)

func readCreateRoadmapSkill(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(skills.SkillFiles, "godark-create-roadmap/SKILL.md")
	if err != nil {
		t.Fatalf("reading godark-create-roadmap/SKILL.md: %v", err)
	}
	return string(data)
}

func TestCreateRoadmapSkillStep1ReferencesArchitectureJSON(t *testing.T) {
	content := readCreateRoadmapSkill(t)
	if !strings.Contains(content, "docs/architecture.json") {
		t.Error("step 1 does not reference docs/architecture.json")
	}
}

func TestCreateRoadmapSkillStep1ReferencesConventionsMD(t *testing.T) {
	content := readCreateRoadmapSkill(t)
	if !strings.Contains(content, "docs/conventions.md") {
		t.Error("step 1 does not reference docs/conventions.md")
	}
}

func TestCreateRoadmapSkillMentionsDefineArchitectureSkill(t *testing.T) {
	content := readCreateRoadmapSkill(t)
	if !strings.Contains(content, "/godark-define-architecture") {
		t.Error("skill does not mention /godark-define-architecture when architecture is missing")
	}
}

func TestCreateRoadmapSkillStep2AskAboutArchitectureLayers(t *testing.T) {
	content := readCreateRoadmapSkill(t)
	if !strings.Contains(content, "Architecture layers") && !strings.Contains(content, "architecture layers") {
		t.Error("step 2 does not ask about architecture layers for structural decisions")
	}
}

func TestCreateRoadmapSkillEmbedded(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-create-roadmap/SKILL.md")
	if err != nil {
		t.Errorf("godark-create-roadmap not embedded: %v", err)
	}
}
