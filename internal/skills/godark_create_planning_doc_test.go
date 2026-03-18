package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/skills"
)

func TestCreatePlanningDocSkillStep3ReferencesArchitectureJSON(t *testing.T) {
	content := readSkill(t, "godark-create-planning-doc")
	if !strings.Contains(content, "docs/architecture.json") {
		t.Error("step 3 does not reference docs/architecture.json")
	}
}

func TestCreatePlanningDocSkillStep3ReferencesConventionsMD(t *testing.T) {
	content := readSkill(t, "godark-create-planning-doc")
	if !strings.Contains(content, "docs/conventions.md") {
		t.Error("step 3 does not reference docs/conventions.md")
	}
}

func TestCreatePlanningDocSkillStep4PromptsArchitectureUpdate(t *testing.T) {
	content := readSkill(t, "godark-create-planning-doc")
	if !strings.Contains(content, "docs/architecture.json") {
		t.Error("step 4 does not mention updating docs/architecture.json when layers don't fit")
	}
	if !strings.Contains(content, "don't fit") && !strings.Contains(content, "don\u2019t fit") {
		t.Error("step 4 does not describe the case where packages don't fit current layers")
	}
}

func TestCreatePlanningDocSkillStep4MentionsDefineArchitectureSkill(t *testing.T) {
	content := readSkill(t, "godark-create-planning-doc")
	if !strings.Contains(content, "/godark-define-architecture") {
		t.Error("step 4 does not suggest /godark-define-architecture for revising layer definitions")
	}
}

func TestCreatePlanningDocSkillEmbedded(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-create-planning-doc/SKILL.md")
	if err != nil {
		t.Errorf("godark-create-planning-doc not embedded: %v", err)
	}
}

func TestEmbedDirectiveIncludesDefineArchitecture(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-define-architecture/SKILL.md")
	if err != nil {
		t.Errorf("godark-define-architecture not included in embed directive: %v", err)
	}
}

func TestEmbedDirectiveIncludesDefineConventions(t *testing.T) {
	_, err := fs.ReadFile(skills.SkillFiles, "godark-define-conventions/SKILL.md")
	if err != nil {
		t.Errorf("godark-define-conventions not included in embed directive: %v", err)
	}
}
