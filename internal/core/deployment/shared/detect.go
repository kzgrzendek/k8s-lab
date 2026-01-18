// Package shared provides shared utilities for NOVA deployment tiers.
//
// This package contains code that is used across multiple deployment tiers,
// such as GPU detection and validation:
//   - Tier 0: GPU configuration and node labels during Minikube cluster creation
//   - Tier 1: Determines whether to deploy NVIDIA GPU Operator or Intel Device Plugin
//
// GPU detection uses ghw to identify available GPU hardware (NVIDIA, Intel) and validates
// that required drivers and container runtimes are properly configured.
package shared

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jaypipes/ghw"
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
	// ModeIntel indicates Intel GPU support.
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

// GPUVendor represents the GPU vendor.
type GPUVendor string

const (
	VendorNVIDIA  GPUVendor = "NVIDIA"
	VendorIntel   GPUVendor = "Intel"
	VendorAMD     GPUVendor = "AMD"
	VendorUnknown GPUVendor = "Unknown"
)

// GPUInfo holds information about a detected GPU.
type GPUInfo struct {
	Vendor  GPUVendor
	Name    string
	Driver  string
	Address string
}

// Detector handles GPU detection and validation.
type Detector struct {
	ctx context.Context
}

// NewDetector creates a new GPU detector.
func NewDetector(ctx context.Context) *Detector {
	return &Detector{ctx: ctx}
}

// DetectGPUsWithGHW uses ghw to detect all GPUs (no root required).
func (d *Detector) DetectGPUsWithGHW() ([]GPUInfo, error) {
	gpu, err := ghw.GPU()
	if err != nil {
		return nil, fmt.Errorf("ghw GPU detection failed: %w", err)
	}

	var gpus []GPUInfo
	for _, card := range gpu.GraphicsCards {
		gpuInfo := GPUInfo{
			Vendor: VendorUnknown,
			Name:   "Unknown GPU",
		}

		if card.Address != "" {
			gpuInfo.Address = card.Address
		}

		if card.DeviceInfo != nil {
			// Get vendor from PCI database
			if card.DeviceInfo.Vendor != nil {
				vendorName := card.DeviceInfo.Vendor.Name
				gpuInfo.Name = vendorName

				vendorLower := strings.ToLower(vendorName)
				switch {
				case strings.Contains(vendorLower, "nvidia"):
					gpuInfo.Vendor = VendorNVIDIA
				case strings.Contains(vendorLower, "intel"):
					gpuInfo.Vendor = VendorIntel
				case strings.Contains(vendorLower, "amd") || strings.Contains(vendorLower, "advanced micro"):
					gpuInfo.Vendor = VendorAMD
				}
			}

			// Get product name
			if card.DeviceInfo.Product != nil && card.DeviceInfo.Product.Name != "" {
				gpuInfo.Name = card.DeviceInfo.Product.Name
			}

			// Get driver
			if card.DeviceInfo.Driver != "" {
				gpuInfo.Driver = card.DeviceInfo.Driver
			}
		}

		gpus = append(gpus, gpuInfo)
	}

	return gpus, nil
}

// DetectMode detects the GPU mode based on available hardware and drivers.
// Priority: NVIDIA first, then Intel.
func (d *Detector) DetectMode() (Mode, error) {
	// Try ghw-based detection first
	gpus, err := d.DetectGPUsWithGHW()
	if err == nil && len(gpus) > 0 {
		// Check for NVIDIA first (higher priority)
		for _, gpu := range gpus {
			if gpu.Vendor == VendorNVIDIA {
				return ModeNVIDIA, nil
			}
		}
		// Check for Intel
		for _, gpu := range gpus {
			if gpu.Vendor == VendorIntel {
				return ModeIntel, nil
			}
		}
	}

	// Fallback to nvidia-smi check if ghw didn't find anything
	if hasNVIDIA, err := d.HasNVIDIAGPU(); err == nil && hasNVIDIA {
		return ModeNVIDIA, nil
	}

	// Fallback to Intel driver check
	if hasIntel, err := d.HasIntelGPU(); err == nil && hasIntel {
		return ModeIntel, nil
	}

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

// HasIntelGPU checks if Intel GPU is available by checking for i915 or xe kernel modules.
func (d *Detector) HasIntelGPU() (bool, error) {
	// Check for i915 (integrated/older discrete) or xe (newer Arc) kernel modules
	cmd := exec.CommandContext(d.ctx, "lsmod")
	output, err := cmd.Output()
	if err != nil {
		return false, nil // lsmod not available
	}

	outputStr := string(output)
	return strings.Contains(outputStr, "i915") || strings.Contains(outputStr, "xe"), nil
}

// ValidateIntelSetup validates the Intel GPU setup.
func (d *Detector) ValidateIntelSetup() error {
	hasGPU, err := d.HasIntelGPU()
	if err != nil {
		return fmt.Errorf("failed to check for Intel GPU: %w", err)
	}
	if !hasGPU {
		return fmt.Errorf("no Intel GPU detected (i915/xe kernel modules not loaded)")
	}

	return nil
}

// GetIntelGPUInfo returns information about Intel GPUs using ghw.
func (d *Detector) GetIntelGPUInfo() ([]string, error) {
	gpus, err := d.DetectGPUsWithGHW()
	if err != nil {
		return nil, err
	}

	var intelGPUs []string
	for _, gpu := range gpus {
		if gpu.Vendor == VendorIntel {
			info := gpu.Name
			if gpu.Driver != "" {
				info += fmt.Sprintf(" (driver: %s)", gpu.Driver)
			}
			intelGPUs = append(intelGPUs, info)
		}
	}

	return intelGPUs, nil
}

// GetGPUConfig determines the GPU configuration based on user preference and system capabilities.
// A GPU (NVIDIA or Intel) is required for LLM inference.
func GetGPUConfig(ctx context.Context, requestedMode string) (*Config, error) {
	detector := NewDetector(ctx)

	cfg := &Config{
		Mode:    ModeDisabled,
		Enabled: false,
	}

	// Detect available GPU
	detectedMode, err := detector.DetectMode()
	if err != nil {
		return nil, fmt.Errorf("failed to detect GPU: %w", err)
	}

	// If auto mode (default), use detected mode
	if requestedMode == "" || requestedMode == "auto" || requestedMode == "all" {
		if detectedMode == ModeDisabled {
			return nil, fmt.Errorf("no GPU detected. A GPU is required for LLM inference.\n\nChecked for:\n  - NVIDIA GPU: nvidia-smi not found or no GPU detected\n  - Intel GPU: i915/xe kernel modules not loaded\n\nTo fix:\n  - For NVIDIA: Install NVIDIA drivers and Container Toolkit\n  - For Intel: Ensure i915 or xe kernel module is loaded")
		}
		cfg.Mode = detectedMode
		cfg.Enabled = true

		// Validate the setup
		switch detectedMode {
		case ModeNVIDIA:
			if err := detector.ValidateNVIDIASetup(); err != nil {
				return nil, fmt.Errorf("NVIDIA GPU detected but setup incomplete: %w", err)
			}
		case ModeIntel:
			if err := detector.ValidateIntelSetup(); err != nil {
				return nil, fmt.Errorf("Intel GPU detected but setup incomplete: %w", err)
			}
		}

		return cfg, nil
	}

	// Explicit mode requested
	switch requestedMode {
	case "nvidia":
		if err := detector.ValidateNVIDIASetup(); err != nil {
			return nil, fmt.Errorf("NVIDIA mode requested but validation failed: %w\n\nTo fix:\n  1. Install NVIDIA drivers: sudo apt install nvidia-driver-550\n  2. Install NVIDIA Container Toolkit\n  3. Restart Docker: sudo systemctl restart docker\n\nOr use: nova start --gpu=intel (for Intel integrated/discrete GPU)", err)
		}
		cfg.Mode = ModeNVIDIA
		cfg.Enabled = true

	case "intel":
		if err := detector.ValidateIntelSetup(); err != nil {
			return nil, fmt.Errorf("Intel mode requested but validation failed: %w\n\nTo fix:\n  Ensure i915 or xe kernel module is loaded:\n  - lsmod | grep -E 'i915|xe'\n\nOr use: nova start --gpu=nvidia (for NVIDIA GPU)", err)
		}
		cfg.Mode = ModeIntel
		cfg.Enabled = true

	default:
		return nil, fmt.Errorf("unsupported GPU mode: %s (supported: auto, nvidia, intel)", requestedMode)
	}

	return cfg, nil
}
