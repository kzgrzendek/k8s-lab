package dns

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckResolvconfAvailable(t *testing.T) {
	// This test checks if the function correctly identifies resolvconf availability
	err := CheckResolvconfAvailable()

	// Check if resolvconf binary is available
	_, binErr := exec.LookPath("resolvconf")

	if binErr == nil {
		// If resolvconf binary exists, CheckResolvconfAvailable should succeed
		assert.NoError(t, err, "resolvconf binary found, check should pass")
	} else {
		// If binary doesn't exist, should error with helpful message
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in PATH")
		// Should include installation instructions
		assert.Contains(t, err.Error(), "Install resolvconf")
	}
}

func TestIsConfigured(t *testing.T) {
	// Just verify the function doesn't panic
	// Actual value depends on whether nova.conf exists
	result := IsConfigured()

	// Result should be a boolean (no panic)
	assert.IsType(t, false, result)
}
