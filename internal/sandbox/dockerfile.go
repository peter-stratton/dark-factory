package sandbox

import (
	"bytes"
	"fmt"
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

# Install Go
RUN curl -fsSL https://go.dev/dl/go{{.RuntimeVersion}}.linux-amd64.tar.gz \
      | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Install Node.js
RUN curl -fsSL https://deb.nodesource.com/setup_{{.NodeVersion}}.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code
RUN npm install -g @anthropic-ai/claude-code

# Install Python agent runner dependencies
RUN pip install 'claude-agent-sdk>=0.1.0,<0.2.0'

# Copy agent runner
COPY agent_runner.py /usr/local/bin/agent_runner.py

# Create non-root user
RUN useradd -m -s /bin/bash {{.User}}
USER {{.User}}
WORKDIR /workspace
`))

// GenerateDockerfile renders a Dockerfile from the given DockerConfig.
func GenerateDockerfile(cfg DockerConfig) (string, error) {
	data := struct {
		Image          string
		RuntimeVersion string
		NodeVersion    string
		User           string
		ExtraPackages  []string
	}{
		Image:          cfg.Image,
		RuntimeVersion: cfg.Runtime.Version,
		NodeVersion:    cfg.NodeVersion,
		User:           cfg.User,
		ExtraPackages:  cfg.ExtraPackages,
	}

	var buf bytes.Buffer
	if err := dockerfileTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering Dockerfile template: %w", err)
	}

	return strings.TrimRight(buf.String(), "\n") + "\n", nil
}
