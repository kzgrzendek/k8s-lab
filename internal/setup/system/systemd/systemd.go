// Package systemd provides management for systemd user services.
// This enables automatic nova stop on system shutdown, halt, or suspend.
package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// ServiceName is the name of the nova stop service
	ServiceName = "nova-stop.service"

	// serviceContent is the systemd user service definition
	// It runs before shutdown.target, suspend.target, and hibernate.target
	serviceContent = `[Unit]
Description=Stop NOVA lab environment on shutdown/suspend
# Managed by nova CLI - DO NOT EDIT MANUALLY
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target suspend.target hibernate.target

[Service]
Type=oneshot
# Check if nova is running before stopping
ExecStart=/bin/bash -c 'if minikube -p nova status &>/dev/null; then %s stop; fi'
TimeoutStartSec=120
RemainAfterExit=yes

[Install]
WantedBy=shutdown.target reboot.target halt.target suspend.target hibernate.target
`
)

// GetUserServiceDir returns the path to the user systemd service directory.
func GetUserServiceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

// GetServicePath returns the full path to the nova-stop service file.
func GetServicePath() (string, error) {
	dir, err := GetUserServiceDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ServiceName), nil
}

// Install installs the nova-stop systemd user service.
// It creates the service file and enables it for shutdown/suspend targets.
func Install(novaBinaryPath string) error {
	// Get the service directory
	serviceDir, err := GetUserServiceDir()
	if err != nil {
		return err
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	// Get the absolute path to the nova binary
	novaPath := novaBinaryPath
	if novaPath == "" {
		// Try to find nova in PATH
		path, err := exec.LookPath("nova")
		if err != nil {
			return fmt.Errorf("nova binary not found in PATH - please provide path or install nova first")
		}
		novaPath = path
	}

	// Ensure the path is absolute
	if !filepath.IsAbs(novaPath) {
		absPath, err := filepath.Abs(novaPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		novaPath = absPath
	}

	// Generate the service content with the actual nova path
	content := fmt.Sprintf(serviceContent, novaPath)

	// Write the service file
	servicePath := filepath.Join(serviceDir, ServiceName)
	if err := os.WriteFile(servicePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd user daemon
	cmd := exec.Command("systemctl", "--user", "daemon-reload")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w\nOutput: %s", err, string(output))
	}

	// Enable the service for all shutdown/suspend targets
	cmd = exec.Command("systemctl", "--user", "enable", ServiceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable service: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Uninstall removes the nova-stop systemd user service.
func Uninstall() error {
	servicePath, err := GetServicePath()
	if err != nil {
		return err
	}

	// Check if service exists
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		// Service doesn't exist, nothing to uninstall
		return nil
	}

	// Disable the service first
	cmd := exec.Command("systemctl", "--user", "disable", ServiceName)
	// Ignore errors - service might not be enabled
	cmd.Run()

	// Stop the service if running
	cmd = exec.Command("systemctl", "--user", "stop", ServiceName)
	cmd.Run()

	// Remove the service file
	if err := os.Remove(servicePath); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd daemon
	cmd = exec.Command("systemctl", "--user", "daemon-reload")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// IsInstalled checks if the nova-stop service is installed.
func IsInstalled() bool {
	servicePath, err := GetServicePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(servicePath)
	return err == nil
}

// IsEnabled checks if the nova-stop service is enabled.
func IsEnabled() bool {
	cmd := exec.Command("systemctl", "--user", "is-enabled", ServiceName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "enabled"
}

// Status returns the current status of the nova-stop service.
func Status() (string, error) {
	cmd := exec.Command("systemctl", "--user", "status", ServiceName)
	output, _ := cmd.CombinedOutput()
	return string(output), nil
}
