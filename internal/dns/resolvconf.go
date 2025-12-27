package dns

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	resolvconfBaseDir = "/etc/resolvconf"
	resolvconfDir     = "/etc/resolvconf/resolv.conf.d"
	novaConfFile      = "nova.conf"
	bind9LocalPort    = 30053
)

// ConfigureResolvconf adds a dedicated DNS entry for NOVA domains.
// Creates /etc/resolvconf/resolv.conf.d/nova.conf with nameserver configuration.
func ConfigureResolvconf(domains []string, port int) error {
	// Verify resolvconf is available first
	if err := CheckResolvconfAvailable(); err != nil {
		return err
	}

	// Ensure the resolv.conf.d directory exists (create if needed)
	if _, err := os.Stat(resolvconfDir); os.IsNotExist(err) {
		cmd := exec.Command("sudo", "mkdir", "-p", resolvconfDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create %s: %w\nOutput: %s", resolvconfDir, err, string(output))
		}
	}

	// Create nova.conf content
	content := fmt.Sprintf("# NOVA DNS configuration\n# Managed by nova CLI - DO NOT EDIT MANUALLY\n\n")
	content += fmt.Sprintf("# DNS server for NOVA domains (Bind9 on localhost:%d)\n", port)
	content += fmt.Sprintf("nameserver 127.0.0.1#%d\n\n", port)
	content += "# Search domains\n"
	for _, domain := range domains {
		content += fmt.Sprintf("search %s\n", domain)
	}

	confPath := filepath.Join(resolvconfDir, novaConfFile)

	// Write configuration file to temp location
	tmpFile := filepath.Join(os.TempDir(), novaConfFile)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Move to /etc/resolvconf with sudo (overwrites existing file to support updates)
	cmd := exec.Command("sudo", "cp", "-f", tmpFile, confPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install DNS config (need sudo): %w\nOutput: %s", err, string(output))
	}

	// Update resolvconf
	cmd = exec.Command("sudo", "resolvconf", "-u")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update resolvconf: %w\nOutput: %s", err, string(output))
	}

	// Cleanup temp file
	os.Remove(tmpFile)

	return nil
}

// RemoveResolvconf removes the NOVA DNS configuration.
func RemoveResolvconf() error {
	confPath := filepath.Join(resolvconfDir, novaConfFile)

	// Check if file exists
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return nil // Already removed
	}

	// Remove with sudo
	cmd := exec.Command("sudo", "rm", confPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove DNS config: %w\nOutput: %s", err, string(output))
	}

	// Update resolvconf
	cmd = exec.Command("sudo", "resolvconf", "-u")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update resolvconf: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// IsConfigured checks if NOVA DNS configuration exists.
func IsConfigured() bool {
	confPath := filepath.Join(resolvconfDir, novaConfFile)
	_, err := os.Stat(confPath)
	return err == nil
}

// CheckResolvconfAvailable checks if resolvconf is installed and available.
func CheckResolvconfAvailable() error {
	// Check resolvconf binary first (most reliable indicator)
	if _, err := exec.LookPath("resolvconf"); err != nil {
		return fmt.Errorf("resolvconf command not found in PATH\n\nInstall resolvconf:\n  Ubuntu/Debian: sudo apt install resolvconf\n  Arch: sudo pacman -S openresolv")
	}

	// Check base directory exists (should exist if resolvconf is properly installed)
	if _, err := os.Stat(resolvconfBaseDir); os.IsNotExist(err) {
		return fmt.Errorf("resolvconf base directory %s not found - resolvconf may not be properly installed", resolvconfBaseDir)
	}

	// Note: We don't check for resolv.conf.d here because we'll create it if needed

	return nil
}
