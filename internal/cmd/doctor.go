package cmd

import (
	"fmt"
	"os"

	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/detect"
	"github.com/phs/dark-factory/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify system dependencies and environment before running godark",
	Long: `Run pre-flight checks to confirm that all required tools and environment
variables are in place. Prints a pass/fail checklist and exits non-zero if
any check fails.

Checks performed:
  • Docker daemon running
  • gh CLI installed
  • gh CLI authenticated
  • ANTHROPIC_API_KEY environment variable set
  • Detected runtime toolchain available (if a project is detected)
  • Python 3 available`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Best-effort runtime detection from the current directory.
		runtime := ""
		if dp, err := detect.DetectRuntime("."); err == nil {
			runtime = dp.Runtime.Name
		}

		// Best-effort config load to obtain lint_command.
		lintCommand := ""
		if cfg, err := config.Load("godark.yaml", config.CLIFlags{}); err == nil {
			lintCommand = cfg.LintCommand
		}

		checks := doctor.Checks(runtime, lintCommand)
		passed := doctor.Run(os.Stdout, checks)
		if !passed {
			return fmt.Errorf("pre-flight checks failed")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
