package dns

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckResolvconfAvailable(t *testing.T) {
	// This test will pass on systems with resolvconf, skip on others
	err := CheckResolvconfAvailable()

	// If resolvconf exists, no error
	if _, statErr := os.Stat(resolvconfDir); statErr == nil {
		assert.NoError(t, err)
	} else {
		// If directory doesn't exist, should error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not installed")
	}
}

func TestIsConfigured(t *testing.T) {
	// Just verify the function doesn't panic
	// Actual value depends on whether nova.conf exists
	_ = IsConfigured()
}
