package coverage

import (
	"reflect"
	"testing"
)

func TestParseProfile_Basic(t *testing.T) {
	data := `mode: set
github.com/phs/dark-factory/internal/foo/bar.go:10.2,15.3 2 1
github.com/phs/dark-factory/internal/foo/bar.go:20.2,25.3 3 0
github.com/phs/dark-factory/internal/baz/qux.go:5.10,8.2 1 1
`

	got := ParseProfile(data, "github.com/phs/dark-factory")

	want := []ProfileBlock{
		{File: "internal/foo/bar.go", StartLine: 10, EndLine: 15, Stmts: 2, Count: 1},
		{File: "internal/foo/bar.go", StartLine: 20, EndLine: 25, Stmts: 3, Count: 0},
		{File: "internal/baz/qux.go", StartLine: 5, EndLine: 8, Stmts: 1, Count: 1},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseProfile() = %v, want %v", got, want)
	}
}

func TestParseProfile_Empty(t *testing.T) {
	got := ParseProfile("", "github.com/phs/dark-factory")
	if got != nil {
		t.Errorf("ParseProfile(\"\") = %v, want nil", got)
	}
}

func TestParseProfile_ModeLineOnly(t *testing.T) {
	got := ParseProfile("mode: atomic\n", "github.com/phs/dark-factory")
	if got != nil {
		t.Errorf("ParseProfile(mode only) = %v, want nil", got)
	}
}

func TestParseProfile_NoModulePrefix(t *testing.T) {
	data := `mode: set
other/module/foo.go:1.1,5.2 1 1
`

	got := ParseProfile(data, "github.com/phs/dark-factory")

	// File path is not stripped since prefix doesn't match.
	if len(got) != 1 {
		t.Fatalf("ParseProfile() returned %d blocks, want 1", len(got))
	}
	if got[0].File != "other/module/foo.go" {
		t.Errorf("File = %q, want %q", got[0].File, "other/module/foo.go")
	}
}

func TestParseDotInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"10.5", 10},
		{"1.1", 1},
		{"100.99", 100},
		{"42", 42},
	}
	for _, tt := range tests {
		got := parseDotInt(tt.input)
		if got != tt.want {
			t.Errorf("parseDotInt(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
