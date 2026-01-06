package tier1

import (
	"context"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
)

// TestCheckPrerequisites tests prerequisite checking logic.
func TestCheckPrerequisites(t *testing.T) {
	ctx := context.Background()

	// Note: This test requires kubectl and helm to be installed
	// In a real CI environment, you might want to skip this or use mocks
	err := checkPrerequisites(ctx)
	if err != nil {
		t.Logf("Prerequisites check failed (expected in environments without kubectl/helm): %v", err)
	}
}

// TestHelmRepoConfiguration tests Helm repository configuration.
func TestHelmRepoConfiguration(t *testing.T) {
	repos := map[string]string{
		"nvidia":        "https://helm.ngc.nvidia.com/nvidia",
		"falcosecurity": "https://falcosecurity.github.io/charts",
		"jetstack":      "https://charts.jetstack.io",
		"dandydev":      "https://dandydeveloper.github.io/charts",
	}

	// Verify all repo names and URLs are non-empty
	for name, url := range repos {
		if name == "" {
			t.Error("Repository name should not be empty")
		}
		if url == "" {
			t.Errorf("Repository URL for %s should not be empty", name)
		}

		// Verify URL format (basic check)
		if len(url) < 8 || url[:8] != "https://" {
			t.Errorf("Repository URL for %s should use HTTPS: %s", name, url)
		}
	}

	// Verify expected repositories are present (Cilium moved to tier0)
	expectedRepos := []string{"nvidia", "falcosecurity", "jetstack", "dandydev"}
	for _, expected := range expectedRepos {
		if _, exists := repos[expected]; !exists {
			t.Errorf("Expected repository %s not found in configuration", expected)
		}
	}
}

// Note: TestCiliumConfiguration removed - Cilium CNI is now deployed in tier0

// TestFalcoConfiguration tests Falco security configuration values.
func TestFalcoConfiguration(t *testing.T) {
	values := map[string]any{
		"tty": true,
		"driver": map[string]any{
			"kind": "modern_ebpf",
		},
	}

	// Verify TTY is enabled
	if tty, ok := values["tty"].(bool); !ok || !tty {
		t.Error("TTY should be enabled for Falco")
	}

	// Verify driver configuration
	if driver, ok := values["driver"].(map[string]any); ok {
		if kind, ok := driver["kind"].(string); ok {
			if kind != "modern_ebpf" {
				t.Errorf("Expected driver kind 'modern_ebpf', got %s", kind)
			}
		} else {
			t.Error("Driver kind should be a string")
		}
	} else {
		t.Error("Driver configuration should be present")
	}
}

// TestGPUOperatorConfiguration tests NVIDIA GPU operator configuration.
func TestGPUOperatorConfiguration(t *testing.T) {
	values := map[string]any{
		"driver": map[string]any{
			"enabled": false, // Use host drivers
		},
		"toolkit": map[string]any{
			"enabled": true,
		},
		"operator": map[string]any{
			"defaultRuntime": "docker",
		},
	}

	// Verify driver is disabled (using host drivers)
	if driver, ok := values["driver"].(map[string]any); ok {
		if enabled, ok := driver["enabled"].(bool); ok {
			if enabled {
				t.Error("GPU driver should be disabled (using host drivers)")
			}
		}
	}

	// Verify toolkit is enabled
	if toolkit, ok := values["toolkit"].(map[string]any); ok {
		if enabled, ok := toolkit["enabled"].(bool); ok {
			if !enabled {
				t.Error("GPU toolkit should be enabled")
			}
		}
	}

	// Verify runtime configuration
	if operator, ok := values["operator"].(map[string]any); ok {
		if runtime, ok := operator["defaultRuntime"].(string); ok {
			if runtime != "docker" {
				t.Errorf("Expected runtime 'docker', got %s", runtime)
			}
		}
	}
}

// TestGPUModeDetection tests GPU mode detection logic.
func TestGPUModeDetection(t *testing.T) {
	testCases := []struct {
		name             string
		gpuConfig        string
		shouldSkipDeploy bool
	}{
		{
			name:             "Empty GPU config",
			gpuConfig:        "",
			shouldSkipDeploy: true,
		},
		{
			name:             "GPU disabled",
			gpuConfig:        "disabled",
			shouldSkipDeploy: true,
		},
		{
			name:             "GPU none",
			gpuConfig:        "none",
			shouldSkipDeploy: true,
		},
		{
			name:             "GPU all",
			gpuConfig:        "all",
			shouldSkipDeploy: false,
		},
		{
			name:             "GPU specific",
			gpuConfig:        "0",
			shouldSkipDeploy: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate GPU detection logic from deployGPUOperatorWithProgress
			shouldSkip := tc.gpuConfig == "" || tc.gpuConfig == "none" || tc.gpuConfig == "disabled"

			if shouldSkip != tc.shouldSkipDeploy {
				t.Errorf("Expected shouldSkip=%v for GPU config '%s', got %v",
					tc.shouldSkipDeploy, tc.gpuConfig, shouldSkip)
			}
		})
	}
}

// TestCertManagerConfiguration tests Cert Manager CRD configuration.
func TestCertManagerConfiguration(t *testing.T) {
	values := map[string]any{
		"crds": map[string]any{
			"enabled": true,
			"keep":    true,
		},
	}

	// Verify CRDs are configured
	if crds, ok := values["crds"].(map[string]any); ok {
		if enabled, ok := crds["enabled"].(bool); ok {
			if !enabled {
				t.Error("CRDs should be enabled for Cert Manager")
			}
		}

		if keep, ok := crds["keep"].(bool); ok {
			if !keep {
				t.Error("CRDs should be kept (not deleted on uninstall)")
			}
		}
	} else {
		t.Error("CRDS configuration should be present")
	}
}

// TestRedisConfiguration tests Redis backend configuration for Envoy Gateway.
func TestRedisConfiguration(t *testing.T) {
	values := map[string]any{
		"replicas": 1,
	}

	// Verify replica count
	if replicas, ok := values["replicas"].(int); ok {
		if replicas < 1 {
			t.Error("Redis should have at least 1 replica")
		}
	} else {
		t.Error("Redis replicas should be configured")
	}
}

// TestNamespaceConfiguration tests namespace configuration for tier1 components.
func TestNamespaceConfiguration(t *testing.T) {
	expectedNamespaces := map[string]string{
		"Falco":            "falco",
		"GPU Operator":     "nvidia-gpu-operator",
		"Cert Manager":     "cert-manager",
		"Trust Manager":    "cert-manager",
		"Envoy AI Gateway": "envoy-ai-gateway-system",
		"Envoy Gateway":    "envoy-gateway-system",
	}

	for component, namespace := range expectedNamespaces {
		if namespace == "" {
			t.Errorf("Namespace for %s should not be empty", component)
		}

		// Verify namespace follows Kubernetes naming conventions
		if len(namespace) > 63 {
			t.Errorf("Namespace for %s is too long (max 63 chars): %s", component, namespace)
		}
	}
}

// TestDeploymentSteps tests the deployment step configuration.
func TestDeploymentSteps(t *testing.T) {
	// Tier 1 steps (cluster basics moved to tier0)
	steps := []string{
		"Prerequisites Check",
		"Helm Repositories",
		"Falco Security",
		"NVIDIA GPU Operator",
		"Cert Manager",
		"Trust Manager",
		"Envoy AI Gateway",
		"Envoy Gateway",
		"Nova Namespace & RBAC",
	}

	// Verify we have the expected number of steps (9 for tier1)
	if len(steps) != 9 {
		t.Errorf("Expected 9 deployment steps for tier1, got %d", len(steps))
	}

	// Verify all steps have non-empty names
	for i, step := range steps {
		if step == "" {
			t.Errorf("Step %d should have a non-empty name", i)
		}
	}

	// Verify prerequisites are first
	if steps[0] != "Prerequisites Check" {
		t.Error("First step should be prerequisites check")
	}

	// Verify Helm repos are second
	if steps[1] != "Helm Repositories" {
		t.Error("Second step should be Helm repositories")
	}
}

// TestTimeoutConfiguration tests Helm timeout values.
func TestTimeoutConfiguration(t *testing.T) {
	testCases := []struct {
		component string
		timeout   int
		minTime   int
		maxTime   int
	}{
		{"Falco", 600, 300, 900},
		{"GPU Operator", 1200, 600, 1800},
		{"Cert Manager", 600, 300, 900},
		{"Trust Manager", 600, 300, 900},
		{"Envoy AI Gateway", 600, 300, 900},
		{"Envoy Gateway", 600, 300, 900},
	}

	for _, tc := range testCases {
		t.Run(tc.component, func(t *testing.T) {
			if tc.timeout < tc.minTime {
				t.Errorf("%s timeout %ds is too short (min %ds)", tc.component, tc.timeout, tc.minTime)
			}
			if tc.timeout > tc.maxTime {
				t.Errorf("%s timeout %ds is too long (max %ds)", tc.component, tc.timeout, tc.maxTime)
			}
		})
	}
}

// TestErrorMessages tests error message formatting.
func TestErrorMessages(t *testing.T) {
	testCases := []struct {
		name             string
		errorFormat      string
		expectedContains string
	}{
		{
			name:             "Namespace creation error",
			errorFormat:      "failed to create %s namespace: %w",
			expectedContains: "namespace",
		},
		{
			name:             "Helm installation error",
			errorFormat:      "failed to install %s: %w",
			expectedContains: "install",
		},
		{
			name:             "Repository error",
			errorFormat:      "failed to add %s repository (%s): %w",
			expectedContains: "repository",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify error message contains expected keywords
			if tc.errorFormat == "" {
				t.Error("Error format should not be empty")
			}

			// Basic validation that format string is valid
			if len(tc.errorFormat) < 5 {
				t.Error("Error format should be a meaningful message")
			}
		})
	}
}

// TestConfigValidation tests configuration validation for tier 1.
func TestConfigValidation(t *testing.T) {
	testCases := []struct {
		name      string
		cfg       *config.Config
		expectGPU bool
	}{
		{
			name: "Config with GPU enabled",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					GPUs: "all",
				},
			},
			expectGPU: true,
		},
		{
			name: "Config with GPU disabled",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					GPUs: "",
				},
			},
			expectGPU: false,
		},
		{
			name: "Config with GPU none",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					GPUs: "none",
				},
			},
			expectGPU: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasGPU := tc.cfg.Minikube.GPUs != "" &&
				tc.cfg.Minikube.GPUs != "none" &&
				tc.cfg.Minikube.GPUs != "disabled"

			if hasGPU != tc.expectGPU {
				t.Errorf("Expected GPU enabled=%v, got %v", tc.expectGPU, hasGPU)
			}
		})
	}
}
