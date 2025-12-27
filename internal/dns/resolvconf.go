package dns

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	resolvconfDir  = "/etc/resolvconf/resolv.conf.d"
	novaConfFile   = "nova.conf"
	bind9LocalPort = 30053
)

// ConfigureResolvconf adds a dedicated DNS entry for NOVA domains.
// Creates /etc/resolvconf/resolv.conf.d/nova.conf with nameserver configuration.
func ConfigureResolvconf(domains []string, port int) error {
	// Check if resolvconf is available
	if _, err := os.Stat(resolvconfDir); os.IsNotExist(err) {
		return fmt.Errorf("resolvconf not found at %s - install resolvconf package", resolvconfDir)
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

	// Write configuration file (requires sudo)
	tmpFile := filepath.Join(os.TempDir(), novaConfFile)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Move to /etc/resolvconf with sudo
	cmd := exec.Command("sudo", "cp", tmpFile, confPath)
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
	// Check directory exists
	if _, err := os.Stat(resolvconfDir); os.IsNotExist(err) {
		return fmt.Errorf("resolvconf not installed (directory %s not found)", resolvconfDir)
	}

	// Check resolvconf binary
	if _, err := exec.LookPath("resolvconf"); err != nil {
		return fmt.Errorf("resolvconf command not found in PATH")
	}

	return nil
}
