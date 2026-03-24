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

func TestDetectRuntime_Gradle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "build.gradle"), "plugins { id 'java' }\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "gradle" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "gradle")
	}
	if got.TestCommand != "./gradlew test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "./gradlew test")
	}
}

func TestDetectRuntime_GradleKts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "build.gradle.kts"), "plugins { java }\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "gradle" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "gradle")
	}
	if got.TestCommand != "./gradlew test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "./gradlew test")
	}
}

func TestDetectRuntime_Maven(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pom.xml"), "<project></project>\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "maven" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "maven")
	}
	if got.TestCommand != "mvn test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "mvn test")
	}
	if got.BuildCommand != "mvn package -DskipTests" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "mvn package -DskipTests")
	}
}

func TestDetectRuntime_DotnetCsproj(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "MyApp.csproj"), "<Project Sdk=\"Microsoft.NET.Sdk\"></Project>\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "dotnet" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "dotnet")
	}
	if got.TestCommand != "dotnet test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "dotnet test")
	}
	if got.BuildCommand != "dotnet build" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "dotnet build")
	}
}

func TestDetectRuntime_DotnetSln(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "MyApp.sln"), "\nMicrosoft Visual Studio Solution File\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "dotnet" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "dotnet")
	}
	if got.TestCommand != "dotnet test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "dotnet test")
	}
}

func TestDetectRuntime_Ruby(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Gemfile"), "source 'https://rubygems.org'\ngem 'rspec'\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "ruby" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "ruby")
	}
	if got.TestCommand != "bundle exec rspec" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "bundle exec rspec")
	}
	if got.BuildCommand != "" {
		t.Errorf("BuildCommand = %q, want empty", got.BuildCommand)
	}
}

func TestDetectRuntime_MakefileWithTestTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Makefile"), ".PHONY: build test\n\nbuild:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "make" {
		t.Errorf("Runtime.Name = %q, want %q", got.Runtime.Name, "make")
	}
	if got.TestCommand != "make test" {
		t.Errorf("TestCommand = %q, want %q", got.TestCommand, "make test")
	}
	if got.BuildCommand != "make build" {
		t.Errorf("BuildCommand = %q, want %q", got.BuildCommand, "make build")
	}
}

func TestDetectRuntime_MakefileWithoutTestTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Makefile"), ".PHONY: build\n\nbuild:\n\tgo build ./...\n")

	_, err := DetectRuntime(dir)
	if err == nil {
		t.Fatal("expected error for Makefile without test target, got nil")
	}
	if !strings.Contains(err.Error(), "could not detect project type") {
		t.Errorf("error %q does not contain %q", err.Error(), "could not detect project type")
	}
}

func TestDetectRuntime_GradleBeforeMaven(t *testing.T) {
	// Gradle (build.gradle) has higher priority than Maven (pom.xml).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "build.gradle"), "plugins { id 'java' }\n")
	writeFile(t, filepath.Join(dir, "pom.xml"), "<project></project>\n")

	got, err := DetectRuntime(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Runtime.Name != "gradle" {
		t.Errorf("Runtime.Name = %q, want gradle (Gradle should win over Maven)", got.Runtime.Name)
	}
}

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
