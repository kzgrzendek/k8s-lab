// Package ui provides terminal UI utilities for colored output and spinners.
package ui

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	// Color functions
	successColor = color.New(color.FgGreen).SprintFunc()
	errorColor   = color.New(color.FgRed).SprintFunc()
	warnColor    = color.New(color.FgYellow).SprintFunc()
	infoColor    = color.New(color.FgCyan).SprintFunc()
	headerColor  = color.New(color.FgMagenta, color.Bold).SprintFunc()
	stepColor    = color.New(color.FgBlue).SprintFunc()
	dimColor     = color.New(color.Faint).SprintFunc()
)

// Header prints a prominent section header.
func Header(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("\n%s %s %s\n\n", headerColor("═══"), headerColor(msg), headerColor("═══"))
}

// Step prints a step indicator for multi-step operations.
func Step(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", stepColor("→"), msg)
}

// Success prints a success message with a checkmark.
func Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", successColor("✓"), msg)
}

// Error prints an error message with an X mark.
func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", errorColor("✗"), msg)
}

// Errorf creates and returns an error with the given format.
func Errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// Warn prints a warning message with a warning symbol.
func Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", warnColor("⚠"), msg)
}

// Info prints an informational message.
func Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", infoColor("ℹ"), msg)
}

// Debug prints a debug message (dimmed).
func Debug(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s\n", dimColor(msg))
}

// Print prints a plain message.
func Print(format string, args ...any) {
	fmt.Printf(format, args...)
}

// Println prints a plain message with newline.
func Println(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}
