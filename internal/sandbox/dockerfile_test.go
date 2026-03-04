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
	if dc.GoVersion != "1.26.0" {
		t.Errorf("GoVersion = %q, want 1.26.0", dc.GoVersion)
	}
	if dc.NodeVersion != "20" {
		t.Errorf("NodeVersion = %q, want 20", dc.NodeVersion)
	}
	if dc.User != "devloop" {
		t.Errorf("User = %q, want devloop", dc.User)
	}
}

func TestDockerConfigFromConfig(t *testing.T) {
	cfg := config.Docker{
		Image:         "debian:bookworm",
		GoVersion:     "1.22",
		NodeVersion:   "18",
		User:          "runner",
		Mount:         "/src",
		ExtraPackages: []string{"vim", "jq"},
	}
	dc := DockerConfigFromConfig(cfg)

	if dc.Image != "debian:bookworm" {
		t.Errorf("Image = %q, want debian:bookworm", dc.Image)
	}
	if dc.GoVersion != "1.22" {
		t.Errorf("GoVersion = %q, want 1.22", dc.GoVersion)
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
	dc := DockerConfigFromConfig(config.Docker{})
	def := DefaultDockerConfig()

	if dc.Image != def.Image {
		t.Errorf("Image = %q, want default %q", dc.Image, def.Image)
	}
	if dc.GoVersion != def.GoVersion {
		t.Errorf("GoVersion = %q, want default %q", dc.GoVersion, def.GoVersion)
	}
}

func TestGenerateDockerfileDefault(t *testing.T) {
	df, err := GenerateDockerfile(DefaultDockerConfig())
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
		{"go install", "go1.26.0.linux-amd64.tar.gz"},
		{"node install", "setup_20.x"},
		{"useradd", "useradd -m -s /bin/bash devloop"},
		{"python3 install", "python3"},
		{"pip sdk install", "pip install claude-agent-sdk"},
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

	df, err := GenerateDockerfile(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(df, "FROM debian:bookworm") {
		t.Error("Dockerfile missing custom base image")
	}
}

func TestGenerateDockerfileCustomGoVersion(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.GoVersion = "1.22"

	df, err := GenerateDockerfile(cfg)
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

	df, err := GenerateDockerfile(cfg)
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

	df, err := GenerateDockerfile(cfg)
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
	df, err := GenerateDockerfile(DefaultDockerConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "python3") {
		t.Error("Dockerfile missing python3 in apt-get install")
	}
}

func TestGenerateDockerfileIncludesPipInstall(t *testing.T) {
	df, err := GenerateDockerfile(DefaultDockerConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "pip install claude-agent-sdk") {
		t.Error("Dockerfile missing 'pip install claude-agent-sdk'")
	}
}

func TestGenerateDockerfileCopiesAgentRunner(t *testing.T) {
	df, err := GenerateDockerfile(DefaultDockerConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "COPY agent_runner.py /usr/local/bin/agent_runner.py") {
		t.Error("Dockerfile missing COPY agent_runner.py directive")
	}
}
