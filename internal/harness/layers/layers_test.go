package layers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_ValidJSON(t *testing.T) {
	input := `{
		"layers": [
			{"name": "types",        "paths": ["internal/types/"],        "may_depend_on": []},
			{"name": "config",       "paths": ["internal/config/"],       "may_depend_on": ["types"]},
			{"name": "deps",         "paths": ["internal/deps/"],         "may_depend_on": ["types", "config"]},
			{"name": "github",       "paths": ["internal/github/"],       "may_depend_on": ["types"]},
			{"name": "orchestrator", "paths": ["internal/orchestrator/"], "may_depend_on": ["types", "config", "deps", "github"]}
		]
	}`

	def, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def.Layers) != 5 {
		t.Fatalf("want 5 layers, got %d", len(def.Layers))
	}
	if def.Layers[0].Name != "types" {
		t.Errorf("want first layer name %q, got %q", "types", def.Layers[0].Name)
	}
	if len(def.Layers[1].Paths) != 1 || def.Layers[1].Paths[0] != "internal/config/" {
		t.Errorf("want second layer paths [%q], got %v", "internal/config/", def.Layers[1].Paths)
	}
	if len(def.Layers[4].MayDependOn) != 4 {
		t.Errorf("want 4 may_depend_on for orchestrator, got %d", len(def.Layers[4].MayDependOn))
	}
}

func TestParse_EmptyLayers(t *testing.T) {
	input := `{"layers": []}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for empty layers, got nil")
	}
}

func TestParse_DuplicateNames(t *testing.T) {
	input := `{
		"layers": [
			{"name": "config", "paths": ["internal/config/"], "may_depend_on": []},
			{"name": "config", "paths": ["internal/config2/"], "may_depend_on": []}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for duplicate layer names, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("want error mentioning duplicate, got: %v", err)
	}
}

func TestParse_BadDependencyReference(t *testing.T) {
	input := `{
		"layers": [
			{"name": "config", "paths": ["internal/config/"], "may_depend_on": ["nonexistent"]}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for bad dependency reference, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("want error mentioning nonexistent, got: %v", err)
	}
}

func TestParse_BadMustNotDependOnReference(t *testing.T) {
	input := `{
		"layers": [
			{"name": "config", "paths": ["internal/config/"], "may_depend_on": [], "must_not_depend_on": ["nonexistent"]}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for bad must_not_depend_on reference, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("want error mentioning nonexistent, got: %v", err)
	}
}

func TestParse_EmptyName(t *testing.T) {
	input := `{
		"layers": [
			{"name": "", "paths": ["internal/types/"], "may_depend_on": []}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for empty layer name, got nil")
	}
}

func TestParse_EmptyPaths(t *testing.T) {
	input := `{
		"layers": [
			{"name": "types", "paths": [], "may_depend_on": []}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for empty layer paths, got nil")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	input := `{not valid json`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for invalid JSON, got nil")
	}
}

func TestParse_NoMayDependOnField(t *testing.T) {
	input := `{
		"layers": [
			{"name": "types", "paths": ["internal/types/"]}
		]
	}`
	def, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Layers[0].MayDependOn == nil {
		// nil slice is acceptable for missing field
		return
	}
	if len(def.Layers[0].MayDependOn) != 0 {
		t.Errorf("want empty may_depend_on, got %v", def.Layers[0].MayDependOn)
	}
}

func TestParseFile_Valid(t *testing.T) {
	content := `{
		"layers": [
			{"name": "types", "paths": ["internal/types/"], "may_depend_on": []}
		]
	}`
	tmp := filepath.Join(t.TempDir(), "architecture.json")
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	def, err := ParseFile(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def.Layers) != 1 {
		t.Fatalf("want 1 layer, got %d", len(def.Layers))
	}
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ParseFile("/nonexistent/path/architecture.json")
	if err == nil {
		t.Fatal("want error for missing file, got nil")
	}
}
