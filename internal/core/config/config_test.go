package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Default: minimal profile (single-node with 6 CPUs and 12GB RAM), CPU mode
	assert.Equal(t, ResourceProfileMinimal, cfg.ResourceProfile)
	assert.Equal(t, 6, cfg.GetCPUs())
	assert.Equal(t, 12288, cfg.GetMemory())
	assert.Equal(t, 1, cfg.GetNodes())
	assert.Equal(t, "v1.33.5", cfg.Minikube.KubernetesVersion)
	assert.Equal(t, "docker", cfg.Minikube.Driver)
	assert.Empty(t, cfg.Minikube.GPUMode) // Empty = CPU mode (default)
	assert.True(t, cfg.IsCPUMode())

	assert.Equal(t, "nova.local", cfg.DNS.Domain)
	assert.Equal(t, "auth.local", cfg.DNS.AuthDomain)
	assert.Equal(t, 30053, cfg.DNS.Bind9Port)

	assert.False(t, cfg.State.Initialized)
	assert.Equal(t, 0, cfg.State.LastDeployedTier)

	assert.Equal(t, "", cfg.LLM.HfToken)
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create and save config
	cfg := Default()
	cfg.ResourceProfile = ResourceProfileCluster
	cfg.State.Initialized = true

	err := cfg.Save()
	require.NoError(t, err)

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".nova", "config.yaml")
	assert.FileExists(t, configPath)

	// Load and verify
	loaded, err := Load()
	require.NoError(t, err)

	assert.Equal(t, ResourceProfileCluster, loaded.ResourceProfile)
	assert.Equal(t, 3, loaded.GetNodes())
	assert.True(t, loaded.State.Initialized)
}

func TestLoadNotFound(t *testing.T) {
	// Create temp directory with no config
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLoadOrDefault(t *testing.T) {
	// Create temp directory with no config
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Should return defaults when no config exists
	cfg := LoadOrDefault()
	assert.Equal(t, 6, cfg.GetCPUs())
	assert.False(t, cfg.State.Initialized)
}

func TestGPUModeType_NodeLabel(t *testing.T) {
	tests := []struct {
		mode     GPUModeType
		expected string
	}{
		{GPUModeNVIDIA, "gpu-nvidia"},
		{"", "cpu"},        // Empty = CPU mode
		{"unknown", "cpu"}, // Unknown mode falls back to CPU
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.NodeLabel())
		})
	}
}

func TestConfig_GetGPUMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     GPUModeType
		expected GPUModeType
	}{
		{"nvidia mode", GPUModeNVIDIA, GPUModeNVIDIA},
		{"empty is CPU mode", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Minikube: MinikubeConfig{GPUMode: tt.mode}}
			assert.Equal(t, tt.expected, cfg.GetGPUMode())
		})
	}
}

func TestConfig_GPUModeHelpers(t *testing.T) {
	tests := []struct {
		name      string
		mode      GPUModeType
		isNVIDIA  bool
		isCPU     bool
		isGPUMode bool
	}{
		{"nvidia", GPUModeNVIDIA, true, false, true},
		{"empty (cpu)", "", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Minikube: MinikubeConfig{GPUMode: tt.mode}}
			assert.Equal(t, tt.isNVIDIA, cfg.IsNVIDIAMode(), "IsNVIDIAMode")
			assert.Equal(t, tt.isCPU, cfg.IsCPUMode(), "IsCPUMode")
			assert.Equal(t, tt.isGPUMode, cfg.IsGPUMode(), "IsGPUMode")
			assert.Equal(t, tt.isGPUMode, cfg.HasGPU(), "HasGPU (alias)")
		})
	}
}

func TestValidateResourcesForMode(t *testing.T) {
	tests := []struct {
		name            string
		profile         ResourceProfileType
		expectedContain string
	}{
		{
			name:            "minimal profile returns info message",
			profile:         ResourceProfileMinimal,
			expectedContain: "minimal",
		},
		{
			name:            "cluster profile returns info message",
			profile:         ResourceProfileCluster,
			expectedContain: "cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ResourceProfile: tt.profile}
			messages := cfg.ValidateResourcesForMode()
			assert.NotEmpty(t, messages, "expected info messages")
			assert.Contains(t, messages[0], tt.expectedContain)
		})
	}
}

func TestGetEffectiveResourceProfile(t *testing.T) {
	tests := []struct {
		name     string
		profile  ResourceProfileType
		expected ResourceProfileType
	}{
		{
			name:     "empty profile defaults to minimal",
			profile:  "",
			expected: ResourceProfileMinimal,
		},
		{
			name:     "explicit minimal is respected",
			profile:  ResourceProfileMinimal,
			expected: ResourceProfileMinimal,
		},
		{
			name:     "explicit cluster is respected",
			profile:  ResourceProfileCluster,
			expected: ResourceProfileCluster,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ResourceProfile: tt.profile}
			assert.Equal(t, tt.expected, cfg.GetEffectiveResourceProfile())
		})
	}
}

func TestMinimalResources(t *testing.T) {
	minimal := MinimalResources()
	standard := DefaultResources()

	// Minimal profile should have lower CPU requests than standard (string comparison works for "25m" < "50m")
	assert.Equal(t, "25m", minimal.Cilium.Resources.Requests.CPU)
	assert.Equal(t, "50m", standard.Cilium.Resources.Requests.CPU)

	// Minimal profile should have lower memory requests
	assert.Equal(t, "64Mi", minimal.Cilium.Resources.Requests.Memory)
	assert.Equal(t, "128Mi", standard.Cilium.Resources.Requests.Memory)

	// Minimal profile should have no limits (best-effort scheduling)
	assert.Empty(t, minimal.Cilium.Resources.Limits.CPU)
	assert.Empty(t, minimal.Cilium.Resources.Limits.Memory)

	// Standard profile should have limits
	assert.NotEmpty(t, standard.Cilium.Resources.Limits.CPU)
	assert.NotEmpty(t, standard.Cilium.Resources.Limits.Memory)

	// Verify minimal CPU inference resources are reduced
	assert.Equal(t, "2000m", minimal.LLMDCPU.Resources.Requests.CPU)
	assert.Equal(t, "4000m", standard.LLMDCPU.Resources.Requests.CPU)
	assert.Equal(t, "6Gi", minimal.LLMDCPU.Resources.Requests.Memory)
	assert.Equal(t, "12Gi", standard.LLMDCPU.Resources.Requests.Memory)
}

func TestGetResourcesWithDefaults_ProfileSelection(t *testing.T) {
	// Minimal profile uses MinimalResources
	minimalCfg := &Config{ResourceProfile: ResourceProfileMinimal}
	minimalRes := minimalCfg.GetResourcesWithDefaults()
	assert.Equal(t, MinimalResources().Cilium.Resources.Requests.CPU, minimalRes.Cilium.Resources.Requests.CPU)

	// Cluster profile uses DefaultResources
	clusterCfg := &Config{ResourceProfile: ResourceProfileCluster}
	clusterRes := clusterCfg.GetResourcesWithDefaults()
	assert.Equal(t, DefaultResources().Cilium.Resources.Requests.CPU, clusterRes.Cilium.Resources.Requests.CPU)

	// Empty profile defaults to minimal
	emptyCfg := &Config{}
	emptyRes := emptyCfg.GetResourcesWithDefaults()
	assert.Equal(t, MinimalResources().Cilium.Resources.Requests.CPU, emptyRes.Cilium.Resources.Requests.CPU)
}

func TestGetResourcesWithDefaults_UserOverride(t *testing.T) {
	cfg := &Config{
		ResourceProfile: ResourceProfileMinimal, // Use minimal profile
		Resources: ResourcesConfig{
			Cilium: ComponentResources{
				Resources: ResourceRequirements{
					Requests: ResourceSpec{CPU: "100m"}, // Override just CPU
				},
			},
		},
	}

	res := cfg.GetResourcesWithDefaults()

	// CPU should be overridden
	assert.Equal(t, "100m", res.Cilium.Resources.Requests.CPU)

	// Memory should come from minimal profile
	assert.Equal(t, MinimalResources().Cilium.Resources.Requests.Memory, res.Cilium.Resources.Requests.Memory)
}

func TestProfileTopology(t *testing.T) {
	// Minimal: single-node with 6 CPUs, 12GB
	nodes, cpus, mem := ProfileTopology(ResourceProfileMinimal)
	assert.Equal(t, 1, nodes)
	assert.Equal(t, 6, cpus)
	assert.Equal(t, 12288, mem)

	// Cluster: 3 nodes with 4 CPUs, 4GB each
	nodes, cpus, mem = ProfileTopology(ResourceProfileCluster)
	assert.Equal(t, 3, nodes)
	assert.Equal(t, 4, cpus)
	assert.Equal(t, 4096, mem)

	// Unknown profile defaults to minimal
	nodes, cpus, mem = ProfileTopology("unknown")
	assert.Equal(t, 1, nodes)
	assert.Equal(t, 6, cpus)
	assert.Equal(t, 12288, mem)
}

func TestGetMemory_GPUModeReducesRAM(t *testing.T) {
	// Minimal + CPU mode: 12GB RAM (needs RAM for inference)
	cpuCfg := &Config{
		ResourceProfile: ResourceProfileMinimal,
		Minikube:        MinikubeConfig{}, // Empty = CPU mode
	}
	assert.Equal(t, 12288, cpuCfg.GetMemory())

	// Minimal + GPU mode: 6GB RAM (VRAM handles inference)
	gpuCfg := &Config{
		ResourceProfile: ResourceProfileMinimal,
		Minikube:        MinikubeConfig{GPUMode: GPUModeNVIDIA},
	}
	assert.Equal(t, 6144, gpuCfg.GetMemory())

	// Cluster profile is unaffected by GPU mode
	clusterCfg := &Config{
		ResourceProfile: ResourceProfileCluster,
		Minikube:        MinikubeConfig{GPUMode: GPUModeNVIDIA},
	}
	assert.Equal(t, 4096, clusterCfg.GetMemory())
}

func TestValidate_CPUModeRequiresMinimalProfile(t *testing.T) {
	// CPU mode (empty GPUMode) with minimal profile should be valid
	minimalCPU := &Config{
		ResourceProfile: ResourceProfileMinimal,
		Minikube:        MinikubeConfig{}, // Empty GPUMode = CPU mode
		DNS:             DNSConfig{Bind9Port: 30053},
	}
	assert.NoError(t, minimalCPU.Validate())

	// CPU mode with cluster profile should fail
	clusterCPU := &Config{
		ResourceProfile: ResourceProfileCluster,
		Minikube:        MinikubeConfig{}, // Empty GPUMode = CPU mode
		DNS:             DNSConfig{Bind9Port: 30053},
	}
	err := clusterCPU.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CPU inference mode requires 'minimal' profile")

	// NVIDIA GPU mode with cluster profile should be valid
	clusterGPU := &Config{
		ResourceProfile: ResourceProfileCluster,
		Minikube:        MinikubeConfig{GPUMode: GPUModeNVIDIA},
		DNS:             DNSConfig{Bind9Port: 30053},
	}
	assert.NoError(t, clusterGPU.Validate())
}

func TestValidateConfigChange_NoDeployedCluster(t *testing.T) {
	// No deployed cluster (LastDeployedTier = 0) - any change is allowed
	cfg := &Config{
		State: StateConfig{
			LastDeployedTier: 0,
			DeployedProfile:  "",
			DeployedGPUMode:  "",
		},
	}

	// Changing to any profile should be allowed
	assert.NoError(t, cfg.ValidateConfigChange(ResourceProfileMinimal, ""))
	assert.NoError(t, cfg.ValidateConfigChange(ResourceProfileCluster, GPUModeNVIDIA))
}

func TestValidateConfigChange_SameSettings(t *testing.T) {
	// Deployed cluster with same settings - should be allowed
	cfg := &Config{
		State: StateConfig{
			LastDeployedTier: 3,
			DeployedProfile:  ResourceProfileMinimal,
			DeployedGPUMode:  "",
		},
	}

	// Same settings should pass
	assert.NoError(t, cfg.ValidateConfigChange(ResourceProfileMinimal, ""))
}

func TestValidateConfigChange_ProfileChange(t *testing.T) {
	// Deployed cluster - changing profile should error
	cfg := &Config{
		State: StateConfig{
			LastDeployedTier: 3,
			DeployedProfile:  ResourceProfileMinimal,
			DeployedGPUMode:  "",
		},
	}

	err := cfg.ValidateConfigChange(ResourceProfileCluster, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot change profile from 'minimal' to 'cluster'")
	assert.Contains(t, err.Error(), "nova delete")
}

func TestValidateConfigChange_GPUModeChange(t *testing.T) {
	// Deployed cluster with CPU mode - changing to GPU should error
	cfg := &Config{
		State: StateConfig{
			LastDeployedTier: 3,
			DeployedProfile:  ResourceProfileMinimal,
			DeployedGPUMode:  "", // CPU mode
		},
	}

	err := cfg.ValidateConfigChange(ResourceProfileMinimal, GPUModeNVIDIA)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot change GPU mode from 'cpu' to 'nvidia'")
	assert.Contains(t, err.Error(), "nova delete")
}

func TestValidateConfigChange_GPUToCPU(t *testing.T) {
	// Deployed cluster with GPU mode - changing to CPU should error
	cfg := &Config{
		State: StateConfig{
			LastDeployedTier: 3,
			DeployedProfile:  ResourceProfileMinimal,
			DeployedGPUMode:  GPUModeNVIDIA,
		},
	}

	err := cfg.ValidateConfigChange(ResourceProfileMinimal, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot change GPU mode from 'nvidia' to 'cpu'")
	assert.Contains(t, err.Error(), "nova delete")
}

func TestValidateConfigChange_BothProfileAndGPU(t *testing.T) {
	// Changing both profile and GPU mode - should error on first check (profile)
	cfg := &Config{
		State: StateConfig{
			LastDeployedTier: 3,
			DeployedProfile:  ResourceProfileMinimal,
			DeployedGPUMode:  "",
		},
	}

	err := cfg.ValidateConfigChange(ResourceProfileCluster, GPUModeNVIDIA)
	assert.Error(t, err)
	// Should fail on profile change first
	assert.Contains(t, err.Error(), "cannot change profile")
}

// ==================== App Profile Tests ====================

func TestGetActiveAppProfiles_Default(t *testing.T) {
	cfg := &Config{}
	profiles := cfg.GetActiveAppProfiles()
	assert.Equal(t, []AppProfileType{AppProfileOpenWebUI, AppProfileLab}, profiles)
}

func TestGetActiveAppProfiles_Custom(t *testing.T) {
	cfg := &Config{
		AppProfiles: AppProfilesConfig{
			ActiveProfiles: []AppProfileType{AppProfileLaSuite},
		},
	}
	profiles := cfg.GetActiveAppProfiles()
	assert.Equal(t, []AppProfileType{AppProfileLaSuite}, profiles)
}

func TestGetActiveAppProfiles_Multiple(t *testing.T) {
	cfg := &Config{
		AppProfiles: AppProfilesConfig{
			ActiveProfiles: []AppProfileType{AppProfileOpenWebUI, AppProfileLaSuite, AppProfileLab},
		},
	}
	profiles := cfg.GetActiveAppProfiles()
	assert.Equal(t, 3, len(profiles))
	assert.Contains(t, profiles, AppProfileOpenWebUI)
	assert.Contains(t, profiles, AppProfileLaSuite)
	assert.Contains(t, profiles, AppProfileLab)
}

func TestSetActiveAppProfiles(t *testing.T) {
	cfg := Default()
	newProfiles := []AppProfileType{AppProfileLaSuite}
	cfg.SetActiveAppProfiles(newProfiles)
	assert.Equal(t, newProfiles, cfg.AppProfiles.ActiveProfiles)
}

func TestGetAppsToDeployFromProfiles_OpenWebUI(t *testing.T) {
	cfg := Default()
	cfg.AppProfiles.ActiveProfiles = []AppProfileType{AppProfileOpenWebUI}

	apps, err := cfg.GetAppsToDeployFromProfiles()
	require.NoError(t, err)

	appNames := make([]string, len(apps))
	for i, app := range apps {
		appNames[i] = app.Name
	}

	assert.Contains(t, appNames, "openwebui")
	assert.NotContains(t, appNames, "helix")
}

func TestGetAppsToDeployFromProfiles_Lab(t *testing.T) {
	cfg := Default()
	cfg.AppProfiles.ActiveProfiles = []AppProfileType{AppProfileLab}

	apps, err := cfg.GetAppsToDeployFromProfiles()
	require.NoError(t, err)

	appNames := make([]string, len(apps))
	for i, app := range apps {
		appNames[i] = app.Name
	}

	assert.Contains(t, appNames, "helix")
	assert.NotContains(t, appNames, "openwebui")
}

func TestGetAppsToDeployFromProfiles_LaSuite(t *testing.T) {
	cfg := Default()
	cfg.AppProfiles.ActiveProfiles = []AppProfileType{AppProfileLaSuite}

	apps, err := cfg.GetAppsToDeployFromProfiles()
	require.NoError(t, err)

	appNames := make([]string, len(apps))
	for i, app := range apps {
		appNames[i] = app.Name
	}

	assert.Contains(t, appNames, "opengatellm")
	assert.Contains(t, appNames, "conversations")
	assert.NotContains(t, appNames, "openwebui")
}

func TestGetAppsToDeployFromProfiles_WithDependencies(t *testing.T) {
	cfg := Default()
	cfg.AppProfiles.ActiveProfiles = []AppProfileType{AppProfileLaSuite}

	apps, err := cfg.GetAppsToDeployFromProfiles()
	require.NoError(t, err)

	// Verify opengatellm comes before conversations (dependency)
	var opengatellmIndex, conversationsIndex int
	for i, app := range apps {
		if app.Name == "opengatellm" {
			opengatellmIndex = i
		}
		if app.Name == "conversations" {
			conversationsIndex = i
		}
	}
	assert.Less(t, opengatellmIndex, conversationsIndex, "opengatellm should come before conversations due to dependency")
}

func TestGetAppsToDeployFromProfiles_UnknownProfile(t *testing.T) {
	cfg := Default()
	cfg.AppProfiles.ActiveProfiles = []AppProfileType{"nonexistent"}

	_, err := cfg.GetAppsToDeployFromProfiles()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown app profile")
}

func TestValidateAppProfile_Valid(t *testing.T) {
	cfg := Default()

	assert.NoError(t, cfg.ValidateAppProfile(AppProfileOpenWebUI))
	assert.NoError(t, cfg.ValidateAppProfile(AppProfileLaSuite))
	assert.NoError(t, cfg.ValidateAppProfile(AppProfileLab))
}

func TestValidateAppProfile_Invalid(t *testing.T) {
	cfg := Default()

	err := cfg.ValidateAppProfile("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown profile")
}

func TestIsAppEnabled(t *testing.T) {
	cfg := Default()
	cfg.AppProfiles.ActiveProfiles = []AppProfileType{AppProfileOpenWebUI}

	assert.True(t, cfg.IsAppEnabled("openwebui"))
	assert.False(t, cfg.IsAppEnabled("helix"))
	assert.False(t, cfg.IsAppEnabled("opengatellm"))
}

func TestIsAppEnabled_MultipleProfiles(t *testing.T) {
	cfg := Default()
	cfg.AppProfiles.ActiveProfiles = []AppProfileType{AppProfileOpenWebUI, AppProfileLab}

	assert.True(t, cfg.IsAppEnabled("openwebui"))
	assert.True(t, cfg.IsAppEnabled("helix"))
	assert.False(t, cfg.IsAppEnabled("opengatellm"))
}

func TestGetAppDefinition(t *testing.T) {
	cfg := Default()

	app, exists := cfg.GetAppDefinition("openwebui")
	assert.True(t, exists)
	assert.Equal(t, "openwebui", app.Name)
	assert.Equal(t, "openwebui", app.Namespace)

	_, exists = cfg.GetAppDefinition("nonexistent")
	assert.False(t, exists)
}

func TestDefaultAppProfiles(t *testing.T) {
	cfg := Default()

	// Verify built-in profiles exist
	assert.Contains(t, cfg.AppProfiles.Profiles, AppProfileOpenWebUI)
	assert.Contains(t, cfg.AppProfiles.Profiles, AppProfileLaSuite)
	assert.Contains(t, cfg.AppProfiles.Profiles, AppProfileLab)

	// Verify default active profiles
	assert.Equal(t, []AppProfileType{AppProfileOpenWebUI, AppProfileLab}, cfg.AppProfiles.ActiveProfiles)

	// Verify apps exist
	assert.Contains(t, cfg.AppProfiles.Apps, "openwebui")
	assert.Contains(t, cfg.AppProfiles.Apps, "helix")
	assert.Contains(t, cfg.AppProfiles.Apps, "opengatellm")
	assert.Contains(t, cfg.AppProfiles.Apps, "conversations")
}

func TestAppDefinition_Dependencies(t *testing.T) {
	cfg := Default()

	// Conversations should depend on opengatellm
	conversations, exists := cfg.GetAppDefinition("conversations")
	assert.True(t, exists)
	assert.Contains(t, conversations.Dependencies, "opengatellm")

	// OpenWebUI should have no dependencies
	openwebui, exists := cfg.GetAppDefinition("openwebui")
	assert.True(t, exists)
	assert.Empty(t, openwebui.Dependencies)
}
