package cmd

import (
	"os"
	"regexp"
	"strings"

	"github.com/phs/dark-factory/internal/vet"
	"github.com/spf13/cobra"
)

// printReport outputs the report in table or JSON format and exits non-zero
// if any errors were found.
func printReport(cmd *cobra.Command, report *vet.Report, asJSON bool) {
	if asJSON {
		report.PrintJSON(cmd.OutOrStdout())
	} else {
		report.Print(cmd.OutOrStdout())
	}
	if code := report.ExitCode(); code != 0 {
		os.Exit(code)
	}
}

// phasePattern matches "Phase N" at the start of a milestone title.
var phasePattern = regexp.MustCompile(`(?i)^phase\s+(\d+)`)

// milestoneToLabel extracts the phase label from a milestone title.
// "Phase 2" and "Phase 2: Vault Reader + Foundation" both become "phase-2".
// Falls back to full lowercase-hyphenated slug if no "Phase N" prefix is found.
func milestoneToLabel(milestone string) string {
	if m := phasePattern.FindStringSubmatch(milestone); m != nil {
		return "phase-" + m[1]
	}
	return strings.ReplaceAll(strings.ToLower(milestone), " ", "-")
}
