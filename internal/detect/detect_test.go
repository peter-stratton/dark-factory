package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectRuntime_Go(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "go" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "go")
	}
	if got.Runtime.Version != "1.26" {
		t.Errorf("Runtime.Version = %q, want %q", got.Runtime.Version, "1.26")
	}
	if got.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "go test ./...")
	}
	if got.BuildCommand != "go build ./..." {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "go build ./...")
	}
}

func TestDetectRuntime_Flutter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pubspec.yaml"), "name: myapp\nenvironment:\n  sdk: '>=3.0.0 <4.0.0'\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "flutter" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "flutter")
	}
	if got.TestCommand != "flutter test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "flutter test")
	}
	if got.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty", got.BuildCommand)
	}
}

func TestDetectRuntime_Node(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"myapp","engines":{"node":">=18"}}`)

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "node" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "node")
	}
	if got.TestCommand != "npm test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "npm test")
	}
	if got.BuildCommand != "npm run build" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "npm run build")
	}
}

func TestDetectRuntime_Rust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"myapp\"\nversion = \"0.1.0\"\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "rust" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "rust")
	}
	if got.TestCommand != "cargo test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "cargo test")
	}
	if got.BuildCommand != "cargo build" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "cargo build")
	}
}

func TestDetectRuntime_Python_Pyproject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[build-system]\nrequires = [\"setuptools\"]\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "python" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "python")
	}
	if got.TestCommand != "pytest" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "pytest")
	}
	if got.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty", got.BuildCommand)
	}
}

func TestDetectRuntime_Python_Requirements(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests==2.28.0\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "python" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "python")
	}
	if got.TestCommand != "pytest" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "pytest")
	}
}

func TestDetectRuntime_NoMarkerFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectRuntime(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "could not detect project type") {
		t.Errorf("error %q does not contain %q", err.Error(), "could not detect project type")
	}
}

func TestDetectRuntime_MultipleMarkers_GoWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"myapp"}`)

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "go" {
		t.Errorf("Runtime.Name = %q, want %q (Go should win)", got.Runtime.Name, "go")
	}
}

func TestDetectRuntime_VersionParsedFromGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.26.0\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Version != "1.26.0" {
		t.Errorf("Runtime.Version = %q, want %q", got.Runtime.Version, "1.26.0")
	}
}

func TestDetectRuntime_VersionMissingFromGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty string", got.Runtime.Version)
	}
}

func TestDetectRuntime_FlutterVersionParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pubspec.yaml"), "name: myapp\nenvironment:\n  sdk: '>=3.0.0 <4.0.0'\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Version != ">=3.0.0 <4.0.0" {
		t.Errorf("Runtime.Version = %q, want %q", got.Runtime.Version, ">=3.0.0 <4.0.0")
	}
}

func TestDetectRuntime_Elixir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\n  def project do\n    [\n      app: :my_app,\n      elixir: \"~> 1.14\"\n    ]\n  end\nend\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "elixir" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "elixir")
	}
	if got.TestCommand != "mix test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "mix test")
	}
	if got.BuildCommand != "mix compile" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "mix compile")
	}
}

func TestDetectRuntime_ElixirVersionParsed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\n  def project do\n    [\n      app: :my_app,\n      elixir: \"~> 1.14\"\n    ]\n  end\nend\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Version != "~> 1.14" {
		t.Errorf("Runtime.Version = %q, want %q", got.Runtime.Version, "~> 1.14")
	}
}

func TestDetectRuntime_ElixirNoVersionInMixExs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\n  def project do\n    [app: :my_app]\n  end\nend\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "elixir" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "elixir")
	}
	if got.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty string", got.Runtime.Version)
	}
}

func TestDetectRuntime_ElixirAfterRust(t *testing.T) {
	// Rust (Cargo.toml) has higher priority than Elixir (mix.exs).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"myapp\"\n")
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\nend\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "rust" {
		t.Errorf("Runtime.Name = %q, want rust (Rust should win over Elixir)", got.Runtime.Name)
	}
}

func TestDetectRuntime_ElixirBeforePython(t *testing.T) {
	// Elixir (mix.exs) has higher priority than Python (pyproject.toml).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\nend\n")
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[build-system]\nrequires = [\"setuptools\"]\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "elixir" {
		t.Errorf("Runtime.Name = %q, want elixir (Elixir should win over Python)", got.Runtime.Name)
	}
}

func TestDetectRuntime_NodeWinsOverElixir(t *testing.T) {
	// Node (package.json) has higher priority than Elixir (mix.exs).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"myapp"}`)
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\nend\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "node" {
		t.Errorf("Runtime.Name = %q, want node (Node should win over Elixir)", got.Runtime.Name)
	}
}

func TestDetectRuntime_ElixirCommentLineNotMatched(t *testing.T) {
	// A mix.exs that only mentions elixir: inside a comment should not parse a version.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\n  # elixir: \"~> 1.14\" (not a real key)\n  def project do\n    [app: :my_app]\n  end\nend\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "elixir" {
		t.Errorf("Runtime.Name = %q, want elixir", got.Runtime.Name)
	}
	if got.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty (comment line must not be matched)", got.Runtime.Version)
	}
}

func TestDetectRuntime_ElixirLongerAtomKeyNotMatched(t *testing.T) {
	// A key like "myelixir:" must not be mistaken for the "elixir:" key.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule MyApp.MixProject do\n  def project do\n    [\n      myelixir: \"~> 1.14\"\n    ]\n  end\nend\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "elixir" {
		t.Errorf("Runtime.Name = %q, want elixir", got.Runtime.Name)
	}
	if got.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty (myelixir: must not be matched)", got.Runtime.Version)
	}
}

// --- DetectAllRuntimes ---

func TestDetectAllRuntimes_Empty(t *testing.T) {
	dir := t.TempDir()
	got := DetectAllRuntimes(dir)
	if len(got) != 0 {
		t.Errorf("DetectAllRuntimes = %v, want empty slice", got)
	}
}

func TestDetectAllRuntimes_SingleRuntime(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.22\n")

	got := DetectAllRuntimes(dir)
	if len(got) != 1 || got[0] != "go" {
		t.Errorf("DetectAllRuntimes = %v, want [go]", got)
	}
}

func TestDetectAllRuntimes_MultipleRuntimes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n")
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"x\"\n")

	got := DetectAllRuntimes(dir)
	if len(got) != 2 {
		t.Fatalf("DetectAllRuntimes = %v, want length 2", got)
	}
	// Priority order: go before python.
	if got[0] != "go" || got[1] != "python" {
		t.Errorf("DetectAllRuntimes = %v, want [go python]", got)
	}
}

func TestDetectAllRuntimes_PythonMarkerEitherFile(t *testing.T) {
	// Either pyproject.toml or requirements.txt counts as one "python" entry,
	// never two.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"x\"\n")
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests\n")

	got := DetectAllRuntimes(dir)
	if len(got) != 1 || got[0] != "python" {
		t.Errorf("DetectAllRuntimes = %v, want [python]", got)
	}
}

func TestDetectAllRuntimes_AllRuntimesPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")
	writeFile(t, filepath.Join(dir, "pubspec.yaml"), "name: x\n")
	writeFile(t, filepath.Join(dir, "package.json"), "{}")
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname=\"x\"\n")
	writeFile(t, filepath.Join(dir, "mix.exs"), "defmodule X.MixProject do\nend\n")
	writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname=\"x\"\n")

	got := DetectAllRuntimes(dir)
	want := []string{"go", "flutter", "node", "rust", "elixir", "python"}
	if len(got) != len(want) {
		t.Fatalf("DetectAllRuntimes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DetectAllRuntimes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// writeFile is a test helper that creates a file with the given content.
func TestRuntimeMarkerPresent_SubdirModule(t *testing.T) {
	// Legacy Python markers at the root, Go module under server/. The Go marker
	// must be found even though it is not at the root.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests\n")
	if err := os.MkdirAll(filepath.Join(dir, "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "server", "go.mod"), "module example.com/server\n\ngo 1.22\n")

	if !RuntimeMarkerPresent(dir, "go") {
		t.Error("expected go marker to be found in server/ subdirectory")
	}
	if !RuntimeMarkerPresent(dir, "python") {
		t.Error("expected python marker to be found at root")
	}
}

func TestRuntimeMarkerPresent_Absent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "requests\n")
	if RuntimeMarkerPresent(dir, "go") {
		t.Error("expected go marker to be absent")
	}
}

func TestRuntimeMarkerPresent_SkipsHeavyDirs(t *testing.T) {
	// A marker buried in a skipped directory must not count.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "dep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "node_modules", "dep", "go.mod"), "module dep\n")
	if RuntimeMarkerPresent(dir, "go") {
		t.Error("expected go marker inside node_modules to be ignored")
	}
}

func TestRuntimeMarkerPresent_UnknownRuntime(t *testing.T) {
	if RuntimeMarkerPresent(t.TempDir(), "cobol") {
		t.Error("expected false for unknown runtime")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
