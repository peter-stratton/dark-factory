package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/peter-stratton/dark-factory/internal/config"
)

// CheckHostServices polls each host service's health check command until it
// succeeds or retries are exhausted. Returns an error listing all unreachable
// services. A nil or empty slice is a no-op.
func CheckHostServices(ctx context.Context, services []config.HostService, logger *slog.Logger) error {
	if len(services) == 0 {
		return nil
	}

	var failures []string
	for _, svc := range services {
		if svc.HealthCheck == nil {
			continue
		}

		retries := svc.HealthCheck.Retries
		if retries <= 0 {
			retries = 3
		}

		timeout := 5 * time.Second
		if svc.HealthCheck.Timeout != "" {
			// Validation already ensured this parses correctly.
			timeout, _ = time.ParseDuration(svc.HealthCheck.Timeout)
		}

		logger.Info("checking host service", "name", svc.Name, "retries", retries, "timeout", timeout)

		var lastErr error
		for attempt := 1; attempt <= retries; attempt++ {
			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			_, err := CommandRunnerWithContext(checkCtx, "sh", "-c", svc.HealthCheck.Command)
			cancel()

			if err == nil {
				logger.Info("host service healthy", "name", svc.Name, "attempt", attempt)
				lastErr = nil
				break
			}
			lastErr = err
			if attempt < retries {
				select {
				case <-ctx.Done():
					failures = append(failures, fmt.Sprintf("%s: %v", svc.Name, ctx.Err()))
					lastErr = nil // already recorded
					break
				case <-time.After(1 * time.Second):
				}
			}
		}

		if lastErr != nil {
			failures = append(failures, svc.Name)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("unreachable host services: %s — start them before running godark", strings.Join(failures, ", "))
	}
	return nil
}
