package preflight

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckBinary(t *testing.T) {
	// "ls" should exist on any Unix-like system
	if runtime.GOOS != "windows" {
		err := checkBinary("ls")
		assert.NoError(t, err)
	}

	// Non-existent binary should fail
	err := checkBinary("definitely-not-a-real-binary-12345")
	assert.Error(t, err)
}

func TestIsBinaryAvailable(t *testing.T) {
	// "ls" should exist on Unix
	if runtime.GOOS != "windows" {
		assert.True(t, IsBinaryAvailable("ls"))
	}

	// Non-existent binary
	assert.False(t, IsBinaryAvailable("definitely-not-a-real-binary-12345"))
}

func TestGetBinaryPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	path := GetBinaryPath("ls")
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "ls")

	path = GetBinaryPath("definitely-not-a-real-binary-12345")
	assert.Empty(t, path)
}

func TestCheckLinux(t *testing.T) {
	err := checkLinux()
	if runtime.GOOS == "linux" {
		assert.NoError(t, err)
	} else {
		assert.Error(t, err)
	}
}

func TestGetLinuxDistro(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux")
	}

	distro := GetLinuxDistro()
	// Should return something, even if "unknown"
	assert.NotEmpty(t, distro)
}

func TestNewChecker(t *testing.T) {
	c := NewChecker()

	// Should have the expected binary checks
	assert.Len(t, c.binaries, 4)

	// Verify expected binaries are configured
	names := make([]string, len(c.binaries))
	for i, b := range c.binaries {
		names[i] = b.Name
	}
	assert.Contains(t, names, "docker")
	assert.Contains(t, names, "minikube")
	assert.Contains(t, names, "mkcert")
	assert.Contains(t, names, "certutil")
}
