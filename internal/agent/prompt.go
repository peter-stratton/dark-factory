package agent

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/phs/dark-factory/internal/config"
)

// Prompts holds the loaded template text for each agent prompt.
type Prompts struct {
	Implementer      string
	ImplementerRetry string
	Reviewer         string
	SpecGenerator    string
}

// PromptData contains the values substituted into prompt templates.
type PromptData struct {
	IssueNumber    int
	IssueTitle     string
	IssueBody      string
	Slug           string
	Repo           string
	PRNumber       int
	BuildCommand   string
	TestCommand    string
	ProtectedPaths string
	ScenarioDir    string
	ReviewDir      string
	BranchExists   bool
}

// LoadPrompts reads the three prompt template files specified in cfg.Prompts.
func LoadPrompts(cfg *config.Config) (*Prompts, error) {
	impl, err := os.ReadFile(cfg.Prompts.Implementer)
	if err != nil {
		return nil, fmt.Errorf("reading implementer prompt %q: %w", cfg.Prompts.Implementer, err)
	}

	retry, err := os.ReadFile(cfg.Prompts.ImplementerRetry)
	if err != nil {
		return nil, fmt.Errorf("reading implementer_retry prompt %q: %w", cfg.Prompts.ImplementerRetry, err)
	}

	rev, err := os.ReadFile(cfg.Prompts.Reviewer)
	if err != nil {
		return nil, fmt.Errorf("reading reviewer prompt %q: %w", cfg.Prompts.Reviewer, err)
	}

	p := &Prompts{
		Implementer:      string(impl),
		ImplementerRetry: string(retry),
		Reviewer:         string(rev),
	}

	// SpecGenerator is optional — skip if path is empty or file doesn't exist.
	if cfg.Prompts.SpecGenerator != "" {
		specGen, err := os.ReadFile(cfg.Prompts.SpecGenerator)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading spec_generator prompt %q: %w", cfg.Prompts.SpecGenerator, err)
			}
			// File doesn't exist — leave SpecGenerator empty.
		} else {
			p.SpecGenerator = string(specGen)
		}
	}

	return p, nil
}

// RenderPrompt executes a text/template against the given data and returns
// the rendered string.
func RenderPrompt(tmplText string, data PromptData) (string, error) {
	t, err := template.New("prompt").Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parsing prompt template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing prompt template: %w", err)
	}
	return buf.String(), nil
}

// slugRe matches characters that are not alphanumeric or hyphens.
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string into a branch-name-safe slug.
// It lowercases, replaces non-alphanumeric runs with hyphens, and trims.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
