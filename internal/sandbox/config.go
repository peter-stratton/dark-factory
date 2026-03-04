package sandbox

import (
	"crypto/sha256"
	"fmt"

	"github.com/phs/dark-factory/internal/config"
)

// DockerConfig holds the resolved configuration for Dockerfile generation.
type DockerConfig struct {
	Image         string
	Runtime       config.Runtime
	NodeVersion   string
	User          string
	ExtraPackages []string
	Mount         string
}

// DefaultDockerConfig returns a DockerConfig with sensible defaults.
func DefaultDockerConfig() DockerConfig {
	return DockerConfig{
		Image:       "ubuntu:22.04",
		NodeVersion: "20",
		User:        "devloop",
	}
}

// DockerConfigFromConfig maps a config.Docker and config.Runtime into a DockerConfig,
// applying defaults for any zero-value fields.
func DockerConfigFromConfig(docker config.Docker, runtime config.Runtime) DockerConfig {
	dc := DefaultDockerConfig()
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
	return dc
}

// ImageTag returns a deterministic image tag derived from the Dockerfile content.
// The tag format is "godark-runner:<first-12-chars-of-sha256>".
func ImageTag(dockerfileContent string) string {
	h := sha256.Sum256([]byte(dockerfileContent))
	return fmt.Sprintf("godark-runner:%x", h[:6])
}
