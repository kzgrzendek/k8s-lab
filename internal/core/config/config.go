// Package config handles NOVA configuration management using Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

// GPUModeType represents the GPU acceleration mode for NOVA deployments.
// Empty string means CPU-only mode (the default).
type GPUModeType string

const (
	// GPUModeNVIDIA enables NVIDIA GPU acceleration via CUDA.
	// Requires NVIDIA GPU with proper drivers installed on the host.
	GPUModeNVIDIA GPUModeType = "nvidia"
)

// ResourceProfileType represents the deployment profile for NOVA.
// The profile determines cluster topology (nodes, CPUs, RAM) and resource requests.
type ResourceProfileType string

const (
	// ResourceProfileMinimal is for single-node deployments.
	// Topology: 1 node, 6 CPUs, 12GB RAM.
	// Supports both GPU and CPU inference modes.
	// Uses low resource requests and no limits (best-effort QoS).
	ResourceProfileMinimal ResourceProfileType = "minimal"

	// ResourceProfileCluster is for multi-node lab environments.
	// Topology: 3 nodes, 4 CPUs / 4GB RAM per node.
	// Requires GPU mode (CPU inference not supported - RAM distributed across nodes).
	// Uses conservative resource requests with limits.
	ResourceProfileCluster ResourceProfileType = "cluster"
)

// AppProfileType identifies an application profile by name.
// App profiles determine which Tier 3 applications are deployed.
type AppProfileType string

const (
	// AppProfileOpenWebUI deploys Open WebUI as the chat interface.
	AppProfileOpenWebUI AppProfileType = "openwebui"

	// AppProfileLaSuite deploys the French government AI stack (OpenGateLLM + Conversations).
	AppProfileLaSuite AppProfileType = "lasuite"

	// AppProfileLab deploys HELIX JupyterHub for ML development.
	AppProfileLab AppProfileType = "lab"
)

// AppDefinition defines a single deployable application.
type AppDefinition struct {
	Name         string         `json:"name" yaml:"name"`                                     // Unique identifier (e.g., "openwebui", "helix")
	Enabled      bool           `json:"enabled" yaml:"enabled"`                               // Whether this app should be deployed
	ChartConfig  Tier3AppConfig `json:"chart" yaml:"chart"`                                   // Helm chart configuration
	Namespace    string         `json:"namespace" yaml:"namespace"`                           // Kubernetes namespace to deploy to
	Dependencies []string       `json:"dependencies,omitempty" yaml:"dependencies,omitempty"` // Apps that must be deployed first
}

// AppProfile defines a named collection of apps to deploy.
type AppProfile struct {
	Name        AppProfileType `json:"name" yaml:"name"`               // Profile identifier
	Description string         `json:"description" yaml:"description"` // Human-readable description
	Apps        []string       `json:"apps" yaml:"apps"`               // List of app names to deploy
}

// AppProfilesConfig holds all app profile configuration.
type AppProfilesConfig struct {
	ActiveProfiles []AppProfileType              `json:"activeProfiles" yaml:"activeProfiles"` // Which profiles are currently enabled
	Profiles       map[AppProfileType]AppProfile `json:"profiles" yaml:"profiles"`             // Available profiles (built-in + custom)
	Apps           map[string]AppDefinition      `json:"apps" yaml:"apps"`                     // All available app definitions
}

// NodeLabel returns the Kubernetes node label value for this GPU mode.
// Used for node selection in deployments (e.g., nova.local/node-type=gpu-nvidia).
func (m GPUModeType) NodeLabel() string {
	switch m {
	case GPUModeNVIDIA:
		return "gpu-nvidia"
	default:
		return "cpu" // Empty/unknown = CPU mode
	}
}

// MinikubeConfig holds Minikube cluster settings.
// Note: CPUs, Memory, and Nodes are derived from the ResourceProfile.
// Use Config.GetNodes(), Config.GetCPUs(), Config.GetMemory() to get topology values.
type MinikubeConfig struct {
	KubernetesVersion string      `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	Driver            string      `json:"driver" yaml:"driver"`
	GPUMode           GPUModeType `json:"gpuMode,omitempty" yaml:"gpuMode,omitempty"` // GPU mode: empty (CPU) or "nvidia"
}

// DNSConfig holds DNS settings.
type DNSConfig struct {
	Domain     string `json:"domain" yaml:"domain"`
	AuthDomain string `json:"authDomain" yaml:"authDomain"`
	Bind9Port  int    `json:"bind9Port" yaml:"bind9Port"`
}

// StateConfig holds runtime state.
type StateConfig struct {
	Initialized      bool                `json:"initialized" yaml:"initialized"`
	LastDeployedTier int                 `json:"lastDeployedTier" yaml:"lastDeployedTier"`
	DeployedProfile  ResourceProfileType `json:"deployedProfile,omitempty" yaml:"deployedProfile,omitempty"` // Profile used when cluster was created
	DeployedGPUMode  GPUModeType         `json:"deployedGPUMode,omitempty" yaml:"deployedGPUMode,omitempty"` // GPU mode used when cluster was created
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
	NodeFeatureDiscovery         ChartVersion `json:"nodeFeatureDiscovery" yaml:"nodeFeatureDiscovery"`
	IntelDevicePluginsOperator   ChartVersion `json:"intelDevicePluginsOperator" yaml:"intelDevicePluginsOperator"`
	IntelGPUPlugin               ChartVersion `json:"intelGpuPlugin" yaml:"intelGpuPlugin"`
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

	// Infrastructure operators
	CNPGOperator  string       `json:"cnpgOperator" yaml:"cnpgOperator"`   // CloudNative PG operator version (e.g., "1.25.0")
	RedisOperator ChartVersion `json:"redisOperator" yaml:"redisOperator"` // OT-CONTAINER-KIT Redis operator
	Garage        ChartVersion `json:"garage" yaml:"garage"`               // Garage S3-compatible storage
}

// LLMDImageConfig holds the image and tag for a specific GPU profile.
type LLMDImageConfig struct {
	Image string `json:"image" yaml:"image"` // Container image (e.g., "ghcr.io/llm-d/llm-d-cuda")
	Tag   string `json:"tag" yaml:"tag"`     // Image tag (e.g., "v0.4.0")
}

// FullImage returns the full image reference (image:tag).
func (c LLMDImageConfig) FullImage() string {
	return fmt.Sprintf("%s:%s", c.Image, c.Tag)
}

// Tier3Versions holds all tier 3 (AI workloads) dependency versions.
type Tier3Versions struct {
	LLMD          Tier3AppConfig             `json:"llmd" yaml:"llmd"`
	InferencePool Tier3AppConfig             `json:"inferencePool" yaml:"inferencePool"`
	OpenWebUI     Tier3AppConfig             `json:"openWebui" yaml:"openWebui"`
	Helix         Tier3AppConfig             `json:"helix" yaml:"helix"`
	OpenGateLLM   Tier3AppConfig             `json:"openGateLLM" yaml:"openGateLLM"`     // LaSuite: OpenGateLLM stack
	Conversations Tier3AppConfig             `json:"conversations" yaml:"conversations"` // LaSuite: Conversations app
	Qdrant        Tier3AppConfig             `json:"qdrant" yaml:"qdrant"`               // LaSuite: Qdrant vector database
	LLMDImages    map[string]LLMDImageConfig `json:"llmdImages" yaml:"llmdImages"`       // vLLM images by GPU profile (nvidia, intel, cpu)
}

// Versions holds all external dependency versions for reproducible deployments.
type Versions struct {
	Kubernetes string        `json:"kubernetes" yaml:"kubernetes"` // Kubernetes version for minikube
	Tier1      Tier1Versions `json:"tier1" yaml:"tier1"`
	Tier2      Tier2Versions `json:"tier2" yaml:"tier2"`
	Tier3      Tier3Versions `json:"tier3" yaml:"tier3"`
}

// ResourceSpec defines CPU and memory resources for a container.
type ResourceSpec struct {
	CPU    string `json:"cpu,omitempty" yaml:"cpu,omitempty"`       // CPU (e.g., "100m", "2")
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"` // Memory (e.g., "256Mi", "2Gi")
}

// ResourceRequirements defines requests and limits for a container.
type ResourceRequirements struct {
	Requests ResourceSpec `json:"requests,omitempty" yaml:"requests,omitempty"`
	Limits   ResourceSpec `json:"limits,omitempty" yaml:"limits,omitempty"`
}

// ComponentResources defines resources for a single component.
type ComponentResources struct {
	Resources ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// ResourcesConfig holds all component resource configurations.
// All fields are optional - if not set, chart defaults are used.
type ResourcesConfig struct {
	// Tier 1 components
	Cilium       ComponentResources `json:"cilium,omitempty" yaml:"cilium,omitempty"`
	CertManager  ComponentResources `json:"certManager,omitempty" yaml:"certManager,omitempty"`
	TrustManager ComponentResources `json:"trustManager,omitempty" yaml:"trustManager,omitempty"`
	EnvoyGateway ComponentResources `json:"envoyGateway,omitempty" yaml:"envoyGateway,omitempty"`
	Redis        ComponentResources `json:"redis,omitempty" yaml:"redis,omitempty"`
	Falco        ComponentResources `json:"falco,omitempty" yaml:"falco,omitempty"`

	// Tier 2 components
	Kyverno         ComponentResources `json:"kyverno,omitempty" yaml:"kyverno,omitempty"`
	Keycloak        ComponentResources `json:"keycloak,omitempty" yaml:"keycloak,omitempty"`
	PostgreSQL      ComponentResources `json:"postgresql,omitempty" yaml:"postgresql,omitempty"`
	VictoriaMetrics ComponentResources `json:"victoriaMetrics,omitempty" yaml:"victoriaMetrics,omitempty"`
	VictoriaLogs    ComponentResources `json:"victoriaLogs,omitempty" yaml:"victoriaLogs,omitempty"`
	Grafana         ComponentResources `json:"grafana,omitempty" yaml:"grafana,omitempty"`
	Hubble          ComponentResources `json:"hubble,omitempty" yaml:"hubble,omitempty"`

	// Tier 3 components
	LLMDNvidia ComponentResources `json:"llmdNvidia,omitempty" yaml:"llmdNvidia,omitempty"` // LLMD with NVIDIA GPU
	LLMDCPU    ComponentResources `json:"llmdCpu,omitempty" yaml:"llmdCpu,omitempty"`       // LLMD with CPU only
	OpenWebUI  ComponentResources `json:"openWebUI,omitempty" yaml:"openWebUI,omitempty"`
	Helix      ComponentResources `json:"helix,omitempty" yaml:"helix,omitempty"`
}

// Config represents the NOVA configuration.
type Config struct {
	Minikube        MinikubeConfig      `json:"minikube" yaml:"minikube"`
	DNS             DNSConfig           `json:"dns" yaml:"dns"`
	State           StateConfig         `json:"state" yaml:"state"`
	Performance     PerformanceConfig   `json:"performance" yaml:"performance"`
	LLM             LLMConfig           `json:"llm" yaml:"llm"`
	Versions        Versions            `json:"versions" yaml:"versions"`
	ResourceProfile ResourceProfileType `json:"resourceProfile,omitempty" yaml:"resourceProfile,omitempty"` // Resource profile: minimal, cluster
	Resources       ResourcesConfig     `json:"resources,omitempty" yaml:"resources,omitempty"`             // Optional resource limits per component (overrides profile)
	AppProfiles     AppProfilesConfig   `json:"appProfiles,omitempty" yaml:"appProfiles,omitempty"`         // App profiles for Tier 3 application selection
}

// GetEffectiveResourceProfile returns the resource profile to use.
// Defaults to "minimal" if not set.
func (c *Config) GetEffectiveResourceProfile() ResourceProfileType {
	if c.ResourceProfile == "" {
		return ResourceProfileMinimal
	}
	return c.ResourceProfile
}

// ProfileTopology returns the cluster topology for the given profile.
// Returns (nodes, cpusPerNode, memoryMBPerNode).
func ProfileTopology(profile ResourceProfileType) (nodes, cpus, memoryMB int) {
	switch profile {
	case ResourceProfileCluster:
		return 3, 4, 4096 // 3 nodes, 4 CPUs, 4GB each
	default: // ResourceProfileMinimal
		return 1, 6, 12288 // 1 node, 6 CPUs, 12GB
	}
}

// GetNodes returns the number of nodes based on the resource profile.
func (c *Config) GetNodes() int {
	nodes, _, _ := ProfileTopology(c.GetEffectiveResourceProfile())
	return nodes
}

// GetCPUs returns CPUs per node based on the resource profile.
func (c *Config) GetCPUs() int {
	_, cpus, _ := ProfileTopology(c.GetEffectiveResourceProfile())
	return cpus
}

// GetMemory returns memory (MB) per node based on the resource profile.
// For minimal profile with GPU mode, returns 6GB (inference uses VRAM).
// For minimal profile with CPU mode, returns 12GB (inference uses RAM).
func (c *Config) GetMemory() int {
	_, _, mem := ProfileTopology(c.GetEffectiveResourceProfile())
	// In minimal profile with GPU, reduce RAM since VRAM handles inference
	if c.GetEffectiveResourceProfile() == ResourceProfileMinimal && c.IsGPUMode() {
		return 6144 // 6GB is enough when using GPU VRAM
	}
	return mem
}

// GetResourcesWithDefaults returns the configured resources merged with profile defaults.
// If a component's resources are not configured, profile defaults are used.
// Profile selection: minimal (single-node) or cluster (multi-node).
func (c *Config) GetResourcesWithDefaults() ResourcesConfig {
	// Select base resources based on effective profile
	var base ResourcesConfig
	switch c.GetEffectiveResourceProfile() {
	case ResourceProfileMinimal:
		base = MinimalResources()
	case ResourceProfileCluster:
		base = DefaultResources() // "cluster" uses the standard/default resources
	default:
		base = MinimalResources() // Fallback to minimal
	}

	// Helper to merge component resources (user config overrides profile defaults)
	merge := func(configured, profileDefault ComponentResources) ComponentResources {
		result := profileDefault
		if configured.Resources.Requests.CPU != "" {
			result.Resources.Requests.CPU = configured.Resources.Requests.CPU
		}
		if configured.Resources.Requests.Memory != "" {
			result.Resources.Requests.Memory = configured.Resources.Requests.Memory
		}
		if configured.Resources.Limits.CPU != "" {
			result.Resources.Limits.CPU = configured.Resources.Limits.CPU
		}
		if configured.Resources.Limits.Memory != "" {
			result.Resources.Limits.Memory = configured.Resources.Limits.Memory
		}
		return result
	}

	return ResourcesConfig{
		// Tier 1
		Cilium:       merge(c.Resources.Cilium, base.Cilium),
		CertManager:  merge(c.Resources.CertManager, base.CertManager),
		TrustManager: merge(c.Resources.TrustManager, base.TrustManager),
		EnvoyGateway: merge(c.Resources.EnvoyGateway, base.EnvoyGateway),
		Redis:        merge(c.Resources.Redis, base.Redis),
		Falco:        merge(c.Resources.Falco, base.Falco),
		// Tier 2
		Kyverno:         merge(c.Resources.Kyverno, base.Kyverno),
		Keycloak:        merge(c.Resources.Keycloak, base.Keycloak),
		PostgreSQL:      merge(c.Resources.PostgreSQL, base.PostgreSQL),
		VictoriaMetrics: merge(c.Resources.VictoriaMetrics, base.VictoriaMetrics),
		VictoriaLogs:    merge(c.Resources.VictoriaLogs, base.VictoriaLogs),
		Grafana:         merge(c.Resources.Grafana, base.Grafana),
		Hubble:          merge(c.Resources.Hubble, base.Hubble),
		// Tier 3
		LLMDNvidia: merge(c.Resources.LLMDNvidia, base.LLMDNvidia),
		LLMDCPU:    merge(c.Resources.LLMDCPU, base.LLMDCPU),
		OpenWebUI:  merge(c.Resources.OpenWebUI, base.OpenWebUI),
		Helix:      merge(c.Resources.Helix, base.Helix),
	}
}

// DefaultResources returns the default ResourcesConfig with conservative values
// suitable for dev/lab environments. These values are based on official documentation
// but scaled down for resource-constrained environments.
func DefaultResources() ResourcesConfig {
	return ResourcesConfig{
		// Tier 1 components
		Cilium: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "128Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "256Mi"},
			},
		},
		CertManager: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "32Mi"},
				Limits:   ResourceSpec{CPU: "100m", Memory: "64Mi"},
			},
		},
		TrustManager: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "10m", Memory: "16Mi"},
				Limits:   ResourceSpec{CPU: "50m", Memory: "32Mi"},
			},
		},
		EnvoyGateway: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "128Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "256Mi"},
			},
		},
		Redis: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "64Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "128Mi"},
			},
		},
		Falco: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "128Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "256Mi"},
			},
		},

		// Tier 2 components
		Kyverno: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "128Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "256Mi"},
			},
		},
		Keycloak: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "100m", Memory: "256Mi"},
				Limits:   ResourceSpec{CPU: "500m", Memory: "768Mi"},
			},
		},
		PostgreSQL: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "128Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "256Mi"},
			},
		},
		VictoriaMetrics: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "128Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "512Mi"},
			},
		},
		VictoriaLogs: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "64Mi"},
				Limits:   ResourceSpec{CPU: "100m", Memory: "128Mi"},
			},
		},
		Grafana: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "64Mi"},
				Limits:   ResourceSpec{CPU: "200m", Memory: "128Mi"},
			},
		},
		Hubble: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "32Mi"},
				Limits:   ResourceSpec{CPU: "100m", Memory: "64Mi"},
			},
		},

		// Tier 3 components
		LLMDNvidia: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "250m", Memory: "512Mi"},
				Limits:   ResourceSpec{CPU: "1000m", Memory: "2Gi"},
			},
		},
		LLMDCPU: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "4000m", Memory: "12Gi"},
				Limits:   ResourceSpec{CPU: "8000m", Memory: "24Gi"},
			},
		},
		OpenWebUI: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "100m", Memory: "1Gi"},
				Limits:   ResourceSpec{CPU: "2000m", Memory: "2Gi"},
			},
		},
		Helix: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "100m", Memory: "256Mi"},
				Limits:   ResourceSpec{CPU: "500m", Memory: "1Gi"},
			},
		},
	}
}

// MinimalResources returns a ResourcesConfig optimized for single-node, resource-constrained environments.
// Target: 4-6 CPUs, 8-12GB RAM total.
// Uses very low requests and no limits to allow Kubernetes best-effort scheduling.
// Some optional components (Falco, Hubble, Kyverno) should be disabled for this profile.
func MinimalResources() ResourcesConfig {
	return ResourcesConfig{
		// Tier 1 components - minimal footprint
		Cilium: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "64Mi"},
				// No limits - allow burst when needed
			},
		},
		CertManager: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "10m", Memory: "24Mi"},
			},
		},
		TrustManager: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "5m", Memory: "12Mi"},
			},
		},
		EnvoyGateway: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "64Mi"},
			},
		},
		Redis: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "32Mi"},
			},
		},
		// Falco - recommend disabling in minimal profile
		Falco: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "64Mi"},
			},
		},

		// Tier 2 components - minimal footprint
		// Kyverno - recommend disabling in minimal profile
		Kyverno: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "64Mi"},
			},
		},
		Keycloak: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "192Mi"},
			},
		},
		PostgreSQL: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "64Mi"},
			},
		},
		VictoriaMetrics: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "64Mi"},
			},
		},
		VictoriaLogs: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "10m", Memory: "32Mi"},
			},
		},
		Grafana: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "25m", Memory: "48Mi"},
			},
		},
		// Hubble - recommend disabling in minimal profile
		Hubble: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "10m", Memory: "24Mi"},
			},
		},

		// Tier 3 components - LLM workloads
		LLMDNvidia: ComponentResources{
			Resources: ResourceRequirements{
				// GPU mode: most work done on GPU, low CPU/RAM needed
				Requests: ResourceSpec{CPU: "100m", Memory: "256Mi"},
			},
		},
		LLMDCPU: ComponentResources{
			Resources: ResourceRequirements{
				// CPU mode: needs significant resources for inference
				// Scaled down for minimal profile - will be slow but functional
				Requests: ResourceSpec{CPU: "2000m", Memory: "6Gi"},
			},
		},
		OpenWebUI: ComponentResources{
			Resources: ResourceRequirements{
				// OpenWebUI needs ~1-2GB memory
				Requests: ResourceSpec{CPU: "100m", Memory: "1Gi"},
				// No limits in minimal profile - allow burst up to 2Gi
			},
		},
		Helix: ComponentResources{
			Resources: ResourceRequirements{
				Requests: ResourceSpec{CPU: "50m", Memory: "128Mi"},
			},
		},
	}
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
// Default profile is "minimal" (single-node with 6 CPUs and 12GB RAM).
// Default mode is CPU (no GPU acceleration).
func Default() *Config {
	return &Config{
		ResourceProfile: ResourceProfileMinimal,
		Minikube: MinikubeConfig{
			KubernetesVersion: "v1.33.5",
			Driver:            "docker",
			// GPUMode empty = CPU mode (default)
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
				NodeFeatureDiscovery: ChartVersion{
					Chart:   "nfd/node-feature-discovery",
					Version: "0.18.1",
				},
				IntelDevicePluginsOperator: ChartVersion{
					Chart:   "intel/intel-device-plugins-operator",
					Version: "0.34.1",
				},
				IntelGPUPlugin: ChartVersion{
					Chart:   "intel/intel-device-plugins-gpu",
					Version: "0.34.1",
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
				// Infrastructure operators
				CNPGOperator: "1.25.0", // CloudNative PG operator
				RedisOperator: ChartVersion{
					Chart:   "ot-helm/redis-operator",
					Version: "0.18.5",
				},
				Garage: ChartVersion{
					Chart:   "garage/garage",
					Version: "1.0.0",
				},
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
				OpenGateLLM: Tier3AppConfig{
					ChartVersion: ChartVersion{
						Chart:   "etalab-ia/opengatellm-stack",
						Version: "0.1.0",
					},
				},
				Conversations: Tier3AppConfig{
					ChartVersion: ChartVersion{
						Chart:   "suitenumerique/conversations",
						Version: "0.1.0",
					},
				},
				Qdrant: Tier3AppConfig{
					ChartVersion: ChartVersion{
						Chart:   "qdrant/qdrant",
						Version: "1.13.4",
					},
				},
				// LLMDImages: vLLM container images by GPU profile
				// Currently supported: nvidia, cpu
				// Planned: intel (IPEX-LLM), amd (ROCm)
				LLMDImages: map[string]LLMDImageConfig{
					"nvidia": {Image: "ghcr.io/llm-d/llm-d-cuda", Tag: "v0.4.0"},
					"cpu":    {Image: "ghcr.io/llm-d/llm-d-cpu", Tag: "v0.4.0"},
					// Reserved for future GPU support:
					// "intel": {Image: "intelanalytics/ipex-llm-serving-xpu", Tag: "latest"},
					// "amd":   {Image: "rocm/vllm", Tag: "latest"},
				},
			},
		},
		// AppProfiles: Built-in app profiles for Tier 3 application selection.
		// Users can override or extend these in their config file.
		AppProfiles: AppProfilesConfig{
			// Default: deploy OpenWebUI and Lab (HELIX) profiles
			ActiveProfiles: []AppProfileType{AppProfileOpenWebUI, AppProfileLab},
			// Built-in profile definitions
			Profiles: map[AppProfileType]AppProfile{
				AppProfileOpenWebUI: {
					Name:        AppProfileOpenWebUI,
					Description: "Chat interface with Open WebUI",
					Apps:        []string{"openwebui"},
				},
				AppProfileLaSuite: {
					Name:        AppProfileLaSuite,
					Description: "French government AI stack (OpenGateLLM + Conversations)",
					Apps:        []string{"opengatellm", "conversations"},
				},
				AppProfileLab: {
					Name:        AppProfileLab,
					Description: "JupyterHub for ML development (HELIX)",
					Apps:        []string{"helix"},
				},
			},
			// Built-in app definitions
			Apps: map[string]AppDefinition{
				"openwebui": {
					Name:      "openwebui",
					Enabled:   true,
					Namespace: "openwebui",
					ChartConfig: Tier3AppConfig{
						ChartVersion: ChartVersion{
							Chart:   "open-webui/open-webui",
							Version: "9.0.0",
						},
					},
				},
				"helix": {
					Name:      "helix",
					Enabled:   true,
					Namespace: "helix",
					ChartConfig: Tier3AppConfig{
						ChartVersion: ChartVersion{
							Chart:   "aphp-helix/helix",
							Version: "1.3.1",
						},
					},
				},
				"opengatellm": {
					Name:      "opengatellm",
					Enabled:   true,
					Namespace: "opengatellm",
					ChartConfig: Tier3AppConfig{
						ChartVersion: ChartVersion{
							Chart:   "etalab-ia/opengatellm-stack",
							Version: "0.1.0",
						},
					},
				},
				"conversations": {
					Name:         "conversations",
					Enabled:      true,
					Namespace:    "conversations",
					Dependencies: []string{"opengatellm"},
					ChartConfig: Tier3AppConfig{
						ChartVersion: ChartVersion{
							Chart:   "suitenumerique/conversations",
							Version: "0.1.0",
						},
					},
				},
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
	defaultCfg := Default()

	// Check if Versions section is missing or incomplete
	if cfg.Versions.Kubernetes == "" {
		// Populate with defaults from Default()
		cfg.Versions = defaultCfg.Versions

		// Preserve existing Kubernetes version if set in Minikube config
		if cfg.Minikube.KubernetesVersion != "" {
			cfg.Versions.Kubernetes = cfg.Minikube.KubernetesVersion
		}

		migrated = true
	}

	// Migrate Node Feature Discovery field (added in GPU refactoring for Intel support)
	if cfg.Versions.Tier1.NodeFeatureDiscovery.Chart == "" {
		cfg.Versions.Tier1.NodeFeatureDiscovery = defaultCfg.Versions.Tier1.NodeFeatureDiscovery
		migrated = true
	}

	// Migrate Intel Device Plugins Operator field (added in GPU refactoring)
	if cfg.Versions.Tier1.IntelDevicePluginsOperator.Chart == "" {
		cfg.Versions.Tier1.IntelDevicePluginsOperator = defaultCfg.Versions.Tier1.IntelDevicePluginsOperator
		migrated = true
	}

	// Migrate Intel GPU Plugin field (added in GPU refactoring)
	if cfg.Versions.Tier1.IntelGPUPlugin.Chart == "" {
		cfg.Versions.Tier1.IntelGPUPlugin = defaultCfg.Versions.Tier1.IntelGPUPlugin
		migrated = true
	}

	// Migrate AppProfiles - add if missing (added in app profiles feature)
	if len(cfg.AppProfiles.Profiles) == 0 {
		cfg.AppProfiles = defaultCfg.AppProfiles
		migrated = true
	}

	// Migrate OpenGateLLM and Conversations versions (added in app profiles feature)
	if cfg.Versions.Tier3.OpenGateLLM.Chart == "" {
		cfg.Versions.Tier3.OpenGateLLM = defaultCfg.Versions.Tier3.OpenGateLLM
		migrated = true
	}
	if cfg.Versions.Tier3.Conversations.Chart == "" {
		cfg.Versions.Tier3.Conversations = defaultCfg.Versions.Tier3.Conversations
		migrated = true
	}

	// Migrate infrastructure operators (added in LaSuite infrastructure feature)
	if cfg.Versions.Tier2.CNPGOperator == "" {
		cfg.Versions.Tier2.CNPGOperator = defaultCfg.Versions.Tier2.CNPGOperator
		migrated = true
	}
	if cfg.Versions.Tier2.RedisOperator.Chart == "" {
		cfg.Versions.Tier2.RedisOperator = defaultCfg.Versions.Tier2.RedisOperator
		migrated = true
	}
	if cfg.Versions.Tier2.Garage.Chart == "" {
		cfg.Versions.Tier2.Garage = defaultCfg.Versions.Tier2.Garage
		migrated = true
	}
	if cfg.Versions.Tier3.Qdrant.Chart == "" {
		cfg.Versions.Tier3.Qdrant = defaultCfg.Versions.Tier3.Qdrant
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
	// Validate profile
	profile := c.GetEffectiveResourceProfile()
	if profile != ResourceProfileMinimal && profile != ResourceProfileCluster {
		return fmt.Errorf("invalid resource profile: %s (must be 'minimal' or 'cluster')", profile)
	}

	// Validate DNS port
	if c.DNS.Bind9Port < 1024 || c.DNS.Bind9Port > 65535 {
		return fmt.Errorf("bind9 port must be between 1024-65535 (got %d)", c.DNS.Bind9Port)
	}

	// CPU mode requires minimal profile (single-node)
	// CPU inference needs ~6GB+ RAM for the model, which doesn't work well
	// when RAM is distributed across multiple nodes in cluster mode
	if !c.IsNVIDIAMode() && profile == ResourceProfileCluster {
		return fmt.Errorf("CPU inference mode requires 'minimal' profile (single-node); cluster profile needs --gpu=nvidia")
	}

	return nil
}

// ValidateConfigChange checks if a configuration change is allowed.
// Returns an error if trying to change profile or GPU mode on an existing cluster.
// newProfile and newGPUMode are the proposed new settings.
func (c *Config) ValidateConfigChange(newProfile ResourceProfileType, newGPUMode GPUModeType) error {
	// No deployed cluster - any change is allowed
	if c.State.LastDeployedTier == 0 {
		return nil
	}

	// Check for profile change
	if c.State.DeployedProfile != "" && c.State.DeployedProfile != newProfile {
		return fmt.Errorf(
			"cannot change profile from '%s' to '%s' on existing cluster\n"+
				"  Run 'nova delete' first, then run setup again",
			c.State.DeployedProfile, newProfile)
	}

	// Check for GPU mode change
	if c.State.DeployedGPUMode != newGPUMode {
		oldMode := "cpu"
		if c.State.DeployedGPUMode != "" {
			oldMode = string(c.State.DeployedGPUMode)
		}
		newMode := "cpu"
		if newGPUMode != "" {
			newMode = string(newGPUMode)
		}
		return fmt.Errorf(
			"cannot change GPU mode from '%s' to '%s' on existing cluster\n"+
				"  Run 'nova delete' first, then run setup again",
			oldMode, newMode)
	}

	return nil
}

// ValidateResourcesForMode returns informational messages about the configuration.
// Returns a list of messages (empty if no special notes).
func (c *Config) ValidateResourcesForMode() []string {
	var messages []string
	profile := c.GetEffectiveResourceProfile()

	// Inform about profile topology
	nodes, cpus, mem := ProfileTopology(profile)
	switch profile {
	case ResourceProfileMinimal:
		messages = append(messages, fmt.Sprintf(
			"using 'minimal' profile: %d node, %d CPUs, %dMB RAM (low requests, no limits)",
			nodes, cpus, mem))
	case ResourceProfileCluster:
		messages = append(messages, fmt.Sprintf(
			"using 'cluster' profile: %d nodes, %d CPUs/node, %dMB RAM/node",
			nodes, cpus, mem))
	}

	return messages
}

// GetGPUMode returns the configured GPU mode.
// Empty string means CPU-only mode (the default).
func (c *Config) GetGPUMode() GPUModeType {
	return c.Minikube.GPUMode
}

// IsNVIDIAMode returns true if NVIDIA GPU mode is enabled.
func (c *Config) IsNVIDIAMode() bool {
	return c.Minikube.GPUMode == GPUModeNVIDIA
}

// IsCPUMode returns true if CPU-only mode is active (no GPU).
// This is the default when GPUMode is empty.
func (c *Config) IsCPUMode() bool {
	return c.Minikube.GPUMode != GPUModeNVIDIA
}

// IsGPUMode returns true if GPU acceleration is enabled.
// Currently only NVIDIA is supported.
func (c *Config) IsGPUMode() bool {
	return c.IsNVIDIAMode()
}

// HasGPU is an alias for IsGPUMode for backward compatibility.
func (c *Config) HasGPU() bool {
	return c.IsGPUMode()
}

// WorkerNodes returns the number of worker nodes (total nodes - 1 master).
func (c *Config) WorkerNodes() int {
	nodes := c.GetNodes()
	if nodes <= 1 {
		return 0
	}
	return nodes - 1
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

// GetCNPGOperatorManifestURL returns the URL for the CloudNative PG Operator manifest.
func (c *Config) GetCNPGOperatorManifestURL() string {
	version := c.Versions.Tier2.CNPGOperator
	// Extract major.minor for branch name (e.g., "1.25.0" -> "1.25")
	parts := strings.Split(version, ".")
	branchVersion := version
	if len(parts) >= 2 {
		branchVersion = parts[0] + "." + parts[1]
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-%s/releases/cnpg-%s.yaml",
		branchVersion, version)
}

// InfraRequirements tracks which infrastructure components are needed based on active profiles.
type InfraRequirements struct {
	CNPGOperator  bool // CloudNative PG Operator (always needed - Keycloak uses it)
	RedisOperator bool // OT Redis Operator (always needed - Envoy Gateway uses it)
	Garage        bool // Garage S3 (conditional - LaSuite profile)
	Qdrant        bool // Qdrant vector DB (conditional - LaSuite profile)
}

// GetInfraRequirements returns infrastructure requirements based on active profiles.
func (c *Config) GetInfraRequirements() InfraRequirements {
	req := InfraRequirements{
		CNPGOperator:  true, // Always needed (Keycloak uses CNPG)
		RedisOperator: true, // Always needed (Envoy Gateway uses Redis Operator)
	}

	// Check if LaSuite profile is active
	for _, profile := range c.GetActiveAppProfiles() {
		if profile == AppProfileLaSuite {
			req.Garage = true
			req.Qdrant = true
			break
		}
	}

	return req
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

// GetLLMDImage returns the full vLLM/llm-d image reference for the given GPU profile.
// Currently supported profiles: "nvidia", "cpu"
// Planned profiles: "intel", "amd"
func (c *Config) GetLLMDImage(profile string) string {
	defaults := map[string]LLMDImageConfig{
		"nvidia": {Image: "ghcr.io/llm-d/llm-d-cuda", Tag: "v0.4.0"},
		"cpu":    {Image: "ghcr.io/llm-d/llm-d-cpu", Tag: "v0.4.0"},
		// Reserved for future GPU support (images defined but not yet tested):
		// "intel": {Image: "intelanalytics/ipex-llm-serving-xpu", Tag: "latest"},
		// "amd":   {Image: "rocm/vllm", Tag: "latest"},
	}

	// Check if configured in config
	if c.Versions.Tier3.LLMDImages != nil {
		if img, ok := c.Versions.Tier3.LLMDImages[profile]; ok {
			return img.FullImage()
		}
	}

	// Fall back to defaults
	if img, ok := defaults[profile]; ok {
		return img.FullImage()
	}

	// Ultimate fallback to CPU (safest option)
	return defaults["cpu"].FullImage()
}

// GetModelsPath returns the host path where LLM models are stored.
// This directory is mounted into minikube nodes via minikube mount.
// Path: ~/.nova/share/models/
func (c *Config) GetModelsPath() string {
	return filepath.Join(ConfigDir(), "share", "models")
}

// GetModelPath returns the full path to a specific model on the host.
// Path: ~/.nova/share/models/{model-slug}/
func (c *Config) GetModelPath(modelSlug string) string {
	return filepath.Join(c.GetModelsPath(), modelSlug)
}

// GetActiveAppProfiles returns the list of active app profiles.
// Defaults to ["openwebui", "lab"] if not configured.
func (c *Config) GetActiveAppProfiles() []AppProfileType {
	if len(c.AppProfiles.ActiveProfiles) == 0 {
		return []AppProfileType{AppProfileOpenWebUI, AppProfileLab}
	}
	return c.AppProfiles.ActiveProfiles
}

// SetActiveAppProfiles sets the active app profiles.
func (c *Config) SetActiveAppProfiles(profiles []AppProfileType) {
	c.AppProfiles.ActiveProfiles = profiles
}

// GetAppsToDeployFromProfiles returns a deduplicated, dependency-sorted list of apps
// based on active profiles. Returns an error if a profile or app is not found.
func (c *Config) GetAppsToDeployFromProfiles() ([]AppDefinition, error) {
	activeProfiles := c.GetActiveAppProfiles()

	// Collect all unique app names from active profiles
	appSet := make(map[string]bool)
	for _, profileName := range activeProfiles {
		profile, exists := c.AppProfiles.Profiles[profileName]
		if !exists {
			return nil, fmt.Errorf("unknown app profile: %s", profileName)
		}
		for _, appName := range profile.Apps {
			appSet[appName] = true
		}
	}

	// Build dependency-sorted list
	return c.resolveAppDependencies(appSet)
}

// resolveAppDependencies performs topological sort on apps to ensure dependencies come first.
func (c *Config) resolveAppDependencies(appSet map[string]bool) ([]AppDefinition, error) {
	// Build app list and check all exist
	apps := make(map[string]AppDefinition)
	for appName := range appSet {
		app, exists := c.AppProfiles.Apps[appName]
		if !exists {
			return nil, fmt.Errorf("unknown app: %s", appName)
		}
		apps[appName] = app
	}

	// Topological sort using Kahn's algorithm
	// Count incoming edges (dependencies)
	inDegree := make(map[string]int)
	for appName := range apps {
		inDegree[appName] = 0
	}

	// For each app, count how many of its dependencies are in our set
	for appName, app := range apps {
		for _, dep := range app.Dependencies {
			if _, inSet := appSet[dep]; inSet {
				inDegree[appName]++
			}
		}
	}

	// Start with apps that have no dependencies in our set
	var queue []string
	for appName, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, appName)
		}
	}

	// Process queue
	var result []AppDefinition
	for len(queue) > 0 {
		// Pop from queue
		appName := queue[0]
		queue = queue[1:]

		app := apps[appName]
		result = append(result, app)

		// For all apps that depend on this one, decrement their in-degree
		for otherName, otherApp := range apps {
			for _, dep := range otherApp.Dependencies {
				if dep == appName {
					inDegree[otherName]--
					if inDegree[otherName] == 0 {
						queue = append(queue, otherName)
					}
				}
			}
		}
	}

	// Check for cycles
	if len(result) != len(apps) {
		return nil, fmt.Errorf("circular dependency detected in app profiles")
	}

	return result, nil
}

// IsAppEnabled checks if an app should be deployed based on active profiles.
func (c *Config) IsAppEnabled(appName string) bool {
	apps, err := c.GetAppsToDeployFromProfiles()
	if err != nil {
		return false
	}
	for _, app := range apps {
		if app.Name == appName {
			return true
		}
	}
	return false
}

// ValidateAppProfile checks if a profile name is valid.
func (c *Config) ValidateAppProfile(profile AppProfileType) error {
	if _, exists := c.AppProfiles.Profiles[profile]; !exists {
		available := make([]string, 0, len(c.AppProfiles.Profiles))
		for name := range c.AppProfiles.Profiles {
			available = append(available, string(name))
		}
		return fmt.Errorf("unknown profile '%s', available: %v", profile, available)
	}
	return nil
}

// GetAppDefinition returns the app definition for a given app name.
func (c *Config) GetAppDefinition(appName string) (AppDefinition, bool) {
	app, exists := c.AppProfiles.Apps[appName]
	return app, exists
}

// LogDir returns the directory where log files are stored.
// Path: ~/.nova/logs/
func LogDir() string {
	return filepath.Join(ConfigDir(), "logs")
}

// LogFilePath returns the path to a log file with the given name.
// Creates the log directory if it doesn't exist.
// Path: ~/.nova/logs/{name}.log
func LogFilePath(name string) string {
	logDir := LogDir()
	os.MkdirAll(logDir, 0755)
	return filepath.Join(logDir, name+".log")
}
