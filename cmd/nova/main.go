// Package main is the entry point for the nova CLI.
package main

import (
	"os"

	"github.com/kzgrzendek/nova/internal/cli/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
