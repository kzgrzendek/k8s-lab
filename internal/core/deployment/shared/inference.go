package shared

import (
	"fmt"
	"os"

	"github.com/kzgrzendek/nova/internal/core/config"
	"sigs.k8s.io/yaml"
)

// GetLLMDValuesPath returns the appropriate llm-d values file path based on GPU/CPU mode.
func GetLLMDValuesPath(cfg *config.Config) string {
	if cfg.IsGPUMode() {
		return "resources/core/deployment/tier3/llmd/helm/llmd-cuda.yaml"
	}
	return "resources/core/deployment/tier3/llmd/helm/llmd-cpu.yaml"
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
func GetLLMDImage(cfg *config.Config) (string, error) {
	valuesPath := GetLLMDValuesPath(cfg)

	// Read values file
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read llmd values file %s: %w", valuesPath, err)
	}

	// Parse YAML
	var values LLMDValues
	if err := yaml.Unmarshal(data, &values); err != nil {
		return "", fmt.Errorf("failed to parse llmd values file %s: %w", valuesPath, err)
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
