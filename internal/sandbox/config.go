package sandbox

import (
	"crypto/sha256"
	"fmt"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/pypi"
)

// DockerConfig holds the resolved configuration for Dockerfile generation.
type DockerConfig struct {
	Image              string
	Dockerfile         string // path to a user-provided Dockerfile; skips generation when set
	Runtime            config.Runtime
	NodeVersion        string
	SDKVersion         string // resolved claude-agent-sdk version; doubles as cache-bust key
	User               string
	ExtraPackages      []string
	InstallCommands    []string
	Mount              string
	SandboxEnv         map[string]string
	ComposeFile        string
	ComposeProjectName string
}

// DefaultDockerConfig returns a DockerConfig with sensible defaults.
func DefaultDockerConfig() DockerConfig {
	return DockerConfig{
		Image:       "ubuntu:22.04",
		NodeVersion: "20",
		User:        "devloop",
	}
}

// DockerConfigFromConfig maps a config.Docker, config.Runtime, sandbox environment
// variables, and an optional config.DockerCompose into a DockerConfig, applying
// defaults for any zero-value fields. compose may be nil (feature disabled).
func DockerConfigFromConfig(docker config.Docker, runtime config.Runtime, sandboxEnv map[string]string, compose *config.DockerCompose) DockerConfig {
	dc := DefaultDockerConfig()
	if docker.Dockerfile != "" {
		dc.Dockerfile = docker.Dockerfile
	}
	if docker.Image != "" {
		dc.Image = docker.Image
	}
	dc.Runtime = runtime
	if docker.NodeVersion != "" {
		dc.NodeVersion = docker.NodeVersion
	}
	if docker.User != "" {
		dc.User = docker.User
	}
	if docker.Mount != "" {
		dc.Mount = docker.Mount
	}
	if len(docker.ExtraPackages) > 0 {
		dc.ExtraPackages = docker.ExtraPackages
	}
	if len(docker.InstallCommands) > 0 {
		dc.InstallCommands = docker.InstallCommands
	}
	dc.SDKVersion = resolveSDKVersion()
	dc.SandboxEnv = sandboxEnv
	if compose != nil {
		dc.ComposeFile = compose.File
		dc.ComposeProjectName = compose.ProjectName
	}
	return dc
}

// resolveSDKVersion queries PyPI for the latest claude-agent-sdk version.
// Returns the version string (e.g. "0.1.50") or empty string on failure,
// which causes the Dockerfile to install the latest without pinning.
var resolveSDKVersion = func() string {
	v, err := pypi.LatestVersion(pypi.SDKPackage)
	if err != nil {
		return ""
	}
	return v
}

// ImageTag returns a deterministic image tag derived from the Dockerfile content.
// The tag format is "godark-runner:<first-12-chars-of-sha256>".
func ImageTag(dockerfileContent string) string {
	h := sha256.Sum256([]byte(dockerfileContent))
	return fmt.Sprintf("godark-runner:%x", h[:6])
}
