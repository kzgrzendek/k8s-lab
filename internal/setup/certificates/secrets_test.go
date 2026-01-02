package pki

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKubernetesSecret(t *testing.T) {
	// Skip if mkcert not installed or CA not installed
	if _, err := exec.LookPath("mkcert"); err != nil {
		t.Skip("mkcert not installed")
	}

	installed, err := IsInstalled()
	if err != nil || !installed {
		t.Skip("mkcert CA not installed")
	}

	// Create temp output file
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "ca-secret.yaml")

	// Generate secret
	err = GenerateKubernetesSecret(outputPath)
	require.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, outputPath)

	// Read and verify content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "apiVersion: v1")
	assert.Contains(t, contentStr, "kind: Secret")
	assert.Contains(t, contentStr, "name: nova-cacert")
	assert.Contains(t, contentStr, "namespace: cert-manager")
	assert.Contains(t, contentStr, "type: kubernetes.io/tls")
	assert.Contains(t, contentStr, "tls.crt:")
	assert.Contains(t, contentStr, "tls.key:")
}

func TestGetDefaultSecretPath(t *testing.T) {
	path := GetDefaultSecretPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, ".nova")
	assert.Contains(t, path, "ca-secret.yaml")
}
