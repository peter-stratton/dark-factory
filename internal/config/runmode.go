package config

import "fmt"

// RunMode holds per-invocation runtime parameters resolved from config and
// CLI flags. It is immutable once constructed and does not mutate its inputs.
type RunMode struct {
	Workers     int
	Integration bool
}

// BuildRunMode resolves per-invocation runtime parameters from cfg and flags.
// It validates constraints and returns an error for any violation.
// It never mutates cfg.
func BuildRunMode(cfg *Config, flags CLIFlags) (RunMode, error) {
	if flags.Integration != nil && *flags.Integration {
		if cfg.DockerCompose == nil {
			return RunMode{}, fmt.Errorf("--integration requires a docker_compose block in config")
		}
		if flags.Workers != nil && *flags.Workers > 1 {
			return RunMode{}, fmt.Errorf("--integration cannot be combined with --workers > 1; integration services are shared and not safe under parallel workers")
		}
	}

	if flags.Workers != nil {
		if *flags.Workers < 1 {
			return RunMode{}, fmt.Errorf("--workers must be >= 1")
		}
		if *flags.Workers > cfg.Concurrency.MaxWorkers {
			return RunMode{}, fmt.Errorf("--workers %d exceeds concurrency.max_workers ceiling %d", *flags.Workers, cfg.Concurrency.MaxWorkers)
		}
	}

	var workers int
	switch {
	case flags.Integration != nil && *flags.Integration:
		workers = 1
	case flags.Workers != nil:
		workers = *flags.Workers
	default:
		workers = cfg.Concurrency.MaxWorkers
	}

	return RunMode{
		Workers:     workers,
		Integration: flags.Integration != nil && *flags.Integration,
	}, nil
}
