// Package config handles NOVA configuration management using Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// MinikubeConfig holds Minikube cluster settings.
type MinikubeConfig struct {
	CPUs              int    `json:"cpus" yaml:"cpus"`
	Memory            int    `json:"memory" yaml:"memory"`
	Nodes             int    `json:"nodes" yaml:"nodes"` // Total nodes (1 master + N workers)
	KubernetesVersion string `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	Driver            string `json:"driver" yaml:"driver"`
	GPUs              string `json:"gpus" yaml:"gpus"`                   // GPU passthrough mode ("all", "none", "disabled")
	CPUModeForced     bool   `json:"cpuModeForced" yaml:"cpuModeForced"` // Force CPU mode even if GPU available
}

// DNSConfig holds DNS settings.
type DNSConfig struct {
	Domain     string `json:"domain" yaml:"domain"`
	AuthDomain string `json:"authDomain" yaml:"authDomain"`
	Bind9Port  int    `json:"bind9Port" yaml:"bind9Port"`
}

// StateConfig holds runtime state.
type StateConfig struct {
	Initialized      bool `json:"initialized" yaml:"initialized"`
	LastDeployedTier int  `json:"lastDeployedTier" yaml:"lastDeployedTier"`
}

// PerformanceConfig holds performance optimization settings.
type PerformanceConfig struct {
	MaxConcurrentDownloads int  `json:"maxConcurrentDownloads" yaml:"maxConcurrentDownloads"` // Docker max concurrent layer downloads (default: 3)
	UseSkopeo              bool `json:"useSkopeo" yaml:"useSkopeo"`                           // Use skopeo for image pulls when available (default: true)
}

// LLMConfig holds LLM-related settings.
type LLMConfig struct {
	Model   string `json:"model" yaml:"model"`     // Hugging Face model to serve (e.g., "Qwen/Qwen3-0.6B", "google/gemma-3-4b-it")
	HfToken string `json:"hfToken" yaml:"hfToken"` // Optional Hugging Face token for model downloads
}

// ChartVersion represents a versioned Helm chart reference.
type ChartVersion struct {
	Chart   string `json:"chart" yaml:"chart"`     // Chart reference (e.g., "cilium/cilium" or "oci://...")
	Version string `json:"version" yaml:"version"` // Chart version
}

// Tier3AppConfig extends ChartVersion with custom values path support.
type Tier3AppConfig struct {
	ChartVersion     `yaml:",inline"`
	CustomValuesPath string `json:"customValuesPath,omitempty" yaml:"customValuesPath,omitempty"` // Optional: override default values file
}

// Tier1Versions holds all tier 1 (infrastructure) dependency versions.
type Tier1Versions struct {
	Cilium                       ChartVersion `json:"cilium" yaml:"cilium"`
	Falco                        ChartVersion `json:"falco" yaml:"falco"`
	GPUOperator                  ChartVersion `json:"gpuOperator" yaml:"gpuOperator"`
	CertManager                  ChartVersion `json:"certManager" yaml:"certManager"`
	TrustManager                 ChartVersion `json:"trustManager" yaml:"trustManager"`
	EnvoyAiGatewayCRDs           ChartVersion `json:"envoyAiGatewayCRDs" yaml:"envoyAiGatewayCRDs"`
	EnvoyAiGateway               ChartVersion `json:"envoyAiGateway" yaml:"envoyAiGateway"`
	Redis                        ChartVersion `json:"redis" yaml:"redis"`
	EnvoyGateway                 ChartVersion `json:"envoyGateway" yaml:"envoyGateway"`
	LocalPathProvisioner         string       `json:"localPathProvisioner" yaml:"localPathProvisioner"`
	GatewayAPIInferenceExtension string       `json:"gatewayApiInferenceExtension" yaml:"gatewayApiInferenceExtension"`
}

// Tier2Versions holds all tier 2 (platform services) dependency versions.
type Tier2Versions struct {
	Kyverno               ChartVersion `json:"kyverno" yaml:"kyverno"`
	Hubble                ChartVersion `json:"hubble" yaml:"hubble"` // Uses Cilium chart
	VictoriaLogsSingle    ChartVersion `json:"victoriaLogsSingle" yaml:"victoriaLogsSingle"`
	VictoriaLogsCollector ChartVersion `json:"victoriaLogsCollector" yaml:"victoriaLogsCollector"`
	VictoriaMetricsStack  ChartVersion `json:"victoriaMetricsStack" yaml:"victoriaMetricsStack"`
	KeycloakOperator      string       `json:"keycloakOperator" yaml:"keycloakOperator"`
}

// Tier3Versions holds all tier 3 (AI workloads) dependency versions.
type Tier3Versions struct {
	LLMD         Tier3AppConfig `json:"llmd" yaml:"llmd"`
	InferencePool Tier3AppConfig `json:"inferencePool" yaml:"inferencePool"`
	OpenWebUI    Tier3AppConfig `json:"openWebui" yaml:"openWebui"`
	Helix        Tier3AppConfig `json:"helix" yaml:"helix"`
	LLMDImageTag string         `json:"llmdImageTag" yaml:"llmdImageTag"` // llm-d image tag (e.g., "v0.4.0"), used for both CPU and CUDA variants
}

// Versions holds all external dependency versions for reproducible deployments.
type Versions struct {
	Kubernetes string        `json:"kubernetes" yaml:"kubernetes"` // Kubernetes version for minikube
	Tier1      Tier1Versions `json:"tier1" yaml:"tier1"`
	Tier2      Tier2Versions `json:"tier2" yaml:"tier2"`
	Tier3      Tier3Versions `json:"tier3" yaml:"tier3"`
}

// Config represents the NOVA configuration.
type Config struct {
	Minikube    MinikubeConfig    `json:"minikube" yaml:"minikube"`
	DNS         DNSConfig         `json:"dns" yaml:"dns"`
	State       StateConfig       `json:"state" yaml:"state"`
	Performance PerformanceConfig `json:"performance" yaml:"performance"`
	LLM         LLMConfig         `json:"llm" yaml:"llm"`
	Versions    Versions          `json:"versions" yaml:"versions"`
}

// ConfigDir returns the NOVA configuration directory.
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nova"
	}
	return filepath.Join(home, ".nova")
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Minikube: MinikubeConfig{
			CPUs:              4,
			Memory:            4096,
			Nodes:             3,
			KubernetesVersion: "v1.33.5",
			Driver:            "docker",
			GPUs:              "all",
			CPUModeForced:     false, // Auto-detect GPU by default
		},
		DNS: DNSConfig{
			Domain:     "nova.local",
			AuthDomain: "auth.local",
			Bind9Port:  30053,
		},
		State: StateConfig{
			Initialized:      false,
			LastDeployedTier: 0,
		},
		Performance: PerformanceConfig{
			MaxConcurrentDownloads: 10,   // Increased from Docker default of 3
			UseSkopeo:              true, // Use skopeo when available for faster pulls
		},
		LLM: LLMConfig{
			Model:   "Qwen/Qwen3-0.6B", // Default model
			HfToken: "",                // Empty by default
		},
		Versions: Versions{
			Kubernetes: "v1.33.5",
			Tier1: Tier1Versions{
				Cilium: ChartVersion{
					Chart:   "cilium/cilium",
					Version: "1.18.5",
				},
				Falco: ChartVersion{
					Chart:   "falcosecurity/falco",
					Version: "7.0.2",
				},
				GPUOperator: ChartVersion{
					Chart:   "nvidia/gpu-operator",
					Version: "v25.10.1",
				},
				CertManager: ChartVersion{
					Chart:   "jetstack/cert-manager",
					Version: "v1.19.2",
				},
				TrustManager: ChartVersion{
					Chart:   "jetstack/trust-manager",
					Version: "v0.20.3",
				},
				EnvoyAiGatewayCRDs: ChartVersion{
					Chart:   "oci://docker.io/envoyproxy/ai-gateway-crds-helm",
					Version: "v0.4.0",
				},
				EnvoyAiGateway: ChartVersion{
					Chart:   "oci://docker.io/envoyproxy/ai-gateway-helm",
					Version: "v0.4.0",
				},
				Redis: ChartVersion{
					Chart:   "dandydev/redis-ha",
					Version: "4.35.5",
				},
				EnvoyGateway: ChartVersion{
					Chart:   "oci://docker.io/envoyproxy/gateway-helm",
					Version: "v1.6.1",
				},
				LocalPathProvisioner:         "v0.0.33",
				GatewayAPIInferenceExtension: "v1.2.1",
			},
			Tier2: Tier2Versions{
				Kyverno: ChartVersion{
					Chart:   "kyverno/kyverno",
					Version: "3.6.1",
				},
				Hubble: ChartVersion{
					Chart:   "cilium/cilium",
					Version: "1.18.5",
				},
				VictoriaLogsSingle: ChartVersion{
					Chart:   "vm/victoria-logs-single",
					Version: "0.11.23",
				},
				VictoriaLogsCollector: ChartVersion{
					Chart:   "vm/victoria-logs-collector",
					Version: "0.2.4",
				},
				VictoriaMetricsStack: ChartVersion{
					Chart:   "vm/victoria-metrics-k8s-stack",
					Version: "0.66.1",
				},
				KeycloakOperator: "26.4.7",
			},
			Tier3: Tier3Versions{
				LLMD: Tier3AppConfig{
					ChartVersion: ChartVersion{
						Chart:   "llm-d-modelservice/llm-d-modelservice",
						Version: "v0.3.17",
					},
				},
				InferencePool: Tier3AppConfig{
					ChartVersion: ChartVersion{
						Chart:   "oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool",
						Version: "v1.2.1",
					},
				},
				OpenWebUI: Tier3AppConfig{
					ChartVersion: ChartVersion{
						Chart:   "open-webui/open-webui",
						Version: "9.0.0",
					},
				},
				Helix: Tier3AppConfig{
					ChartVersion: ChartVersion{
						Chart:   "aphp-helix/helix",
						Version: "1.3.1",
					},
				},
				LLMDImageTag: "v0.4.0",
			},
		},
	}
}

// Load reads the configuration from disk.
func Load() (*Config, error) {
	configPath := DefaultConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found at %s", configPath)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Migrate config if needed
	if err := migrateConfig(cfg); err != nil {
		return nil, fmt.Errorf("migrate config: %w", err)
	}

	return cfg, nil
}

// migrateConfig migrates old config files to the new format.
// This ensures backward compatibility when new fields are added.
func migrateConfig(cfg *Config) error {
	migrated := false

	// Check if Versions section is missing or incomplete
	if cfg.Versions.Kubernetes == "" {
		// Populate with defaults from Default()
		defaultCfg := Default()
		cfg.Versions = defaultCfg.Versions

		// Preserve existing Kubernetes version if set in Minikube config
		if cfg.Minikube.KubernetesVersion != "" {
			cfg.Versions.Kubernetes = cfg.Minikube.KubernetesVersion
		}

		migrated = true
	}

	// Save migrated config
	if migrated {
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save migrated config: %w", err)
		}
	}

	return nil
}

// LoadOrDefault loads config from disk, or returns defaults if not found.
func LoadOrDefault() *Config {
	cfg, err := Load()
	if err != nil {
		return Default()
	}
	return cfg
}

// Save writes the configuration to disk.
func (c *Config) Save() error {
	configDir := ConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	configPath := DefaultConfigPath()
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate node count
	if c.Minikube.Nodes < 1 {
		return fmt.Errorf("minikube nodes must be at least 1 (got %d)", c.Minikube.Nodes)
	}
	if c.Minikube.Nodes > 10 {
		return fmt.Errorf("minikube nodes should not exceed 10 (got %d)", c.Minikube.Nodes)
	}

	// Validate CPUs
	if c.Minikube.CPUs < 2 {
		return fmt.Errorf("minikube cpus must be at least 2 (got %d)", c.Minikube.CPUs)
	}

	// Validate Memory
	if c.Minikube.Memory < 2048 {
		return fmt.Errorf("minikube memory must be at least 2048MB (got %d)", c.Minikube.Memory)
	}

	// Validate DNS port
	if c.DNS.Bind9Port < 1024 || c.DNS.Bind9Port > 65535 {
		return fmt.Errorf("bind9 port must be between 1024-65535 (got %d)", c.DNS.Bind9Port)
	}

	return nil
}

// HasGPU returns true if GPU support is enabled.
func (c *Config) HasGPU() bool {
	return c.Minikube.GPUs != "" && c.Minikube.GPUs != "none" && c.Minikube.GPUs != "disabled"
}

// IsGPUMode returns true if GPU mode should be used for deployments.
// GPU mode is enabled only when:
// - Host has GPU configured (GPUs != "" && != "none" && != "disabled")
// - AND CPU mode is NOT forced
func (c *Config) IsGPUMode() bool {
	return c.HasGPU() && !c.Minikube.CPUModeForced
}

// WorkerNodes returns the number of worker nodes (total nodes - 1 master).
func (c *Config) WorkerNodes() int {
	if c.Minikube.Nodes <= 1 {
		return 0
	}
	return c.Minikube.Nodes - 1
}

// ChartRef returns the full chart reference with version.
// For OCI charts, the version is already embedded in the chart URL.
// For non-OCI charts, version must be passed separately to Helm.
func (cv *ChartVersion) ChartRef() string {
	return cv.Chart
}

// GetVersion returns the chart version.
func (cv *ChartVersion) GetVersion() string {
	return cv.Version
}

// GetLocalPathProvisionerManifestURL returns the URL for the local-path-provisioner manifest.
func (c *Config) GetLocalPathProvisionerManifestURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/rancher/local-path-provisioner/%s/deploy/local-path-storage.yaml", c.Versions.Tier1.LocalPathProvisioner)
}

// GetGatewayAPIInferenceExtensionManifestURL returns the URL for the Gateway API Inference Extension CRDs.
func (c *Config) GetGatewayAPIInferenceExtensionManifestURL() string {
	return fmt.Sprintf("https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/%s/manifests.yaml", c.Versions.Tier1.GatewayAPIInferenceExtension)
}

// GetKeycloakCRDManifestURL returns the URL for the Keycloak CRD manifest.
func (c *Config) GetKeycloakCRDManifestURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/%s/kubernetes/keycloaks.k8s.keycloak.org-v1.yml", c.Versions.Tier2.KeycloakOperator)
}

// GetKeycloakRealmImportCRDManifestURL returns the URL for the Keycloak RealmImport CRD manifest.
func (c *Config) GetKeycloakRealmImportCRDManifestURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/%s/kubernetes/keycloakrealmimports.k8s.keycloak.org-v1.yml", c.Versions.Tier2.KeycloakOperator)
}

// GetKeycloakOperatorManifestURL returns the URL for the Keycloak Operator manifest.
func (c *Config) GetKeycloakOperatorManifestURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/%s/kubernetes/kubernetes.yml", c.Versions.Tier2.KeycloakOperator)
}

// GetModelSlug returns a Kubernetes-safe slug from the model name (e.g., "Qwen/Qwen3-0.6B" -> "qwen3-0-6b").
func (c *Config) GetModelSlug() string {
	model := c.LLM.Model
	if model == "" {
		return "qwen3-0-6b"
	}

	// Extract model name after slash
	if idx := strings.LastIndex(model, "/"); idx != -1 {
		model = model[idx+1:]
	}

	// Convert to lowercase and normalize separators
	model = strings.ToLower(model)
	model = strings.ReplaceAll(model, ".", "-")
	model = strings.ReplaceAll(model, "_", "-")

	// Keep only alphanumeric and dashes
	var result strings.Builder
	for _, r := range model {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	slug := result.String()
	if slug == "" {
		return "unknown-model"
	}

	return slug
}

// GetModelName returns the full Hugging Face model name.
func (c *Config) GetModelName() string {
	if c.LLM.Model == "" {
		return "Qwen/Qwen3-0.6B" // Default fallback
	}
	return c.LLM.Model
}

// GetModelURI returns the Hugging Face URI for the model.
func (c *Config) GetModelURI() string {
	return fmt.Sprintf("hf://%s", c.GetModelName())
}

// GetLLMDImageTag returns the llm-d image tag with a fallback to default.
func (c *Config) GetLLMDImageTag() string {
	if c.Versions.Tier3.LLMDImageTag == "" {
		return "v0.4.0"
	}
	return c.Versions.Tier3.LLMDImageTag
}
