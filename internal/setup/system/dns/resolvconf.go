package dns

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	resolvconfBaseDir = "/etc/resolvconf"
	resolvconfDir     = "/etc/resolvconf/resolv.conf.d"
	novaConfFile      = "nova.conf"
	bind9LocalPort    = 30053

	// systemd-resolved drop-in directory
	systemdResolvedDir  = "/etc/systemd/resolved.conf.d"
	systemdResolvedFile = "nova.conf"
)

// ConfigureResolvconf adds a dedicated DNS entry for NOVA domains.
// Uses resolvectl for systemd-resolved (preferred) and falls back to resolvconf.
func ConfigureResolvconf(domains []string, port int) error {
	// Try systemd-resolved first (modern approach)
	if isSystemdResolvedActive() {
		return configureSystemdResolved(domains, port)
	}

	// Fallback to traditional resolvconf
	if err := CheckResolvconfAvailable(); err != nil {
		return err
	}

	return configureResolvconfLegacy(domains, port)
}

// isSystemdResolvedActive checks if systemd-resolved is running.
func isSystemdResolvedActive() bool {
	cmd := exec.Command("systemctl", "is-active", "systemd-resolved")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return string(output) == "active\n"
}

// configureSystemdResolved configures DNS using a drop-in file for systemd-resolved.
func configureSystemdResolved(domains []string, port int) error {
	// Ensure the drop-in directory exists
	if _, err := os.Stat(systemdResolvedDir); os.IsNotExist(err) {
		cmd := exec.Command("sudo", "mkdir", "-p", systemdResolvedDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create %s: %w\nOutput: %s", systemdResolvedDir, err, string(output))
		}
	}

	// Build routing domains string (~ prefix means routing-only)
	var routingDomains []string
	for _, domain := range domains {
		routingDomains = append(routingDomains, "~"+domain)
	}

	// Create drop-in config content
	// Format: /etc/systemd/resolved.conf.d/nova.conf
	content := fmt.Sprintf(`# NOVA DNS configuration
# Managed by nova CLI - DO NOT EDIT MANUALLY

[Resolve]
DNS=127.0.0.1:%d
Domains=%s
`, port, strings.Join(routingDomains, " "))

	confPath := filepath.Join(systemdResolvedDir, systemdResolvedFile)

	// Write configuration file to temp location
	tmpFile := filepath.Join(os.TempDir(), systemdResolvedFile)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Move to /etc/systemd/resolved.conf.d with sudo
	cmd := exec.Command("sudo", "cp", "-f", tmpFile, confPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install DNS config: %w\nOutput: %s", err, string(output))
	}

	// Cleanup temp file
	os.Remove(tmpFile)

	// Restart systemd-resolved to apply changes
	cmd = exec.Command("sudo", "systemctl", "restart", "systemd-resolved")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// configureResolvconfLegacy uses the traditional resolvconf system.
func configureResolvconfLegacy(domains []string, port int) error {
	// Ensure the resolv.conf.d directory exists (create if needed)
	if _, err := os.Stat(resolvconfDir); os.IsNotExist(err) {
		cmd := exec.Command("sudo", "mkdir", "-p", resolvconfDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create %s: %w\nOutput: %s", resolvconfDir, err, string(output))
		}
	}

	// Create nova.conf content
	content := "# NOVA DNS configuration\n# Managed by nova CLI - DO NOT EDIT MANUALLY\n\n"
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
	// Try systemd-resolved first
	if isSystemdResolvedActive() {
		return removeSystemdResolved()
	}

	// Fallback to traditional resolvconf
	return removeResolvconfLegacy()
}

// removeSystemdResolved removes DNS configuration from systemd-resolved.
func removeSystemdResolved() error {
	confPath := filepath.Join(systemdResolvedDir, systemdResolvedFile)

	// Check if file exists
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return nil // Already removed
	}

	// Remove with sudo
	cmd := exec.Command("sudo", "rm", confPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove DNS config: %w\nOutput: %s", err, string(output))
	}

	// Restart systemd-resolved to apply changes
	cmd = exec.Command("sudo", "systemctl", "restart", "systemd-resolved")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// removeResolvconfLegacy removes DNS configuration from traditional resolvconf.
func removeResolvconfLegacy() error {
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
	// Check systemd-resolved drop-in file first
	if isSystemdResolvedActive() {
		confPath := filepath.Join(systemdResolvedDir, systemdResolvedFile)
		_, err := os.Stat(confPath)
		return err == nil
	}

	// Fallback: check resolvconf file
	confPath := filepath.Join(resolvconfDir, novaConfFile)
	_, err := os.Stat(confPath)
	return err == nil
}

// CheckResolvconfAvailable checks if DNS configuration is possible.
// Returns nil if either systemd-resolved or resolvconf is available.
func CheckResolvconfAvailable() error {
	// systemd-resolved is preferred and doesn't need resolvconf
	if isSystemdResolvedActive() {
		// Check resolvectl is available
		if _, err := exec.LookPath("resolvectl"); err != nil {
			return fmt.Errorf("resolvectl command not found but systemd-resolved is active")
		}
		return nil
	}

	// Fallback to traditional resolvconf
	if _, err := exec.LookPath("resolvconf"); err != nil {
		return fmt.Errorf("neither systemd-resolved nor resolvconf available\n\nInstall one of:\n  systemd-resolved (modern): sudo systemctl enable --now systemd-resolved\n  resolvconf (legacy): sudo apt install resolvconf")
	}

	// Check base directory exists (should exist if resolvconf is properly installed)
	if _, err := os.Stat(resolvconfBaseDir); os.IsNotExist(err) {
		return fmt.Errorf("resolvconf base directory %s not found - resolvconf may not be properly installed", resolvconfBaseDir)
	}

	return nil
}
