// Package config handles NOVA configuration management using Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

// Config represents the NOVA configuration.
type Config struct {
	Minikube MinikubeConfig `json:"minikube" yaml:"minikube"`
	DNS      DNSConfig      `json:"dns" yaml:"dns"`
	State    StateConfig    `json:"state" yaml:"state"`
}

// MinikubeConfig holds Minikube cluster settings.
type MinikubeConfig struct {
	CPUs              int    `json:"cpus" yaml:"cpus"`
	Memory            int    `json:"memory" yaml:"memory"`
	Nodes             int    `json:"nodes" yaml:"nodes"`
	KubernetesVersion string `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	Driver            string `json:"driver" yaml:"driver"`
	GPUs              string `json:"gpus" yaml:"gpus"`
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
		},
		DNS: DNSConfig{
			Domain:     "lab.k8s.local",
			AuthDomain: "auth.k8s.local",
			Bind9Port:  30053,
		},
		State: StateConfig{
			Initialized:      false,
			LastDeployedTier: 0,
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

	return cfg, nil
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

// Get returns a config value from Viper by key.
func Get(key string) interface{} {
	return viper.Get(key)
}

// GetString returns a string config value from Viper.
func GetString(key string) string {
	return viper.GetString(key)
}

// GetInt returns an int config value from Viper.
func GetInt(key string) int {
	return viper.GetInt(key)
}

// GetBool returns a bool config value from Viper.
func GetBool(key string) bool {
	return viper.GetBool(key)
}
