package main

import (
	"os"

	"github.com/phs/dark-factory/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
