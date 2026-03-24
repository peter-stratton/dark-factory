package detect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peter-stratton/dark-factory/internal/config"
	"gopkg.in/yaml.v3"
)

// DetectedProject holds the auto-detected project type and default commands.
type DetectedProject struct {
	Runtime      config.Runtime
	BuildCommand string
	TestCommand  string
}

// DetectRuntime scans repoPath for language marker files and returns a
// DetectedProject with sensible defaults. First match wins.
// Returns an error if no known marker file is found.
func DetectRuntime(repoPath string) (*DetectedProject, error) {
	// 1. Go — go.mod
	if data, err := os.ReadFile(filepath.Join(repoPath, "go.mod")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "go", Version: parseGoMod(data)},
			BuildCommand: "go build ./...",
			TestCommand:  "go test ./...",
		}, nil
	}

	// 2. Flutter — pubspec.yaml
	if data, err := os.ReadFile(filepath.Join(repoPath, "pubspec.yaml")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "flutter", Version: parsePubspecSDK(data)},
			BuildCommand: "",
			TestCommand:  "flutter test",
		}, nil
	}

	// 3. Node — package.json
	if data, err := os.ReadFile(filepath.Join(repoPath, "package.json")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "node", Version: parseNodeEngines(data)},
			BuildCommand: "npm run build",
			TestCommand:  "npm test",
		}, nil
	}

	// 4. Rust — Cargo.toml
	if _, err := os.Stat(filepath.Join(repoPath, "Cargo.toml")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "rust"},
			BuildCommand: "cargo build",
			TestCommand:  "cargo test",
		}, nil
	}

	// 5. Elixir — mix.exs
	if data, err := os.ReadFile(filepath.Join(repoPath, "mix.exs")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "elixir", Version: parseMixExs(data)},
			BuildCommand: "mix compile",
			TestCommand:  "mix test",
		}, nil
	}

	// 6. Python — pyproject.toml (preferred) or requirements.txt (fallback)
	_, errPy1 := os.Stat(filepath.Join(repoPath, "pyproject.toml"))
	_, errPy2 := os.Stat(filepath.Join(repoPath, "requirements.txt"))
	if errPy1 == nil || errPy2 == nil {
		return &DetectedProject{Runtime: config.Runtime{Name: "python"}, BuildCommand: "", TestCommand: "pytest"}, nil
	}

	// 7. Gradle — build.gradle or build.gradle.kts
	_, errG1 := os.Stat(filepath.Join(repoPath, "build.gradle"))
	_, errG2 := os.Stat(filepath.Join(repoPath, "build.gradle.kts"))
	if errG1 == nil || errG2 == nil {
		return &DetectedProject{Runtime: config.Runtime{Name: "gradle"}, BuildCommand: "", TestCommand: "./gradlew test"}, nil
	}

	// 8. Maven — pom.xml
	if _, err := os.Stat(filepath.Join(repoPath, "pom.xml")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "maven"},
			BuildCommand: "mvn package -DskipTests",
			TestCommand:  "mvn test",
		}, nil
	}

	// 9. .NET — *.csproj or *.sln
	csprojMatches, _ := filepath.Glob(filepath.Join(repoPath, "*.csproj"))
	slnMatches, _ := filepath.Glob(filepath.Join(repoPath, "*.sln"))
	if len(csprojMatches) > 0 || len(slnMatches) > 0 {
		return &DetectedProject{Runtime: config.Runtime{Name: "dotnet"}, BuildCommand: "dotnet build", TestCommand: "dotnet test"}, nil
	}

	// 10. Ruby — Gemfile
	if _, err := os.Stat(filepath.Join(repoPath, "Gemfile")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "ruby"},
			BuildCommand: "",
			TestCommand:  "bundle exec rspec",
		}, nil
	}

	// 11. Make — Makefile with a "test:" target
	if hasMakeTestTarget(filepath.Join(repoPath, "Makefile")) {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "make"},
			BuildCommand: "make build",
			TestCommand:  "make test",
		}, nil
	}

	return nil, fmt.Errorf("could not detect project type: no known marker files found in %s", repoPath)
}


// hasMakeTestTarget returns true if the given Makefile contains a "test:" target line.
func hasMakeTestTarget(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "test:") {
			return true
		}
	}
	return false
}

// parseGoMod extracts the Go version from go.mod content.
// Returns an empty string if no "go <version>" directive is found.
func parseGoMod(data []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	return ""
}

// parsePubspecSDK extracts the sdk version hint from pubspec.yaml content.
// Returns an empty string if the field is absent or the file cannot be parsed.
func parsePubspecSDK(data []byte) string {
	var pubspec struct {
		Environment struct {
			SDK string `yaml:"sdk"`
		} `yaml:"environment"`
	}
	if err := yaml.Unmarshal(data, &pubspec); err != nil {
		return ""
	}
	return pubspec.Environment.SDK
}

// parseNodeEngines extracts the node engine version from package.json content.
// Returns an empty string if the field is absent or the file cannot be parsed.
func parseNodeEngines(data []byte) string {
	var pkg struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Engines.Node
}

// parseMixExs extracts the elixir version constraint from mix.exs content.
// Returns an empty string if no elixir key is found or the value cannot be parsed.
// The check is anchored to the start of the trimmed line (like parseGoMod uses
// HasPrefix), which avoids false positives on comment lines and longer atom keys.
func parseMixExs(data []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "elixir:") {
			continue
		}
		rest := strings.TrimPrefix(line, "elixir:")
		i := strings.Index(rest, `"`)
		if i < 0 {
			continue
		}
		j := strings.Index(rest[i+1:], `"`)
		if j < 0 {
			continue
		}
		return rest[i+1 : i+1+j]
	}
	return ""
}
