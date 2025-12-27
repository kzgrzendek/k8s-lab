package pki

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCARoot(t *testing.T) {
	// Skip if mkcert not installed
	if _, err := exec.LookPath("mkcert"); err != nil {
		t.Skip("mkcert not installed")
	}

	caRoot, err := GetCARoot()
	assert.NoError(t, err)
	assert.NotEmpty(t, caRoot)
}

func TestIsInstalled(t *testing.T) {
	// Skip if mkcert not installed
	if _, err := exec.LookPath("mkcert"); err != nil {
		t.Skip("mkcert not installed")
	}

	// Just verify the function works
	installed, err := IsInstalled()
	assert.NoError(t, err)
	// installed could be true or false, both are valid
	_ = installed
}

func TestGetCAPaths(t *testing.T) {
	// Skip if mkcert not installed or CA not installed
	if _, err := exec.LookPath("mkcert"); err != nil {
		t.Skip("mkcert not installed")
	}

	installed, err := IsInstalled()
	if err != nil || !installed {
		t.Skip("mkcert CA not installed")
	}

	certPath, keyPath, err := GetCAPaths()
	assert.NoError(t, err)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)
	assert.Contains(t, certPath, "rootCA.pem")
	assert.Contains(t, keyPath, "rootCA-key.pem")
}
