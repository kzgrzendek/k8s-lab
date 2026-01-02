package sysctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	sysctlDir  = "/etc/sysctl.d"
	novaConf   = "99-nova.conf"
	sysctlConf = "/etc/sysctl.conf"
)

// InotifyLimits defines the inotify limits for Kubernetes workloads.
type InotifyLimits struct {
	MaxUserInstances int
	MaxUserWatches   int
}

// DefaultInotifyLimits returns recommended limits for Kubernetes.
func DefaultInotifyLimits() InotifyLimits {
	return InotifyLimits{
		MaxUserInstances: 2280,
		MaxUserWatches:   1255360,
	}
}

// ConfigureInotifyLimits sets inotify limits required for Kubernetes workloads.
// Uses /etc/sysctl.d/99-nova.conf for persistent configuration.
func ConfigureInotifyLimits(limits InotifyLimits) error {
	// Check if sysctl.d directory exists, fall back to sysctl.conf
	useSysctlD := true
	if _, err := os.Stat(sysctlDir); os.IsNotExist(err) {
		useSysctlD = false
	}

	content := fmt.Sprintf(`# NOVA inotify limits for Kubernetes
# Managed by nova CLI - DO NOT EDIT MANUALLY

fs.inotify.max_user_instances=%d
fs.inotify.max_user_watches=%d
`, limits.MaxUserInstances, limits.MaxUserWatches)

	var confPath string
	if useSysctlD {
		confPath = filepath.Join(sysctlDir, novaConf)
	} else {
		// Append to sysctl.conf
		return appendToSysctlConf(content)
	}

	// Write configuration file to temp location
	tmpFile := filepath.Join(os.TempDir(), novaConf)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Move to /etc/sysctl.d with sudo
	cmd := exec.Command("sudo", "cp", "-f", tmpFile, confPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install sysctl config: %w\nOutput: %s", err, string(output))
	}

	// Cleanup temp file
	os.Remove(tmpFile)

	// Apply the settings immediately
	return applySysctl()
}

// appendToSysctlConf appends nova settings to /etc/sysctl.conf.
func appendToSysctlConf(content string) error {
	// Read existing content
	existing, err := os.ReadFile(sysctlConf)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", sysctlConf, err)
	}

	// Check if already configured
	if strings.Contains(string(existing), "NOVA inotify limits") {
		// Already configured, skip
		return applySysctl()
	}

	// Append new content
	newContent := string(existing) + "\n" + content

	// Write to temp file
	tmpFile := filepath.Join(os.TempDir(), "sysctl.conf.nova")
	if err := os.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Copy with sudo
	cmd := exec.Command("sudo", "cp", "-f", tmpFile, sysctlConf)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update sysctl.conf: %w\nOutput: %s", err, string(output))
	}

	os.Remove(tmpFile)
	return applySysctl()
}

// applySysctl applies sysctl settings.
func applySysctl() error {
	cmd := exec.Command("sudo", "sysctl", "--system")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply sysctl settings: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// IsConfigured checks if NOVA sysctl configuration exists.
func IsConfigured() bool {
	// Check sysctl.d first
	confPath := filepath.Join(sysctlDir, novaConf)
	if _, err := os.Stat(confPath); err == nil {
		return true
	}

	// Check sysctl.conf
	content, err := os.ReadFile(sysctlConf)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), "NOVA inotify limits")
}

// RemoveConfiguration removes NOVA sysctl configuration.
func RemoveConfiguration() error {
	// Remove from sysctl.d
	confPath := filepath.Join(sysctlDir, novaConf)
	if _, err := os.Stat(confPath); err == nil {
		cmd := exec.Command("sudo", "rm", confPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to remove sysctl config: %w\nOutput: %s", err, string(output))
		}
	}

	// Note: We don't remove from sysctl.conf to avoid breaking other configs
	return nil
}
