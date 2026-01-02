package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	assert.Equal(t, 4, cfg.Minikube.CPUs)
	assert.Equal(t, 4096, cfg.Minikube.Memory)
	assert.Equal(t, 3, cfg.Minikube.Nodes)
	assert.Equal(t, "v1.33.5", cfg.Minikube.KubernetesVersion)
	assert.Equal(t, "docker", cfg.Minikube.Driver)
	assert.Equal(t, "all", cfg.Minikube.GPUs)

	assert.Equal(t, "nova.local", cfg.DNS.Domain)
	assert.Equal(t, "auth.nova.local", cfg.DNS.AuthDomain)
	assert.Equal(t, 30053, cfg.DNS.Bind9Port)

	assert.False(t, cfg.State.Initialized)
	assert.Equal(t, 0, cfg.State.LastDeployedTier)
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create and save config
	cfg := Default()
	cfg.Minikube.CPUs = 8
	cfg.State.Initialized = true

	err := cfg.Save()
	require.NoError(t, err)

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".nova", "config.yaml")
	assert.FileExists(t, configPath)

	// Load and verify
	loaded, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8, loaded.Minikube.CPUs)
	assert.True(t, loaded.State.Initialized)
}

func TestLoadNotFound(t *testing.T) {
	// Create temp directory with no config
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoadOrDefault(t *testing.T) {
	// Create temp directory with no config
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Should return defaults when no config exists
	cfg := LoadOrDefault()
	assert.Equal(t, 4, cfg.Minikube.CPUs)
	assert.False(t, cfg.State.Initialized)
}
