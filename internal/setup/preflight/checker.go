// Package preflight provides dependency and system checks for NOVA.
package preflight

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
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
