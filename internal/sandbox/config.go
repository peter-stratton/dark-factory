package sandbox

import (
	"crypto/sha256"
	"fmt"

	"github.com/phs/dark-factory/internal/config"
)

// DockerConfig holds the resolved configuration for Dockerfile generation.
type DockerConfig struct {
	Image         string
	GoVersion     string
	NodeVersion   string
	User          string
	ExtraPackages []string
	Mount         string
}

// DefaultDockerConfig returns a DockerConfig with sensible defaults.
func DefaultDockerConfig() DockerConfig {
	return DockerConfig{
		Image:       "ubuntu:22.04",
		GoVersion:   "1.26.0",
		NodeVersion: "20",
		User:        "devloop",
	}
}

// DockerConfigFromConfig maps a config.Docker into a DockerConfig,
// applying defaults for any zero-value fields.
func DockerConfigFromConfig(cfg config.Docker) DockerConfig {
	dc := DefaultDockerConfig()
	if cfg.Image != "" {
		dc.Image = cfg.Image
	}
	if cfg.GoVersion != "" {
		dc.GoVersion = cfg.GoVersion
	}
	if cfg.NodeVersion != "" {
		dc.NodeVersion = cfg.NodeVersion
	}
	if cfg.User != "" {
		dc.User = cfg.User
	}
	if cfg.Mount != "" {
		dc.Mount = cfg.Mount
	}
	if len(cfg.ExtraPackages) > 0 {
		dc.ExtraPackages = cfg.ExtraPackages
	}
	return dc
}

// ImageTag returns a deterministic image tag derived from the Dockerfile content.
// The tag format is "godark-runner:<first-12-chars-of-sha256>".
func ImageTag(dockerfileContent string) string {
	h := sha256.Sum256([]byte(dockerfileContent))
	return fmt.Sprintf("godark-runner:%x", h[:6])
}
