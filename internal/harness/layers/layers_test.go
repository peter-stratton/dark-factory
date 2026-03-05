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
			{"name": "types",        "dir": "internal/types/",        "imports": []},
			{"name": "config",       "dir": "internal/config/",       "imports": ["types"]},
			{"name": "deps",         "dir": "internal/deps/",         "imports": ["types", "config"]},
			{"name": "github",       "dir": "internal/github/",       "imports": ["types"]},
			{"name": "orchestrator", "dir": "internal/orchestrator/", "imports": ["types", "config", "deps", "github"]}
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
	if def.Layers[1].Dir != "internal/config/" {
		t.Errorf("want second layer dir %q, got %q", "internal/config/", def.Layers[1].Dir)
	}
	if len(def.Layers[4].Imports) != 4 {
		t.Errorf("want 4 imports for orchestrator, got %d", len(def.Layers[4].Imports))
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
			{"name": "config", "dir": "internal/config/", "imports": []},
			{"name": "config", "dir": "internal/config2/", "imports": []}
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

func TestParse_BadImportReference(t *testing.T) {
	input := `{
		"layers": [
			{"name": "config", "dir": "internal/config/", "imports": ["nonexistent"]}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for bad import reference, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("want error mentioning nonexistent, got: %v", err)
	}
}

func TestParse_EmptyName(t *testing.T) {
	input := `{
		"layers": [
			{"name": "", "dir": "internal/types/", "imports": []}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for empty layer name, got nil")
	}
}

func TestParse_EmptyDir(t *testing.T) {
	input := `{
		"layers": [
			{"name": "types", "dir": "", "imports": []}
		]
	}`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for empty layer dir, got nil")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	input := `{not valid json`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for invalid JSON, got nil")
	}
}

func TestParse_NoImportsField(t *testing.T) {
	input := `{
		"layers": [
			{"name": "types", "dir": "internal/types/"}
		]
	}`
	def, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Layers[0].Imports == nil {
		// nil slice is acceptable for missing field
		return
	}
	if len(def.Layers[0].Imports) != 0 {
		t.Errorf("want empty imports, got %v", def.Layers[0].Imports)
	}
}

func TestParseFile_Valid(t *testing.T) {
	content := `{
		"layers": [
			{"name": "types", "dir": "internal/types/", "imports": []}
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
