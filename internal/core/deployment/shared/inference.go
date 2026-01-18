package shared

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/core/config"
	"sigs.k8s.io/yaml"
)

// GetLLMDValuesPath returns the appropriate llm-d values file path based on GPU mode.
func GetLLMDValuesPath(cfg *config.Config) string {
	switch cfg.GetGPUMode() {
	case config.GPUModeNVIDIA:
		return "resources/core/deployment/tier3/llmd/helm/llmd-cuda.yaml"
	case config.GPUModeIntel:
		return "resources/core/deployment/tier3/llmd/helm/llmd-intel.yaml"
	case config.GPUModeCPU:
		return "resources/core/deployment/tier3/llmd/helm/llmd-cpu.yaml"
	default:
		// Default to Intel for auto mode (should be resolved before reaching here)
		return "resources/core/deployment/tier3/llmd/helm/llmd-intel.yaml"
	}
}

// LLMDValues represents the structure of llm-d Helm values (partial, only what we need).
type LLMDValues struct {
	Decode struct {
		Containers []struct {
			Name  string `yaml:"name"`
			Image string `yaml:"image"`
		} `yaml:"containers"`
	} `yaml:"decode"`
}

// GetLLMDImage reads the llm-d values file and extracts the vllm container image.
// This ensures the prepull mechanism uses the same image version as the actual deployment.
// The values file is treated as a template and rendered with model configuration before parsing.
func GetLLMDImage(cfg *config.Config) (string, error) {
	valuesPath := GetLLMDValuesPath(cfg)

	// Prepare template data with model configuration
	templateData := map[string]string{
		"ModelName": cfg.GetModelName(),
		"ModelSlug": cfg.GetModelSlug(),
	}

	// Render template
	rendered, err := RenderTemplate(valuesPath, templateData)
	if err != nil {
		return "", err
	}

	// Parse rendered YAML
	var values LLMDValues
	if err := yaml.Unmarshal([]byte(rendered), &values); err != nil {
		return "", fmt.Errorf("failed to parse rendered llmd values from %s: %w", valuesPath, err)
	}

	// Find vllm container
	for _, container := range values.Decode.Containers {
		if container.Name == "vllm" {
			if container.Image == "" {
				return "", fmt.Errorf("vllm container image is empty in %s", valuesPath)
			}
			return container.Image, nil
		}
	}

	return "", fmt.Errorf("vllm container not found in %s", valuesPath)
}
