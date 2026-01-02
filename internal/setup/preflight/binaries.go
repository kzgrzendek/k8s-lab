package preflight

import (
	"fmt"
	"os/exec"
)

// checkBinary verifies that a binary is available in PATH.
func checkBinary(name string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("not found in PATH")
	}
	_ = path // could log path in verbose mode
	return nil
}

// GetBinaryPath returns the path to a binary, or empty string if not found.
func GetBinaryPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// IsBinaryAvailable checks if a binary is available without error reporting.
func IsBinaryAvailable(name string) bool {
	return checkBinary(name) == nil
}
