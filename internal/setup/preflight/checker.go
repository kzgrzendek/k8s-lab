// Package preflight provides dependency and system checks for NOVA.
package preflight

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// Checker performs preflight checks before NOVA operations.
type Checker struct {
	binaries []BinaryCheck
	system   []SystemCheck
}

// BinaryCheck defines a required binary dependency.
type BinaryCheck struct {
	Name        string
	InstallHint string
}

// SystemCheck defines a system requirement check.
type SystemCheck struct {
	Name  string
	Check func() error
}

// NewChecker creates a new preflight checker with default checks.
func NewChecker() *Checker {
	return &Checker{
		binaries: []BinaryCheck{
			{
				Name:        "docker",
				InstallHint: "https://docs.docker.com/get-docker/",
			},
			{
				Name:        "minikube",
				InstallHint: "https://minikube.sigs.k8s.io/docs/start/",
			},
			{
				Name:        "mkcert",
				InstallHint: "https://github.com/FiloSottile/mkcert#installation",
			},
			{
				Name:        "certutil",
				InstallHint: "apt install libnss3-tools (Ubuntu/Debian)",
			},
		},
		system: []SystemCheck{
			{
				Name:  "Linux OS",
				Check: checkLinux,
			},
			{
				Name:  "systemd-resolved",
				Check: checkSystemdResolved,
			},
		},
	}
}

// CheckAll runs all preflight checks.
func (c *Checker) CheckAll(ctx context.Context) error {
	// Check binaries
	for _, b := range c.binaries {
		if err := checkBinary(b.Name); err != nil {
			return fmt.Errorf("%s not found: %w\nInstall: %s", b.Name, err, b.InstallHint)
		}
		ui.Success("%s found", b.Name)

		// Special handling for minikube: check version
		if b.Name == "minikube" {
			if err := checkMinikubeVersion(ctx); err != nil {
				ui.Warn("%s", err.Error())
			}
		}
	}

	return nil
}

// CheckSystem runs system requirement checks.
func (c *Checker) CheckSystem() error {
	for _, s := range c.system {
		if err := s.Check(); err != nil {
			return fmt.Errorf("%s check failed: %w", s.Name, err)
		}
		ui.Success("%s OK", s.Name)
	}
	return nil
}

// CheckBinaries checks only binary dependencies.
func (c *Checker) CheckBinaries() error {
	for _, b := range c.binaries {
		if err := checkBinary(b.Name); err != nil {
			return fmt.Errorf("%s not found: %w\nInstall: %s", b.Name, err, b.InstallHint)
		}
	}
	return nil
}

// CheckGPU checks GPU availability and configuration.
func (c *Checker) CheckGPU(ctx context.Context, requestedMode string) (*shared.Config, error) {
	cfg, err := shared.GetGPUConfig(ctx, requestedMode)
	if err != nil {
		return nil, fmt.Errorf("GPU configuration failed: %w", err)
	}

	if cfg.Enabled {
		ui.Success("GPU mode: %s", cfg.Mode.String())

		// Get GPU info if NVIDIA
		if cfg.Mode == shared.ModeNVIDIA {
			detector := shared.NewDetector(ctx)
			gpus, err := detector.GetNVIDIAGPUInfo()
			if err == nil && len(gpus) > 0 {
				for i, gpuInfo := range gpus {
					ui.Info("  GPU %d: %s", i, gpuInfo)
				}
			}
		}
	} else {
		ui.Info("GPU mode: disabled (CPU-only)")
	}

	return cfg, nil
}

// checkMinikubeVersion checks the installed Minikube version and warns if it's outdated.
// Minimum recommended version is v1.30.0 for NOVA compatibility.
func checkMinikubeVersion(ctx context.Context) error {
	version, err := minikube.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("could not determine minikube version: %w", err)
	}

	ui.Info("  Minikube version: %s", version)

	// Parse version to check if it's below minimum recommended
	// Version format: "v1.37.0" or similar
	const minVersion = "v1.30.0"

	if compareVersions(version, minVersion) < 0 {
		return fmt.Errorf("minikube version %s is outdated (minimum recommended: %s)\nUpdate minikube: https://minikube.sigs.k8s.io/docs/start/", version, minVersion)
	}

	return nil
}

// compareVersions compares two semantic versions in format "v1.2.3".
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Strip 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Split into parts
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int

		if i < len(parts1) {
			p1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			p2, _ = strconv.Atoi(parts2[i])
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}
