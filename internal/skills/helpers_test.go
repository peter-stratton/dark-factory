package skills_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/skills"
)

func readSkill(t *testing.T, name string) string {
	t.Helper()
	path := name + "/SKILL.md"
	data, err := fs.ReadFile(skills.SkillFiles, path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
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
