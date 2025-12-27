// Package gpu provides GPU detection and validation for NOVA.
package gpu

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Mode represents the GPU mode for the cluster.
type Mode int

const (
	// ModeDisabled indicates no GPU support.
	ModeDisabled Mode = iota
	// ModeNVIDIA indicates NVIDIA GPU support.
	ModeNVIDIA
	// ModeAMD indicates AMD GPU support (future).
	ModeAMD
	// ModeIntel indicates Intel GPU support (future).
	ModeIntel
)

// String returns the string representation of the GPU mode.
func (m Mode) String() string {
	switch m {
	case ModeDisabled:
		return "disabled"
	case ModeNVIDIA:
		return "nvidia"
	case ModeAMD:
		return "amd"
	case ModeIntel:
		return "intel"
	default:
		return "unknown"
	}
}

// Config holds GPU configuration.
type Config struct {
	Mode    Mode
	Enabled bool
}

// Detector handles GPU detection and validation.
type Detector struct {
	ctx context.Context
}

// NewDetector creates a new GPU detector.
func NewDetector(ctx context.Context) *Detector {
	return &Detector{ctx: ctx}
}

// DetectMode detects the GPU mode based on available hardware and drivers.
func (d *Detector) DetectMode() (Mode, error) {
	// Check for NVIDIA GPUs first
	if hasNVIDIA, err := d.HasNVIDIAGPU(); err == nil && hasNVIDIA {
		return ModeNVIDIA, nil
	}

	// Future: Check for AMD GPUs
	// Future: Check for Intel GPUs

	return ModeDisabled, nil
}

// HasNVIDIAGPU checks if NVIDIA GPU is available.
func (d *Detector) HasNVIDIAGPU() (bool, error) {
	// Check if nvidia-smi is available
	cmd := exec.CommandContext(d.ctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return false, nil // nvidia-smi not available or no GPU
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// HasNVIDIARuntime checks if NVIDIA container runtime is available.
func (d *Detector) HasNVIDIARuntime() (bool, error) {
	// Check docker info for nvidia runtime
	cmd := exec.CommandContext(d.ctx, "docker", "info", "-f", "{{.Runtimes}}")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check docker runtimes: %w", err)
	}

	runtimes := string(output)
	return strings.Contains(runtimes, "nvidia"), nil
}

// ValidateNVIDIASetup validates the complete NVIDIA setup.
func (d *Detector) ValidateNVIDIASetup() error {
	// Check for nvidia-smi
	hasGPU, err := d.HasNVIDIAGPU()
	if err != nil {
		return fmt.Errorf("failed to check for NVIDIA GPU: %w", err)
	}
	if !hasGPU {
		return fmt.Errorf("no NVIDIA GPU detected (nvidia-smi not available or no GPU found)")
	}

	// Check for NVIDIA container runtime
	hasRuntime, err := d.HasNVIDIARuntime()
	if err != nil {
		return fmt.Errorf("failed to check for NVIDIA runtime: %w", err)
	}
	if !hasRuntime {
		return fmt.Errorf("NVIDIA container runtime not available in Docker")
	}

	return nil
}

// GetNVIDIAGPUInfo returns information about NVIDIA GPUs.
func (d *Detector) GetNVIDIAGPUInfo() ([]string, error) {
	cmd := exec.CommandContext(d.ctx, "nvidia-smi", "--query-gpu=name,driver_version,memory.total", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query GPU info: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var gpus []string
	for _, line := range lines {
		if line != "" {
			gpus = append(gpus, strings.TrimSpace(line))
		}
	}

	return gpus, nil
}

// GetGPUConfig determines the GPU configuration based on user preference and system capabilities.
func GetGPUConfig(ctx context.Context, requestedMode string) (*Config, error) {
	detector := NewDetector(ctx)

	cfg := &Config{
		Mode:    ModeDisabled,
		Enabled: false,
	}

	// Handle explicit disable
	if requestedMode == "" || requestedMode == "none" || requestedMode == "disabled" {
		return cfg, nil
	}

	// Detect available GPU
	detectedMode, err := detector.DetectMode()
	if err != nil {
		return nil, fmt.Errorf("failed to detect GPU: %w", err)
	}

	// If auto mode, use detected mode
	if requestedMode == "auto" || requestedMode == "all" {
		if detectedMode == ModeDisabled {
			return cfg, nil // No GPU detected, return disabled
		}
		cfg.Mode = detectedMode
		cfg.Enabled = true

		// Validate the setup
		if detectedMode == ModeNVIDIA {
			if err := detector.ValidateNVIDIASetup(); err != nil {
				return nil, fmt.Errorf("NVIDIA GPU detected but setup incomplete: %w", err)
			}
		}

		return cfg, nil
	}

	// Explicit mode requested
	switch requestedMode {
	case "nvidia":
		if err := detector.ValidateNVIDIASetup(); err != nil {
			return nil, fmt.Errorf("NVIDIA mode requested but validation failed: %w", err)
		}
		cfg.Mode = ModeNVIDIA
		cfg.Enabled = true
	default:
		return nil, fmt.Errorf("unsupported GPU mode: %s (supported: auto, nvidia, none)", requestedMode)
	}

	return cfg, nil
}
