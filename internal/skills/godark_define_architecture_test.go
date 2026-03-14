package skills_test

import (
	"strings"
	"testing"
)

func TestDefineArchitectureSkillFrontmatterName(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "name:") {
		t.Error("frontmatter missing 'name' field")
	}
}

func TestDefineArchitectureSkillFrontmatterDescription(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "description:") {
		t.Error("frontmatter missing 'description' field")
	}
}

func TestDefineArchitectureSkillFrontmatterArgumentHint(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "argument-hint:") {
		t.Error("frontmatter missing 'argument-hint' field")
	}
}

func TestDefineArchitectureSkillFrontmatterDisableModelInvocation(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "disable-model-invocation: true") {
		t.Error("frontmatter missing 'disable-model-invocation: true'")
	}
}

func TestDefineArchitectureSkillStepsDescribePackageScanning(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "package") && !strings.Contains(content, "module") {
		t.Error("steps do not describe scanning for package or module directories")
	}
}

func TestDefineArchitectureSkillStepsDescribeImportRelationships(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "import") {
		t.Error("steps do not describe identifying import relationships")
	}
}

func TestDefineArchitectureSkillStepsDescribeLanguageFrameworkQuestions(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "language") && !strings.Contains(content, "framework") {
		t.Error("steps do not describe asking about language and framework for new projects")
	}
}

func TestDefineArchitectureSkillStepsDescribeIdiomaticLayers(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "idiomatic") {
		t.Error("steps do not describe proposing idiomatic layers for new projects")
	}
}

func TestDefineArchitectureSkillStepsIncludeVetArchitecture(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "godark vet architecture") {
		t.Error("steps do not include running 'godark vet architecture'")
	}
}

func TestDefineArchitectureSkillStepsSuggestCreateMilestoneForDiscrepancies(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "/godark-create-milestone") {
		t.Error("steps do not suggest running /godark-create-milestone for discrepancies")
	}
}

func TestDefineArchitectureSkillStepsWriteArchitectureJSON(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "docs/architecture.json") {
		t.Error("steps do not include writing docs/architecture.json")
	}
}

func TestDefineArchitectureSkillStepsWriteArchitectureMD(t *testing.T) {
	content := readSkill(t, "godark-define-architecture")
	if !strings.Contains(content, "docs/architecture.md") {
		t.Error("steps do not include writing or updating docs/architecture.md")
	}
}
