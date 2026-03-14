package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/skills"
)

func TestCreateMilestoneSkillStep1ReferencesArchitectureJSON(t *testing.T) {
	content := readSkill(t, "godark-create-milestone")
	if !strings.Contains(content, "docs/architecture.json") {
		t.Error("step 1 does not reference docs/architecture.json")
	}
}

func TestCreateMilestoneSkillStep1ReferencesConventionsMD(t *testing.T) {
	content := readSkill(t, "godark-create-milestone")
	if !strings.Contains(content, "docs/conventions.md") {
		t.Error("step 1 does not reference docs/conventions.md")
	}
}

func TestCreateMilestoneSkillMentionsDefineArchitectureSkill(t *testing.T) {
	content := readSkill(t, "godark-create-milestone")
	if !strings.Contains(content, "/godark-define-architecture") {
		t.Error("skill does not mention /godark-define-architecture when architecture is missing")
	}
}

func TestCreateMilestoneSkillStep2AskAboutArchitectureLayers(t *testing.T) {
	content := readSkill(t, "godark-create-milestone")
	if !strings.Contains(content, "Architecture layers") && !strings.Contains(content, "architecture layers") {
		t.Error("step 2 does not ask about architecture layers for structural decisions")
	}
}

func TestCreateMilestoneSkillEmbedded(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-create-milestone/SKILL.md")
	if err != nil {
		t.Errorf("godark-create-milestone not embedded: %v", err)
	}
}
