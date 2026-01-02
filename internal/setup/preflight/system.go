package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// checkLinux verifies we're running on Linux.
func checkLinux() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("NOVA requires Linux (detected: %s)", runtime.GOOS)
	}

	// Check for Debian/Ubuntu-based distro
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return nil // Debian-based
	}

	// Check os-release for Ubuntu/Debian
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fmt.Errorf("could not determine Linux distribution")
	}

	content := string(data)
	if strings.Contains(content, "ubuntu") || strings.Contains(content, "debian") {
		return nil
	}

	// Allow other distros but warn
	return nil // Be lenient - other distros may work
}

// checkSystemdResolved verifies systemd-resolved is running.
func checkSystemdResolved() error {
	// Check if systemd-resolved is active
	cmd := exec.Command("systemctl", "is-active", "systemd-resolved")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("systemd-resolved is not active (required for DNS configuration)")
	}

	status := strings.TrimSpace(string(output))
	if status != "active" {
		return fmt.Errorf("systemd-resolved is %s, expected active", status)
	}

	return nil
}

// IsRoot checks if the current process is running as root.
func IsRoot() bool {
	return os.Geteuid() == 0
}

// GetLinuxDistro returns the Linux distribution name.
func GetLinuxDistro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name := strings.TrimPrefix(line, "PRETTY_NAME=")
			name = strings.Trim(name, "\"")
			return name
		}
	}

	return "unknown"
}
