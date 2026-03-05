package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/skills"
)

func readDefineArchitectureSkill(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(skills.SkillFiles, "godark-define-architecture/SKILL.md")
	if err != nil {
		t.Fatalf("reading godark-define-architecture/SKILL.md: %v", err)
	}
	return string(data)
}

// parseFrontmatter extracts the YAML block between the first pair of --- delimiters.
func parseFrontmatter(content string) string {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func TestDefineArchitectureSkillFrontmatterName(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "name:") {
		t.Error("frontmatter missing 'name' field")
	}
}

func TestDefineArchitectureSkillFrontmatterDescription(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "description:") {
		t.Error("frontmatter missing 'description' field")
	}
}

func TestDefineArchitectureSkillFrontmatterArgumentHint(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "argument-hint:") {
		t.Error("frontmatter missing 'argument-hint' field")
	}
}

func TestDefineArchitectureSkillFrontmatterDisableModelInvocation(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	fm := parseFrontmatter(content)
	if !strings.Contains(fm, "disable-model-invocation: true") {
		t.Error("frontmatter missing 'disable-model-invocation: true'")
	}
}

func TestDefineArchitectureSkillStepsDescribePackageScanning(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "package") && !strings.Contains(content, "module") {
		t.Error("steps do not describe scanning for package or module directories")
	}
}

func TestDefineArchitectureSkillStepsDescribeImportRelationships(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "import") {
		t.Error("steps do not describe identifying import relationships")
	}
}

func TestDefineArchitectureSkillStepsDescribeLanguageFrameworkQuestions(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "language") && !strings.Contains(content, "framework") {
		t.Error("steps do not describe asking about language and framework for new projects")
	}
}

func TestDefineArchitectureSkillStepsDescribeIdiomaticLayers(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "idiomatic") {
		t.Error("steps do not describe proposing idiomatic layers for new projects")
	}
}

func TestDefineArchitectureSkillStepsIncludeVetArchitecture(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "godark vet architecture") {
		t.Error("steps do not include running 'godark vet architecture'")
	}
}

func TestDefineArchitectureSkillStepsSuggestCreateRoadmapForDiscrepancies(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "/godark-create-roadmap") {
		t.Error("steps do not suggest running /godark-create-roadmap for discrepancies")
	}
}

func TestDefineArchitectureSkillStepsWriteArchitectureJSON(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "docs/architecture.json") {
		t.Error("steps do not include writing docs/architecture.json")
	}
}

func TestDefineArchitectureSkillStepsWriteArchitectureMD(t *testing.T) {
	content := readDefineArchitectureSkill(t)
	if !strings.Contains(content, "docs/architecture.md") {
		t.Error("steps do not include writing or updating docs/architecture.md")
	}
}
