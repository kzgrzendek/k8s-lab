//go:build mage

// Package main contains Mage build targets for the nova CLI.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var (
	// Binary name
	binaryName = "nova"
	// Build output directory
	buildDir = "bin"
	// Main package path
	mainPkg = "./cmd/nova"
)

// Build builds the nova binary for the current platform.
func Build() error {
	mg.Deps(Tidy)
	fmt.Println("Building nova...")

	output := filepath.Join(buildDir, binaryName)
	if runtime.GOOS == "windows" {
		output += ".exe"
	}

	return sh.RunV("go", "build", "-o", output, mainPkg)
}

// BuildAll builds binaries for all supported platforms.
func BuildAll() error {
	mg.Deps(Tidy)
	platforms := []struct{ os, arch string }{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
	}

	for _, p := range platforms {
		fmt.Printf("Building for %s/%s...\n", p.os, p.arch)
		output := filepath.Join(buildDir, fmt.Sprintf("%s-%s-%s", binaryName, p.os, p.arch))
		if p.os == "windows" {
			output += ".exe"
		}

		env := map[string]string{"GOOS": p.os, "GOARCH": p.arch}
		if err := sh.RunWithV(env, "go", "build", "-o", output, mainPkg); err != nil {
			return err
		}
	}
	return nil
}

// Test runs all unit tests.
func Test() error {
	fmt.Println("Running tests...")
	return sh.RunV("go", "test", "-v", "-race", "-cover", "./...")
}

// TestShort runs tests in short mode (skip slow tests).
func TestShort() error {
	fmt.Println("Running short tests...")
	return sh.RunV("go", "test", "-v", "-short", "./...")
}

// TestCoverage runs tests and generates coverage report.
func TestCoverage() error {
	fmt.Println("Running tests with coverage...")
	coverageDir := "coverage"
	if err := os.MkdirAll(coverageDir, 0755); err != nil {
		return err
	}

	coverProfile := filepath.Join(coverageDir, "coverage.out")
	coverHTML := filepath.Join(coverageDir, "coverage.html")

	// Run tests with coverage
	if err := sh.RunV("go", "test", "-v", "-race", "-coverprofile="+coverProfile, "./..."); err != nil {
		return err
	}

	// Generate HTML report
	if err := sh.RunV("go", "tool", "cover", "-html="+coverProfile, "-o", coverHTML); err != nil {
		return err
	}

	fmt.Printf("Coverage report generated: %s\n", coverHTML)
	return nil
}

// Ci runs all checks for continuous integration (format, lint, test).
func Ci() error {
	mg.Deps(Fmt, Lint, Test)
	return nil
}

// Lint runs golangci-lint.
func Lint() error {
	fmt.Println("Running linter...")
	if err := checkCommand("golangci-lint"); err != nil {
		fmt.Println("golangci-lint not found, installing...")
		if err := sh.RunV("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"); err != nil {
			return err
		}
	}
	return sh.RunV("golangci-lint", "run", "./...")
}

// Tidy runs go mod tidy.
func Tidy() error {
	fmt.Println("Tidying modules...")
	return sh.RunV("go", "mod", "tidy")
}

// Install builds and installs nova to $GOPATH/bin.
func Install() error {
	mg.Deps(Build)
	fmt.Println("Installing nova...")
	return sh.RunV("go", "install", mainPkg)
}

// Clean removes build artifacts.
func Clean() error {
	fmt.Println("Cleaning...")
	return os.RemoveAll(buildDir)
}

// Fmt formats all Go source files.
func Fmt() error {
	fmt.Println("Formatting code...")
	return sh.RunV("go", "fmt", "./...")
}

// checkCommand checks if a command is available in PATH.
func checkCommand(name string) error {
	_, err := exec.LookPath(name)
	return err
}
