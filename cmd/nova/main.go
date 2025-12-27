// Package main is the entry point for the nova CLI.
package main

import (
	"os"

	"github.com/kzgrzendek/nova/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
