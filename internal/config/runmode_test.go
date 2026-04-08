package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildRunModeDefaultNoFlags(t *testing.T) {
	cfg := &Config{
		Concurrency: Concurrency{MaxWorkers: 4},
	}
	rm, err := BuildRunMode(cfg, CLIFlags{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := RunMode{Workers: 4, Integration: false}
	if rm != want {
		t.Errorf("got %+v, want %+v", rm, want)
	}
}

func TestBuildRunModeExplicitWorkers(t *testing.T) {
	cfg := &Config{
		Concurrency: Concurrency{MaxWorkers: 4},
	}
	rm, err := BuildRunMode(cfg, CLIFlags{Workers: intPtr(2)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := RunMode{Workers: 2, Integration: false}
	if rm != want {
		t.Errorf("got %+v, want %+v", rm, want)
	}
}

func TestBuildRunModeIntegrationForcesSerial(t *testing.T) {
	cfg := &Config{
		Concurrency:   Concurrency{MaxWorkers: 4},
		DockerCompose: &DockerCompose{File: "docker-compose.test.yml"},
	}
	rm, err := BuildRunMode(cfg, CLIFlags{Integration: boolPtr(true)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := RunMode{Workers: 1, Integration: true}
	if rm != want {
		t.Errorf("got %+v, want %+v", rm, want)
	}
}

func TestBuildRunModeIntegrationWithoutCompose(t *testing.T) {
	cfg := &Config{
		Concurrency: Concurrency{MaxWorkers: 4},
	}
	_, err := BuildRunMode(cfg, CLIFlags{Integration: boolPtr(true)})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "requires a docker_compose block") {
		t.Errorf("error = %q, want mention of 'requires a docker_compose block'", err.Error())
	}
}

func TestBuildRunModeIntegrationPlusWorkers(t *testing.T) {
	cfg := &Config{
		Concurrency:   Concurrency{MaxWorkers: 4},
		DockerCompose: &DockerCompose{File: "docker-compose.test.yml"},
	}
	_, err := BuildRunMode(cfg, CLIFlags{Integration: boolPtr(true), Workers: intPtr(2)})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("error = %q, want mention of 'cannot be combined'", err.Error())
	}
}

func TestBuildRunModeWorkersExceedsCeiling(t *testing.T) {
	cfg := &Config{
		Concurrency: Concurrency{MaxWorkers: 4},
	}
	_, err := BuildRunMode(cfg, CLIFlags{Workers: intPtr(10)})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds concurrency.max_workers") {
		t.Errorf("error = %q, want mention of 'exceeds concurrency.max_workers'", err.Error())
	}
}

func TestBuildRunModeWorkersLessThanOne(t *testing.T) {
	cfg := &Config{
		Concurrency: Concurrency{MaxWorkers: 4},
	}
	_, err := BuildRunMode(cfg, CLIFlags{Workers: intPtr(0)})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must be >= 1") {
		t.Errorf("error = %q, want mention of 'must be >= 1'", err.Error())
	}
}

func TestBuildRunModeConfigNotMutated(t *testing.T) {
	cases := []struct {
		name  string
		flags CLIFlags
	}{
		{"no flags", CLIFlags{}},
		{"explicit workers", CLIFlags{Workers: intPtr(2)}},
		{"integration", CLIFlags{Integration: boolPtr(true)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Concurrency:   Concurrency{MaxWorkers: 4},
				DockerCompose: &DockerCompose{File: "docker-compose.test.yml"},
			}
			// Create an independent snapshot with the same values.
			snapshot := &Config{
				Concurrency:   Concurrency{MaxWorkers: 4},
				DockerCompose: &DockerCompose{File: "docker-compose.test.yml"},
			}

			// Ignore errors — some combos are valid, some are not.
			_, _ = BuildRunMode(cfg, tc.flags)

			if !reflect.DeepEqual(cfg, snapshot) {
				t.Errorf("BuildRunMode mutated config for flags %+v", tc.flags)
			}
		})
	}
}
