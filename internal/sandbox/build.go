package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/peter-stratton/dark-factory/internal/agent/runner"
	darkexec "github.com/peter-stratton/dark-factory/internal/exec"
)

// CommandRunner executes a command and returns its combined output.
// Replaceable for testing.
var CommandRunner darkexec.CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// BuildImage generates a Dockerfile from cfg, writes it to a temp directory,
// and runs docker build. It returns the image tag on success.
func BuildImage(ctx context.Context, cfg DockerConfig, logger *slog.Logger) (string, error) {
	content, err := GenerateDockerfile(cfg, logger)
	if err != nil {
		return "", fmt.Errorf("generating Dockerfile: %w", err)
	}

	tag := ImageTag(content)
	logger.Info("building Docker image", "tag", tag)

	tmpDir, err := os.MkdirTemp("", "godark-docker-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dfPath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dfPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing Dockerfile: %w", err)
	}

	// Write embedded agent_runner.py to the build context so the COPY
	// directive in the Dockerfile can find it.
	pyContent, err := runner.FS.ReadFile("agent_runner.py")
	if err != nil {
		return "", fmt.Errorf("reading embedded agent_runner.py: %w", err)
	}
	pyPath := filepath.Join(tmpDir, "agent_runner.py")
	if err := os.WriteFile(pyPath, pyContent, 0o644); err != nil {
		return "", fmt.Errorf("writing agent_runner.py to build context: %w", err)
	}

	out, err := CommandRunner("docker", "build", "-t", tag, "-f", dfPath, tmpDir)
	if err != nil {
		return "", fmt.Errorf("docker build: %w\noutput: %s", err, out)
	}

	logger.Info("Docker image built", "tag", tag)
	return tag, nil
}
