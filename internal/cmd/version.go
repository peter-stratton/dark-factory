package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version and BuildTime are set via ldflags at build time.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version and build time",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("godark %s (built %s)\n", Version, BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
