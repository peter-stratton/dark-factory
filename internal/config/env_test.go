package config

import (
	"strings"
	"testing"
)

func TestValidateRequiredEnvAllPresent(t *testing.T) {
	// HOME is always set in the test environment.
	if err := ValidateRequiredEnv([]string{"HOME"}); err != nil {
		t.Errorf("unexpected error for present var: %v", err)
	}
}

func TestValidateRequiredEnvOneMissing(t *testing.T) {
	err := ValidateRequiredEnv([]string{"DEFINITELY_NOT_SET_12345"})
	if err == nil {
		t.Fatal("expected error for missing var, got nil")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_NOT_SET_12345") {
		t.Errorf("error = %q, want mention of 'DEFINITELY_NOT_SET_12345'", err.Error())
	}
	if !strings.Contains(err.Error(), "missing required environment variables") {
		t.Errorf("error = %q, want mention of 'missing required environment variables'", err.Error())
	}
}

func TestValidateRequiredEnvMultipleMissing(t *testing.T) {
	err := ValidateRequiredEnv([]string{"DEFINITELY_NOT_SET_AAA", "DEFINITELY_NOT_SET_BBB"})
	if err == nil {
		t.Fatal("expected error for missing vars, got nil")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_NOT_SET_AAA") {
		t.Errorf("error = %q, want mention of 'DEFINITELY_NOT_SET_AAA'", err.Error())
	}
	if !strings.Contains(err.Error(), "DEFINITELY_NOT_SET_BBB") {
		t.Errorf("error = %q, want mention of 'DEFINITELY_NOT_SET_BBB'", err.Error())
	}
}

func TestValidateRequiredEnvEmptyList(t *testing.T) {
	if err := ValidateRequiredEnv([]string{}); err != nil {
		t.Errorf("unexpected error for empty list: %v", err)
	}
}

func TestValidateRequiredEnvNilList(t *testing.T) {
	if err := ValidateRequiredEnv(nil); err != nil {
		t.Errorf("unexpected error for nil list: %v", err)
	}
}

func TestValidateRequiredEnvErrorFormat(t *testing.T) {
	err := ValidateRequiredEnv([]string{"MISSING_X", "MISSING_Y"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "missing required environment variables: MISSING_X, MISSING_Y"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}
