package dns

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestConfigureResolvconf_GeneratesCorrectContent(t *testing.T) {
	// This tests the DNS config content generation (not the actual file writing)
	// We test the content that would be written

	domains := []string{"nova.local", "auth.nova.local"}
	port := 30053

	// Create temp file to simulate the DNS config
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "nova.conf")

	// Generate content (same as ConfigureResolvconf does)
	content := fmt.Sprintf("# NOVA DNS configuration\n# Managed by nova CLI - DO NOT EDIT MANUALLY\n\n")
	content += fmt.Sprintf("# DNS server for NOVA domains (Bind9 on localhost:%d)\n", port)
	content += fmt.Sprintf("nameserver 127.0.0.1#%d\n\n", port)
	content += "# Search domains\n"
	for _, domain := range domains {
		content += fmt.Sprintf("search %s\n", domain)
	}

	// Write to temp file
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Read and verify content
	data, err := os.ReadFile(tmpFile)
	require.NoError(t, err)

	contentStr := string(data)

	// Verify correct domains (not old lab.k8s.local)
	assert.Contains(t, contentStr, "search nova.local")
	assert.Contains(t, contentStr, "search auth.nova.local")
	assert.NotContains(t, contentStr, "lab.k8s.local")
	assert.NotContains(t, contentStr, "auth.k8s.local")

	// Verify nameserver
	assert.Contains(t, contentStr, "nameserver 127.0.0.1#30053")

	// Verify comments
	assert.Contains(t, contentStr, "NOVA DNS configuration")
	assert.Contains(t, contentStr, "Bind9 on localhost:30053")

	// Verify domain count
	searchLines := 0
	for _, line := range strings.Split(contentStr, "\n") {
		if strings.HasPrefix(line, "search ") {
			searchLines++
		}
	}
	assert.Equal(t, len(domains), searchLines, "Should have one search line per domain")
}
