package sandbox

import (
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

// safeElixirVersion matches version strings containing only alphanumerics and
// version-constraint characters (e.g. "~>1.14.3", "1.14.3-rc.1+build").
var safeElixirVersion = regexp.MustCompile(`^[a-zA-Z0-9.~+\-><=]*$`)

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
{{- if .HasCompose}}
# Install Docker CLI and Compose v2 plugin (architecture-aware)
RUN apt-get update && apt-get install -y --no-install-recommends docker.io \
    && rm -rf /var/lib/apt/lists/* \
    && ARCH=$(dpkg --print-architecture) \
    && if [ "$ARCH" = "arm64" ]; then ARCH=aarch64; else ARCH=x86_64; fi \
    && mkdir -p /usr/local/lib/docker/cli-plugins \
    && curl -fsSL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-${ARCH} \
         -o /usr/local/lib/docker/cli-plugins/docker-compose \
    && chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
{{- end}}
{{if eq .RuntimeName "go"}}
# Install Go (architecture-aware)
RUN ARCH=$(dpkg --print-architecture) && \
    curl -fsSL https://go.dev/dl/go{{.RuntimeVersion}}.linux-${ARCH}.tar.gz \
      | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
{{else if eq .RuntimeName "flutter"}}
# Install Flutter SDK
RUN apt-get update && apt-get install -y --no-install-recommends unzip xz-utils \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --branch {{.RuntimeVersion}} https://github.com/flutter/flutter /usr/local/flutter
ENV PATH="/usr/local/flutter/bin:${PATH}"
RUN git config --system --add safe.directory /usr/local/flutter
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

{{range .SandboxEnv}}
ENV {{.Key}}={{.Value}}
{{- end}}
{{- range .InstallCommands}}
RUN {{.}}
{{- end}}

# Install Claude Code CLI via native installer (not the Python SDK, which has
# critical bugs — see claude-agent-sdk-python#666, #739).
WORKDIR /tmp
RUN curl -fsSL https://claude.ai/install.sh | bash \
    && cp -L /root/.local/bin/claude /usr/local/bin/claude \
    && chmod +x /usr/local/bin/claude

# Create non-root user
RUN useradd -m -s /bin/bash {{.User}}
{{- if eq .RuntimeName "flutter"}}
RUN chown -R {{.User}}:{{.User}} /usr/local/flutter
{{- end}}
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
			return "", fmt.Errorf("go runtime requires a version (Runtime.Version must be set)")
		}
		// Go download URLs require a full major.minor.patch version (e.g. "1.25.4").
		// A version like "1.25" produces a 404 on go.dev/dl, so append ".0" if
		// only major.minor is provided (common in go.mod files).
		parts := strings.Split(runtimeVersion, ".")
		if len(parts) == 2 {
			runtimeVersion += ".0"
		} else if len(parts) < 2 {
			return "", fmt.Errorf("go runtime.version %q must be at least major.minor (e.g. 1.25)", runtimeVersion)
		}
	case "flutter":
		// pubspec.yaml's environment.sdk is a Dart SDK constraint (e.g.
		// ^3.11.0), not a Flutter version. Flutter tags don't correspond
		// to Dart SDK versions, so always clone the stable channel unless
		// the user explicitly configures an exact Flutter tag.
		if runtimeVersion == "" || strings.ContainsAny(runtimeVersion, "^~><=") {
			runtimeVersion = "stable"
		}
	case "elixir":
		// Strip spaces to normalise Elixir version constraints (e.g. "~> 1.14" → "~>1.14").
		// mix.exs uses the pessimistic operator with a space, but apt package specs do not
		// allow spaces, so we remove them before validation and template rendering.
		runtimeVersion = strings.ReplaceAll(runtimeVersion, " ", "")
		// Enforce an allowlist for the version to prevent apt package injection.
		// The version is embedded in a quoted apt package spec (e.g. "elixir=1.14.3*"),
		// so characters like `"`, `$`, and backtick could break out of the argument.
		if !safeElixirVersion.MatchString(runtimeVersion) {
			return "", fmt.Errorf("runtime.version contains invalid character for Elixir: %q", runtimeVersion)
		}
	case "node", "rust", "python", "":
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
	for _, cmd := range cfg.InstallCommands {
		if strings.ContainsAny(cmd, "\n\r") {
			return "", fmt.Errorf("InstallCommands entry must not contain newlines: %q", cmd)
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
		Image             string
		RuntimeName       string
		RuntimeVersion    string
		NodeVersion       string
		SDKVersion        string
		User              string
		ExtraPackages     []string
		InstallCommands   []string
		SandboxEnv        []envVar
		HasCompose        bool
	}{
		Image:             cfg.Image,
		RuntimeName:       runtimeName,
		RuntimeVersion:    runtimeVersion,
		NodeVersion:       cfg.NodeVersion,
		SDKVersion:        cfg.SDKVersion,
		User:              cfg.User,
		ExtraPackages:     cfg.ExtraPackages,
		InstallCommands:   cfg.InstallCommands,
		SandboxEnv:        sortedEnv,
		HasCompose:        cfg.ComposeFile != "",
	}

	var buf bytes.Buffer
	if err := dockerfileTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering Dockerfile template: %w", err)
	}

	return strings.TrimRight(buf.String(), "\n") + "\n", nil
}
