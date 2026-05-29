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

Checks:
  • Docker daemon running
  • gh CLI installed and authenticated
  • Anthropic auth token set
  • Configured runtime matches repo files (when both are detectable)
  • Multi-runtime repo has modules: configured (when more than one runtime is detected)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Best-effort runtime detection from the current directory.
		runtime := ""
		if dp, err := detect.DetectRuntime("."); err == nil {
			runtime = dp.Runtime.Name
		}
		detectedRuntimes := detect.DetectAllRuntimes(".")

		// Best-effort config load to obtain lint_command, modules, compose, oauth.
		lintCommand := ""
		configuredRuntime := ""
		modulesConfigured := false
		composeConfigured := false
		oauthTokenEnv := ""
		if cfg, err := config.Load("godark.yaml", config.CLIFlags{}); err == nil {
			lintCommand = cfg.LintCommand
			configuredRuntime = cfg.Runtime.Name
			modulesConfigured = len(cfg.Modules) > 0
			composeConfigured = cfg.DockerCompose != nil
			oauthTokenEnv = cfg.OAuthTokenEnv
		}

		checks := doctor.Checks(doctor.Opts{
			Runtime:           runtime,
			ConfiguredRuntime: configuredRuntime,
			DetectedRuntimes:  detectedRuntimes,
			ModulesConfigured: modulesConfigured,
			LintCommand:       lintCommand,
			ComposeConfigured: composeConfigured,
			OAuthTokenEnv:     oauthTokenEnv,
		})
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
