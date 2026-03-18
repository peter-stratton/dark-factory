package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// ComposeUp runs `docker compose up -d` for the configured compose file and
// project name. It is a no-op when dc.ComposeFile is empty (compose not
// configured). requiredEnv names values from the host environment that should
// be forwarded to compose containers; auth-managed vars are excluded. A
// temporary .env file is written when any vars are forwarded; the returned
// cleanup func removes it. The caller must always invoke the cleanup func,
// even on error, and must call it after ComposeDown returns.
func ComposeUp(_ context.Context, dc DockerConfig, requiredEnv []string, logger *slog.Logger) (func(), error) {
	if dc.ComposeFile == "" {
		return func() {}, nil
	}

	projectName := dc.ComposeProjectName
	if projectName == "" {
		projectName = "godark"
	}

	logger.Info("starting compose services", "file", dc.ComposeFile, "project", projectName)

	args := []string{"compose", "-f", dc.ComposeFile, "-p", projectName}

	// Build the set of env vars to forward. Values are never logged.
	envVars := collectComposeEnv(requiredEnv)

	cleanup := func() {}
	if len(envVars) > 0 {
		f, err := os.CreateTemp("", "godark-compose-env-*")
		if err != nil {
			return func() {}, fmt.Errorf("creating compose env file: %w", err)
		}
		envFilePath := f.Name()
		cleanup = func() {
			os.Remove(envFilePath) //nolint:errcheck // best-effort cleanup
		}

		for k, v := range envVars {
			if _, err := fmt.Fprintf(f, "%s=%s\n", k, v); err != nil {
				f.Close()          //nolint:errcheck
				os.Remove(envFilePath) //nolint:errcheck
				return func() {}, fmt.Errorf("writing compose env file: %w", err)
			}
		}
		if err := f.Close(); err != nil {
			os.Remove(envFilePath) //nolint:errcheck
			return func() {}, fmt.Errorf("closing compose env file: %w", err)
		}

		args = append(args, "--env-file", envFilePath)
	}

	args = append(args, "up", "-d")

	out, err := CommandRunner("docker", args...)
	if err != nil {
		cleanup()
		return func() {}, fmt.Errorf("docker compose up: %w\noutput: %s", err, out)
	}

	logger.Info("compose services started", "file", dc.ComposeFile, "project", projectName)
	return cleanup, nil
}

// collectComposeEnv returns a map of environment variable names to values for
// forwarding to compose containers. It reads values from the host environment
// for each name in requiredEnv, skipping auth-managed vars and vars with empty
// or absent values. Values are never logged to prevent secret leakage.
func collectComposeEnv(requiredEnv []string) map[string]string {
	env := make(map[string]string)
	for _, name := range requiredEnv {
		if _, isAuth := authManagedVars[name]; isAuth {
			continue
		}
		if v := os.Getenv(name); v != "" {
			env[name] = v
		}
	}
	return env
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
