package cmd

import (
	"fmt"
	"os"

	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/detect"
	"github.com/peter-stratton/dark-factory/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify system dependencies and environment before running godark",
	Long: `Run pre-flight checks to confirm that all required tools and environment
variables are in place. Prints a pass/fail checklist and exits non-zero if
any check fails.

Default checks (sandbox mode):
  • Docker daemon running
  • gh CLI installed and authenticated
  • Anthropic auth token set

With --no-sandbox, additional host toolchain checks are included:
  • Detected runtime toolchain available
  • Python 3 available
  • Lint tools available (if configured)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		noSandbox, _ := cmd.Flags().GetBool("no-sandbox")

		// Best-effort runtime detection from the current directory.
		runtime := ""
		if dp, err := detect.DetectRuntime("."); err == nil {
			runtime = dp.Runtime.Name
		}

		// Best-effort config load to obtain lint_command and compose config.
		lintCommand := ""
		composeConfigured := false
		if cfg, err := config.Load("godark.yaml", config.CLIFlags{}); err == nil {
			lintCommand = cfg.LintCommand
			composeConfigured = cfg.DockerCompose != nil
		}

		checks := doctor.Checks(doctor.Opts{
			Runtime:           runtime,
			LintCommand:       lintCommand,
			ComposeConfigured: composeConfigured,
			NoSandbox:         noSandbox,
		})
		passed := doctor.Run(os.Stdout, checks)
		if !passed {
			return fmt.Errorf("pre-flight checks failed")
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().Bool("no-sandbox", false, "Include host toolchain checks for --no-sandbox mode")
	rootCmd.AddCommand(doctorCmd)
}
