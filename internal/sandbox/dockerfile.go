package sandbox

import (
	"bytes"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"text/template"
)

var dockerfileTmpl = template.Must(template.New("Dockerfile").Parse(`FROM {{.Image}}

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    gnupg \
    python3 \
    python3-pip \
{{- range .ExtraPackages}}
    {{.}} \
{{- end}}
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
      | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
      | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*
{{if eq .RuntimeName "go"}}
# Install Go
RUN curl -fsSL https://go.dev/dl/go{{.RuntimeVersion}}.linux-amd64.tar.gz \
      | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
{{else if eq .RuntimeName "flutter"}}
# Install Flutter SDK
RUN git clone --branch {{.RuntimeVersion}} https://github.com/flutter/flutter /usr/local/flutter
ENV PATH="/usr/local/flutter/bin:${PATH}"
RUN flutter precache
{{else if eq .RuntimeName "rust"}}
# Install Rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
{{else if eq .RuntimeName "python"}}
# Install Python venv support
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*
{{else if eq .RuntimeName "elixir"}}
# Install Erlang/OTP and Elixir
RUN curl -fsSL https://packages.erlang-solutions.com/erlang-solutions_2.0_all.deb \
      -o /tmp/erlang-solutions.deb \
    && dpkg -i /tmp/erlang-solutions.deb && rm /tmp/erlang-solutions.deb \
    && apt-get update \
    && apt-get install -y --no-install-recommends esl-erlang{{if .RuntimeVersion}} "elixir={{.RuntimeVersion}}*"{{else}} elixir{{end}} \
    && rm -rf /var/lib/apt/lists/*
{{end}}
# Install Node.js
RUN curl -fsSL https://deb.nodesource.com/setup_{{.NodeVersion}}.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code
RUN npm install -g @anthropic-ai/claude-code
{{range .SandboxEnv}}
ENV {{.Key}}={{.Value}}
{{- end}}

# Install Python agent runner dependencies
RUN pip install 'claude-agent-sdk>=0.1.0,<0.2.0'

# Copy agent runner
COPY agent_runner.py /usr/local/bin/agent_runner.py

# Create non-root user
RUN useradd -m -s /bin/bash {{.User}}
USER {{.User}}
WORKDIR /workspace
`))

// envVar is a sorted key-value pair used for deterministic ENV rendering.
type envVar struct {
	Key   string
	Value string
}

// GenerateDockerfile renders a Dockerfile from the given DockerConfig.
func GenerateDockerfile(cfg DockerConfig, logger *slog.Logger) (string, error) {
	runtimeName := cfg.Runtime.Name
	runtimeVersion := cfg.Runtime.Version

	switch runtimeName {
	case "go":
		if runtimeVersion == "" {
			return "", fmt.Errorf("Go runtime requires a version (Runtime.Version must be set)")
		}
	case "flutter":
		if runtimeVersion == "" {
			runtimeVersion = "stable"
		}
	case "node", "rust", "python", "elixir", "":
		// no validation needed
	default:
		logger.Warn("unknown runtime, skipping toolchain install", "runtime", runtimeName)
		runtimeName = ""
	}

	// Validate runtimeVersion for newline injection.
	if strings.ContainsAny(runtimeVersion, "\n\r") {
		return "", fmt.Errorf("Runtime.Version must not contain newlines: %q", runtimeVersion)
	}

	// Validate config fields for newline injection.
	for _, s := range []string{cfg.Image, cfg.User, cfg.NodeVersion} {
		if strings.ContainsAny(s, "\n\r") {
			return "", fmt.Errorf("config field must not contain newlines: %q", s)
		}
	}
	for _, pkg := range cfg.ExtraPackages {
		if strings.ContainsAny(pkg, "\n\r") {
			return "", fmt.Errorf("ExtraPackages entry must not contain newlines: %q", pkg)
		}
	}

	// Sort SandboxEnv keys for deterministic Dockerfile output.
	sortedEnv := make([]envVar, 0, len(cfg.SandboxEnv))
	for k, v := range cfg.SandboxEnv {
		if strings.ContainsAny(k, "\n\r") || strings.ContainsAny(v, "\n\r") {
			return "", fmt.Errorf("SandboxEnv key/value must not contain newlines: %q=%q", k, v)
		}
		if strings.ContainsAny(k, "= \t") {
			return "", fmt.Errorf("SandboxEnv key must not contain '=', spaces, or tabs: %q", k)
		}
		if strings.ContainsAny(v, " \t") {
			return "", fmt.Errorf("SandboxEnv value must not contain spaces or tabs: %q=%q", k, v)
		}
		sortedEnv = append(sortedEnv, envVar{Key: k, Value: v})
	}
	sort.Slice(sortedEnv, func(i, j int) bool {
		return sortedEnv[i].Key < sortedEnv[j].Key
	})

	data := struct {
		Image          string
		RuntimeName    string
		RuntimeVersion string
		NodeVersion    string
		User           string
		ExtraPackages  []string
		SandboxEnv     []envVar
	}{
		Image:          cfg.Image,
		RuntimeName:    runtimeName,
		RuntimeVersion: runtimeVersion,
		NodeVersion:    cfg.NodeVersion,
		User:           cfg.User,
		ExtraPackages:  cfg.ExtraPackages,
		SandboxEnv:     sortedEnv,
	}

	var buf bytes.Buffer
	if err := dockerfileTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering Dockerfile template: %w", err)
	}

	return strings.TrimRight(buf.String(), "\n") + "\n", nil
}
