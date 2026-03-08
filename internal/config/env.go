package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidateRequiredEnv checks that all required environment variables are set.
// Returns an error listing any missing variables.
func ValidateRequiredEnv(required []string) error {
	if len(required) == 0 {
		return nil
	}

	var missing []string
	for _, name := range required {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}
