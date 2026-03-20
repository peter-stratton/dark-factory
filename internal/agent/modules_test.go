package agent

import (
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
)

// --- topologicalSortModules ---

func TestTopologicalSortModules_EmptyModules(t *testing.T) {
	result, err := topologicalSortModules(map[string]config.Module{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("got %v, want empty slice", result)
	}
}

func TestTopologicalSortModules_SingleModule(t *testing.T) {
	modules := map[string]config.Module{
		"service": {},
	}
	result, err := topologicalSortModules(modules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "service" {
		t.Errorf("got %v, want [service]", result)
	}
}

func TestTopologicalSortModules_DependencyOrder(t *testing.T) {
	// admin-cli depends on service — service must come first.
	modules := map[string]config.Module{
		"admin-cli": {DependsOn: []string{"service"}},
		"service":   {},
	}
	result, err := topologicalSortModules(modules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d modules, want 2", len(result))
	}
	if result[0] != "service" {
		t.Errorf("result[0] = %q, want \"service\" (dependency must come first)", result[0])
	}
	if result[1] != "admin-cli" {
		t.Errorf("result[1] = %q, want \"admin-cli\"", result[1])
	}
}

func TestTopologicalSortModules_NoDependenciesAlphaOrder(t *testing.T) {
	// Modules without dependencies should be sorted alphabetically.
	modules := map[string]config.Module{
		"zebra":  {},
		"alpha":  {},
		"middle": {},
	}
	result, err := topologicalSortModules(modules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("got %d modules, want 3", len(result))
	}
	if result[0] != "alpha" || result[1] != "middle" || result[2] != "zebra" {
		t.Errorf("got %v, want [alpha middle zebra]", result)
	}
}

func TestTopologicalSortModules_ChainDependency(t *testing.T) {
	// a → b → c: should produce [c, b, a].
	modules := map[string]config.Module{
		"a": {DependsOn: []string{"b"}},
		"b": {DependsOn: []string{"c"}},
		"c": {},
	}
	result, err := topologicalSortModules(modules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("got %d modules, want 3", len(result))
	}
	// c has no deps, b depends on c, a depends on b.
	if result[0] != "c" {
		t.Errorf("result[0] = %q, want \"c\"", result[0])
	}
	if result[1] != "b" {
		t.Errorf("result[1] = %q, want \"b\"", result[1])
	}
	if result[2] != "a" {
		t.Errorf("result[2] = %q, want \"a\"", result[2])
	}
}

func TestTopologicalSortModules_CycleReturnsError(t *testing.T) {
	modules := map[string]config.Module{
		"a": {DependsOn: []string{"b"}},
		"b": {DependsOn: []string{"a"}},
	}
	_, err := topologicalSortModules(modules)
	if err == nil {
		t.Error("expected error for cycle, got nil")
	}
}

// --- buildModuleVerifyChecks ---

func TestBuildModuleVerifyChecks_UsesModuleCommands(t *testing.T) {
	mod := config.Module{
		BuildCommand: "go build ./cmd/...",
		TestCommand:  "go test ./cmd/...",
		LintCommand:  "golint ./cmd/...",
	}
	cfg := &config.Config{
		BuildCommand: "go build ./...",
		TestCommand:  "go test ./...",
		LintCommand:  "golint ./...",
	}
	checks := buildModuleVerifyChecks("mymodule", mod, cfg)

	if len(checks) != 3 {
		t.Fatalf("got %d checks, want 3", len(checks))
	}
	for _, ch := range checks {
		if !strings.Contains(ch.Command, "cd mymodule && ") {
			t.Errorf("command %q missing cd prefix", ch.Command)
		}
		if !strings.Contains(ch.Command, "./cmd/...") {
			t.Errorf("command %q should use module command (./cmd/...), not root (./...)", ch.Command)
		}
	}
}

func TestBuildModuleVerifyChecks_InheritsRootCommands(t *testing.T) {
	// Module with no commands should inherit root-level defaults.
	mod := config.Module{}
	cfg := &config.Config{
		BuildCommand: "go build ./...",
		TestCommand:  "go test ./...",
	}
	checks := buildModuleVerifyChecks("svc", mod, cfg)

	if len(checks) != 2 {
		t.Fatalf("got %d checks, want 2", len(checks))
	}
	for _, ch := range checks {
		if !strings.HasPrefix(ch.Command, "cd svc && ") {
			t.Errorf("command %q should start with \"cd svc && \"", ch.Command)
		}
	}
	if checks[0].Name != "build" {
		t.Errorf("checks[0].Name = %q, want \"build\"", checks[0].Name)
	}
	if checks[1].Name != "test" {
		t.Errorf("checks[1].Name = %q, want \"test\"", checks[1].Name)
	}
}

func TestBuildModuleVerifyChecks_PartialInheritance(t *testing.T) {
	// Module with only lint_command: inherits build+test from root, uses own lint.
	mod := config.Module{
		LintCommand: "custom-lint",
	}
	cfg := &config.Config{
		BuildCommand: "go build ./...",
		TestCommand:  "go test ./...",
		LintCommand:  "root-lint",
	}
	checks := buildModuleVerifyChecks("mod", mod, cfg)

	if len(checks) != 3 {
		t.Fatalf("got %d checks, want 3", len(checks))
	}
	var lintCmd string
	for _, ch := range checks {
		if ch.Name == "lint" {
			lintCmd = ch.Command
		}
	}
	if !strings.Contains(lintCmd, "custom-lint") {
		t.Errorf("lint command = %q, should use module's custom-lint not root-lint", lintCmd)
	}
}

func TestBuildModuleVerifyChecks_CommandPrefixedWithModuleDir(t *testing.T) {
	mod := config.Module{
		BuildCommand: "make build",
	}
	cfg := &config.Config{}
	checks := buildModuleVerifyChecks("subdir/mymod", mod, cfg)

	if len(checks) != 1 {
		t.Fatalf("got %d checks, want 1", len(checks))
	}
	want := "cd subdir/mymod && make build"
	if checks[0].Command != want {
		t.Errorf("Command = %q, want %q", checks[0].Command, want)
	}
}

func TestBuildModuleVerifyChecks_EmptyWhenNoCommands(t *testing.T) {
	// No module commands and no root commands → empty list.
	mod := config.Module{}
	cfg := &config.Config{}
	checks := buildModuleVerifyChecks("empty", mod, cfg)

	if len(checks) != 0 {
		t.Errorf("got %d checks, want 0", len(checks))
	}
}

func TestBuildModuleVerifyChecks_OrderGenerateBuildLintTest(t *testing.T) {
	mod := config.Module{
		GenerateCommand: "go generate ./...",
		BuildCommand:    "go build ./...",
		LintCommand:     "golint ./...",
		TestCommand:     "go test ./...",
	}
	cfg := &config.Config{}
	checks := buildModuleVerifyChecks("mod", mod, cfg)

	if len(checks) != 4 {
		t.Fatalf("got %d checks, want 4", len(checks))
	}
	wantOrder := []string{"generate", "build", "lint", "test"}
	for i, want := range wantOrder {
		if checks[i].Name != want {
			t.Errorf("checks[%d].Name = %q, want %q", i, checks[i].Name, want)
		}
	}
}

// --- buildModuleContext ---

func TestBuildModuleContext_Empty(t *testing.T) {
	got := buildModuleContext(nil)
	if got != "" {
		t.Errorf("buildModuleContext(nil) = %q, want empty", got)
	}
}

func TestBuildModuleContext_SingleModule(t *testing.T) {
	modules := map[string]config.Module{
		"service": {},
	}
	got := buildModuleContext(modules)
	if !strings.Contains(got, "service") {
		t.Errorf("buildModuleContext missing module name: %q", got)
	}
}

func TestBuildModuleContext_IncludesDependencies(t *testing.T) {
	modules := map[string]config.Module{
		"admin-cli": {DependsOn: []string{"service"}},
		"service":   {},
	}
	got := buildModuleContext(modules)
	if !strings.Contains(got, "depends_on") {
		t.Errorf("buildModuleContext missing depends_on: %q", got)
	}
	if !strings.Contains(got, "service") {
		t.Errorf("buildModuleContext missing service: %q", got)
	}
	if !strings.Contains(got, "admin-cli") {
		t.Errorf("buildModuleContext missing admin-cli: %q", got)
	}
}

func TestBuildModuleContext_AlphaOrdering(t *testing.T) {
	modules := map[string]config.Module{
		"zebra": {},
		"alpha": {},
	}
	got := buildModuleContext(modules)
	alphaIdx := strings.Index(got, "alpha")
	zebraIdx := strings.Index(got, "zebra")
	if alphaIdx < 0 || zebraIdx < 0 {
		t.Fatalf("missing module names in output: %q", got)
	}
	if alphaIdx > zebraIdx {
		t.Errorf("alpha should appear before zebra in output: %q", got)
	}
}

// --- buildComposeServices ---

func TestBuildComposeServices_Nil(t *testing.T) {
	got := buildComposeServices(nil, nil)
	if got != "" {
		t.Errorf("buildComposeServices(nil, nil) = %q, want empty", got)
	}
}

func TestBuildComposeServices_Empty(t *testing.T) {
	dc := &config.DockerCompose{File: "docker-compose.yml"}
	got := buildComposeServices(dc, nil)
	if got != "" {
		t.Errorf("buildComposeServices with no services = %q, want empty", got)
	}
}

func TestBuildComposeServices_TwoServices(t *testing.T) {
	dc := &config.DockerCompose{
		File: "docker-compose.yml",
		Services: []config.ComposeService{
			{Name: "postgres", Description: "PostgreSQL on localhost:5432"},
			{Name: "redis", Description: "Redis on localhost:6379"},
		},
	}
	got := buildComposeServices(dc, nil)
	if !strings.Contains(got, "postgres") {
		t.Errorf("output missing postgres: %q", got)
	}
	if !strings.Contains(got, "PostgreSQL on localhost:5432") {
		t.Errorf("output missing postgres description: %q", got)
	}
	if !strings.Contains(got, "redis") {
		t.Errorf("output missing redis: %q", got)
	}
	if !strings.Contains(got, "Redis on localhost:6379") {
		t.Errorf("output missing redis description: %q", got)
	}
}

func TestBuildComposeServices_DescriptionOptional(t *testing.T) {
	dc := &config.DockerCompose{
		File: "docker-compose.yml",
		Services: []config.ComposeService{
			{Name: "postgres"},
		},
	}
	got := buildComposeServices(dc, nil)
	if !strings.Contains(got, "postgres") {
		t.Errorf("output missing service name: %q", got)
	}
	// Should not contain a trailing colon or empty description after name.
	if strings.Contains(got, "postgres:") {
		t.Errorf("output should not have colon after name when description is empty: %q", got)
	}
}

func TestBuildComposeServices_HostServicesOnly(t *testing.T) {
	hostServices := []config.HostService{
		{Name: "supabase", Description: "Supabase local stack"},
	}
	got := buildComposeServices(nil, hostServices)
	if !strings.Contains(got, "supabase") {
		t.Errorf("output missing host service name: %q", got)
	}
	if !strings.Contains(got, "(host)") {
		t.Errorf("output missing (host) suffix: %q", got)
	}
}

func TestBuildComposeServices_ComposeAndHostServicesCombined(t *testing.T) {
	dc := &config.DockerCompose{
		File: "docker-compose.yml",
		Services: []config.ComposeService{
			{Name: "redis", Description: "Redis on localhost:6379"},
		},
	}
	hostServices := []config.HostService{
		{Name: "supabase", Description: "Supabase local stack"},
	}
	got := buildComposeServices(dc, hostServices)
	if !strings.Contains(got, "redis") {
		t.Errorf("output missing compose service: %q", got)
	}
	if !strings.Contains(got, "supabase") {
		t.Errorf("output missing host service: %q", got)
	}
	// Only host services should have (host) suffix.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	for _, line := range lines {
		if strings.Contains(line, "redis") && strings.Contains(line, "(host)") {
			t.Errorf("compose service should not have (host) suffix: %q", line)
		}
		if strings.Contains(line, "supabase") && !strings.Contains(line, "(host)") {
			t.Errorf("host service should have (host) suffix: %q", line)
		}
	}
}

func TestBuildComposeServices_HostServiceDescriptionOptional(t *testing.T) {
	hostServices := []config.HostService{
		{Name: "wrangler"},
	}
	got := buildComposeServices(nil, hostServices)
	if !strings.Contains(got, "wrangler") {
		t.Errorf("output missing host service name: %q", got)
	}
	if strings.Contains(got, "wrangler:") {
		t.Errorf("output should not have colon after name when description is empty: %q", got)
	}
}
