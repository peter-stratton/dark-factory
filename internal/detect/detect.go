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

// DetectAllRuntimes scans repoPath for language marker files and returns every
// runtime whose marker file is present. The returned slice is deduplicated and
// in deterministic priority order: go, flutter, node, rust, elixir, python.
// Returns an empty slice if no known marker files are found.
//
// This is meant for callers that need to detect multi-runtime repos (e.g.
// pre-flight checks that warn when modules: is not configured for a repo with
// both go.mod and pyproject.toml). For "what runtime is this project?" use
// DetectRuntime instead.
func DetectAllRuntimes(repoPath string) []string {
	var runtimes []string
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		runtimes = append(runtimes, "go")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "pubspec.yaml")); err == nil {
		runtimes = append(runtimes, "flutter")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "package.json")); err == nil {
		runtimes = append(runtimes, "node")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "Cargo.toml")); err == nil {
		runtimes = append(runtimes, "rust")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "mix.exs")); err == nil {
		runtimes = append(runtimes, "elixir")
	}
	pyProject := false
	if _, err := os.Stat(filepath.Join(repoPath, "pyproject.toml")); err == nil {
		pyProject = true
	}
	if !pyProject {
		if _, err := os.Stat(filepath.Join(repoPath, "requirements.txt")); err == nil {
			pyProject = true
		}
	}
	if pyProject {
		runtimes = append(runtimes, "python")
	}
	return runtimes
}

// runtimeMarkers maps a runtime name to the marker filenames that indicate its
// presence. A runtime is considered present if any of its markers exist.
var runtimeMarkers = map[string][]string{
	"go":      {"go.mod"},
	"flutter": {"pubspec.yaml"},
	"node":    {"package.json"},
	"rust":    {"Cargo.toml"},
	"elixir":  {"mix.exs"},
	"python":  {"pyproject.toml", "requirements.txt"},
}

// RuntimeMarkerPresent reports whether a marker file for the named runtime
// exists anywhere in repoPath, including subdirectories. This catches monorepo
// and rewrite-in-progress layouts where the module lives in a subdirectory
// (e.g. a Go module under server/ while legacy Python markers sit at the root),
// which root-only detection via DetectRuntime/DetectAllRuntimes would miss.
//
// Common heavy or vendored directories are skipped. Returns false for unknown
// runtimes or when no marker is found.
func RuntimeMarkerPresent(repoPath, runtime string) bool {
	markers := runtimeMarkers[runtime]
	if len(markers) == 0 {
		return false
	}
	markerSet := make(map[string]struct{}, len(markers))
	for _, m := range markers {
		markerSet[m] = struct{}{}
	}

	found := false
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than aborting the walk
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", ".godark", "build", "dist", ".dart_tool":
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := markerSet[d.Name()]; ok {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
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
	if _, err := os.Stat(filepath.Join(repoPath, "pyproject.toml")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "python"},
			BuildCommand: "",
			TestCommand:  "pytest",
		}, nil
	}
	if _, err := os.Stat(filepath.Join(repoPath, "requirements.txt")); err == nil {
		return &DetectedProject{
			Runtime:      config.Runtime{Name: "python"},
			BuildCommand: "",
			TestCommand:  "pytest",
		}, nil
	}

	return nil, fmt.Errorf("could not detect project type: no known marker files found in %s", repoPath)
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
