package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/peter-stratton/dark-factory/internal/config"
)

func TestCheckHostServices_NoOp_WhenEmpty(t *testing.T) {
	err := CheckHostServices(context.Background(), nil, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = CheckHostServices(context.Background(), []config.HostService{}, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error for empty slice: %v", err)
	}
}

func TestCheckHostServices_NoHealthCheck_Skipped(t *testing.T) {
	services := []config.HostService{
		{Name: "description-only", Description: "Just a description"},
	}
	err := CheckHostServices(context.Background(), services, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error for service with no health_check: %v", err)
	}
}

func TestCheckHostServices_SuccessOnFirstAttempt(t *testing.T) {
	orig := CommandRunnerWithContext
	defer func() { CommandRunnerWithContext = orig }()

	CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	services := []config.HostService{
		{
			Name: "supabase",
			HealthCheck: &config.HostServiceHealthCheck{
				Command: "curl -sf http://localhost:54321/rest/v1/",
				Retries: 3,
			},
		},
	}
	err := CheckHostServices(context.Background(), services, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckHostServices_SuccessAfterRetry(t *testing.T) {
	orig := CommandRunnerWithContext
	defer func() { CommandRunnerWithContext = orig }()

	attempts := 0
	CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("connection refused")
		}
		return []byte("ok"), nil
	}

	services := []config.HostService{
		{
			Name: "supabase",
			HealthCheck: &config.HostServiceHealthCheck{
				Command: "curl -sf http://localhost:54321/rest/v1/",
				Timeout: "1s",
				Retries: 3,
			},
		},
	}
	err := CheckHostServices(context.Background(), services, testNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestCheckHostServices_AllRetriesExhausted(t *testing.T) {
	orig := CommandRunnerWithContext
	defer func() { CommandRunnerWithContext = orig }()

	CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("connection refused")
	}

	services := []config.HostService{
		{
			Name: "supabase",
			HealthCheck: &config.HostServiceHealthCheck{
				Command: "curl -sf http://localhost:54321/rest/v1/",
				Timeout: "1s",
				Retries: 2,
			},
		},
	}
	err := CheckHostServices(context.Background(), services, testNopLogger())
	if err == nil {
		t.Fatal("expected error when all retries exhausted")
	}
	if !strings.Contains(err.Error(), "supabase") {
		t.Errorf("error should name the failed service: %v", err)
	}
}

func TestCheckHostServices_MultipleServices_OneUnreachable(t *testing.T) {
	orig := CommandRunnerWithContext
	defer func() { CommandRunnerWithContext = orig }()

	CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := strings.Join(args, " ")
		if strings.Contains(cmd, "8787") {
			return nil, errors.New("connection refused")
		}
		return []byte("ok"), nil
	}

	services := []config.HostService{
		{
			Name: "supabase",
			HealthCheck: &config.HostServiceHealthCheck{
				Command: "curl -sf http://localhost:54321/rest/v1/",
				Timeout: "1s",
				Retries: 1,
			},
		},
		{
			Name: "wrangler",
			HealthCheck: &config.HostServiceHealthCheck{
				Command: "curl -sf http://localhost:8787/",
				Timeout: "1s",
				Retries: 1,
			},
		},
	}
	err := CheckHostServices(context.Background(), services, testNopLogger())
	if err == nil {
		t.Fatal("expected error when one service is unreachable")
	}
	if !strings.Contains(err.Error(), "wrangler") {
		t.Errorf("error should name the failed service: %v", err)
	}
	if strings.Contains(err.Error(), "supabase") {
		t.Errorf("error should not name the healthy service: %v", err)
	}
}

func TestCheckHostServices_DefaultRetries(t *testing.T) {
	orig := CommandRunnerWithContext
	defer func() { CommandRunnerWithContext = orig }()

	attempts := 0
	CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts++
		return nil, errors.New("fail")
	}

	services := []config.HostService{
		{
			Name: "svc",
			HealthCheck: &config.HostServiceHealthCheck{
				Command: "check",
				Timeout: "1s",
				// Retries: 0 → should default to 3
			},
		},
	}
	_ = CheckHostServices(context.Background(), services, testNopLogger())
	if attempts != 3 {
		t.Errorf("expected 3 attempts (default retries), got %d", attempts)
	}
}
