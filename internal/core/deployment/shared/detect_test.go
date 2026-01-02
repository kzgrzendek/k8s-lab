package shared

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeDisabled, "disabled"},
		{ModeNVIDIA, "nvidia"},
		{ModeAMD, "amd"},
		{ModeIntel, "intel"},
		{Mode(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestNewDetector(t *testing.T) {
	ctx := context.Background()
	detector := NewDetector(ctx)

	assert.NotNil(t, detector)
	assert.Equal(t, ctx, detector.ctx)
}

func TestDetector_DetectMode(t *testing.T) {
	ctx := context.Background()
	detector := NewDetector(ctx)

	// This test will vary based on the system
	// Just ensure it doesn't panic and returns a valid mode
	mode, err := detector.DetectMode()
	assert.NoError(t, err)
	assert.True(t, mode == ModeDisabled || mode == ModeNVIDIA || mode == ModeAMD || mode == ModeIntel)
}

func TestDetector_HasNVIDIAGPU(t *testing.T) {
	ctx := context.Background()
	detector := NewDetector(ctx)

	// This test will vary based on the system
	// Just ensure it doesn't panic
	hasGPU, err := detector.HasNVIDIAGPU()
	assert.NoError(t, err)
	assert.IsType(t, false, hasGPU)
}

func TestDetector_HasNVIDIARuntime(t *testing.T) {
	ctx := context.Background()
	detector := NewDetector(ctx)

	// This test will vary based on the system
	// Just ensure it handles errors gracefully
	hasRuntime, err := detector.HasNVIDIARuntime()

	// If docker is not available, we expect an error
	// If docker is available, we expect a boolean result
	if err != nil {
		assert.Contains(t, err.Error(), "docker")
	} else {
		assert.IsType(t, false, hasRuntime)
	}
}

func TestDetector_ValidateNVIDIASetup(t *testing.T) {
	ctx := context.Background()
	detector := NewDetector(ctx)

	// This test will vary based on the system
	// If NVIDIA GPU is not available, it should return an error
	err := detector.ValidateNVIDIASetup()

	// We expect either no error (if NVIDIA is fully set up)
	// or an error explaining what's missing
	if err != nil {
		assert.Contains(t, err.Error(), "NVIDIA")
	}
}

func TestDetector_GetNVIDIAGPUInfo(t *testing.T) {
	ctx := context.Background()
	detector := NewDetector(ctx)

	// This test will vary based on the system
	gpus, err := detector.GetNVIDIAGPUInfo()

	// If nvidia-smi is not available, we expect an error
	if err != nil {
		assert.Contains(t, err.Error(), "GPU info")
	} else {
		// If successful, gpus should be a slice (possibly empty)
		assert.IsType(t, []string{}, gpus)
	}
}

func TestGetGPUConfig_Disabled(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		requestedMode string
	}{
		{"empty string", ""},
		{"none", "none"},
		{"disabled", "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := GetGPUConfig(ctx, tt.requestedMode)
			assert.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, ModeDisabled, cfg.Mode)
			assert.False(t, cfg.Enabled)
		})
	}
}

func TestGetGPUConfig_Auto(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		requestedMode string
	}{
		{"auto", "auto"},
		{"all", "all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := GetGPUConfig(ctx, tt.requestedMode)

			// Auto mode should not error - it detects what's available
			assert.NoError(t, err)
			assert.NotNil(t, cfg)

			// The mode should be either disabled (no GPU) or a valid GPU mode
			assert.True(t, cfg.Mode == ModeDisabled || cfg.Mode == ModeNVIDIA)

			// If mode is disabled, enabled should be false
			if cfg.Mode == ModeDisabled {
				assert.False(t, cfg.Enabled)
			}
		})
	}
}

func TestGetGPUConfig_ExplicitNVIDIA(t *testing.T) {
	ctx := context.Background()

	cfg, err := GetGPUConfig(ctx, "nvidia")

	// If NVIDIA is not available, we expect an error
	// If NVIDIA is available, we expect a valid config
	if err != nil {
		assert.Contains(t, err.Error(), "NVIDIA")
		assert.Contains(t, err.Error(), "validation failed")
	} else {
		assert.NotNil(t, cfg)
		assert.Equal(t, ModeNVIDIA, cfg.Mode)
		assert.True(t, cfg.Enabled)
	}
}

func TestGetGPUConfig_UnsupportedMode(t *testing.T) {
	ctx := context.Background()

	cfg, err := GetGPUConfig(ctx, "unsupported")

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "unsupported GPU mode")
	assert.Contains(t, err.Error(), "unsupported")
}

func TestConfig_Scenarios(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantStr string
	}{
		{
			name:    "disabled config",
			config:  &Config{Mode: ModeDisabled, Enabled: false},
			wantStr: "disabled",
		},
		{
			name:    "nvidia enabled",
			config:  &Config{Mode: ModeNVIDIA, Enabled: true},
			wantStr: "nvidia",
		},
		{
			name:    "amd config",
			config:  &Config{Mode: ModeAMD, Enabled: true},
			wantStr: "amd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantStr, tt.config.Mode.String())
		})
	}
}
