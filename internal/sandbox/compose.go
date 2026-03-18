package sandbox

import (
	"context"
	"fmt"
	"log/slog"
)

// ComposeUp runs `docker compose up -d` for the configured compose file and
// project name. It is a no-op when dc.ComposeFile is empty (compose not
// configured).
func ComposeUp(_ context.Context, dc DockerConfig, logger *slog.Logger) error {
	if dc.ComposeFile == "" {
		return nil
	}

	projectName := dc.ComposeProjectName
	if projectName == "" {
		projectName = "godark"
	}

	logger.Info("starting compose services", "file", dc.ComposeFile, "project", projectName)

	out, err := CommandRunner("docker", "compose", "-f", dc.ComposeFile, "-p", projectName, "up", "-d")
	if err != nil {
		return fmt.Errorf("docker compose up: %w\noutput: %s", err, out)
	}

	logger.Info("compose services started", "file", dc.ComposeFile, "project", projectName)
	return nil
}

// ComposeDown runs `docker compose down --volumes` for the configured compose
// file and project name. It is a no-op when dc.ComposeFile is empty. Errors
// are logged as warnings but not returned — teardown is best-effort.
func ComposeDown(dc DockerConfig, logger *slog.Logger) {
	if dc.ComposeFile == "" {
		return
	}

	projectName := dc.ComposeProjectName
	if projectName == "" {
		projectName = "godark"
	}

	logger.Info("stopping compose services", "file", dc.ComposeFile, "project", projectName)

	out, err := CommandRunner("docker", "compose", "-f", dc.ComposeFile, "-p", projectName, "down", "--volumes")
	if err != nil {
		logger.Warn("docker compose down failed", "error", err, "output", string(out))
		return
	}

	logger.Info("compose services stopped", "file", dc.ComposeFile, "project", projectName)
}
