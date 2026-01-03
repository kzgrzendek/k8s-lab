// Package helm provides a centralized wrapper around the Helm SDK.
//
// This package uses the official Helm SDK (helm.sh/helm/v4/pkg/action)
// to manage Helm charts and releases. All Helm operations in NOVA should
// go through this package to maintain consistency and ease future refactoring.
package helm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/registry"
	repov1 "helm.sh/helm/v4/pkg/repo/v1"
)

// Client wraps the Helm SDK with NOVA-specific functionality.
type Client struct {
	settings *cli.EnvSettings
}

// NewClient creates a new Helm client.
func NewClient(namespace string) *Client {
	settings := cli.New()
	if namespace != "" {
		settings.SetNamespace(namespace)
	}
	return &Client{settings: settings}
}

// AddRepository adds a Helm repository.
func (c *Client) AddRepository(ctx context.Context, name, url string) error {
	// Get repo file path
	repoFile := c.settings.RepositoryConfig

	// Load existing repos
	f, err := repov1.LoadFile(repoFile)
	if os.IsNotExist(err) || err != nil {
		f = repov1.NewFile()
	}

	// Check if repo already exists
	if f.Has(name) {
		// Update URL if different
		if entry := f.Get(name); entry.URL != url {
			f.Update(&repov1.Entry{Name: name, URL: url})
		}
	} else {
		// Add new repo
		f.Add(&repov1.Entry{Name: name, URL: url})
	}

	// Save repo file
	if err := f.WriteFile(repoFile, 0644); err != nil {
		return fmt.Errorf("failed to write repo file: %w", err)
	}

	// Update repo index
	r, err := repov1.NewChartRepository(&repov1.Entry{Name: name, URL: url}, getter.All(c.settings))
	if err != nil {
		return fmt.Errorf("failed to create chart repository: %w", err)
	}

	if _, err := r.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download repository index: %w", err)
	}

	return nil
}

// UpdateRepositories updates all configured Helm repositories.
func (c *Client) UpdateRepositories(ctx context.Context) error {
	repoFile := c.settings.RepositoryConfig

	f, err := repov1.LoadFile(repoFile)
	if err != nil {
		return fmt.Errorf("failed to load repository file: %w", err)
	}

	for _, cfg := range f.Repositories {
		r, err := repov1.NewChartRepository(cfg, getter.All(c.settings))
		if err != nil {
			return fmt.Errorf("failed to create chart repository %s: %w", cfg.Name, err)
		}

		if _, err := r.DownloadIndexFile(); err != nil {
			return fmt.Errorf("failed to update repository %s: %w", cfg.Name, err)
		}
	}

	return nil
}

// InstallOrUpgradeRelease installs or upgrades a Helm release.
func (c *Client) InstallOrUpgradeRelease(ctx context.Context, releaseName, chartRef, namespace string, values map[string]interface{}, wait bool) error {
	return c.InstallOrUpgradeReleaseWithTimeout(ctx, releaseName, chartRef, namespace, values, wait, 300)
}

// InstallOrUpgradeReleaseWithTimeout installs or upgrades a Helm release with a custom timeout.
// timeout is in seconds. Default Helm timeout is 300s (5 minutes).
func (c *Client) InstallOrUpgradeReleaseWithTimeout(ctx context.Context, releaseName, chartRef, namespace string, values map[string]interface{}, wait bool, timeoutSeconds int) error {
	return c.InstallOrUpgradeReleaseWithOptions(ctx, releaseName, chartRef, namespace, values, wait, timeoutSeconds, false)
}

// InstallOrUpgradeReleaseWithOptions installs or upgrades a Helm release with additional options.
// Handles both OCI (oci://) and traditional repository charts transparently.
func (c *Client) InstallOrUpgradeReleaseWithOptions(ctx context.Context, releaseName, chartRef, namespace string, values map[string]interface{}, wait bool, timeoutSeconds int, reuseValues bool) error {
	// Initialize Helm CLI settings fresh for this operation
	settings := cli.New()

	// Create registry client for OCI chart support
	// ClientOptEnableCache is CRUCIAL for proper OCI mediatype handling
	registryClient, err := registry.NewClient(
		registry.ClientOptEnableCache(true),
	)
	if err != nil {
		return fmt.Errorf("failed to create registry client: %w", err)
	}

	// Configure the action
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return fmt.Errorf("failed to initialize action config: %w", err)
	}
	actionConfig.RegistryClient = registryClient

	// Check if release exists using Status (lighter than History)
	statusClient := action.NewStatus(actionConfig)
	_, err = statusClient.Run(releaseName)
	if err != nil {
		// Release doesn't exist, install it
		if strings.Contains(err.Error(), "not found") {
			return c.installRelease(ctx, settings, actionConfig, releaseName, chartRef, namespace, values, wait, timeoutSeconds)
		}
		return fmt.Errorf("failed to check release status: %w", err)
	}

	// Release exists, upgrade it
	return c.upgradeRelease(ctx, settings, actionConfig, releaseName, chartRef, namespace, values, wait, timeoutSeconds, reuseValues)
}

// installRelease installs a new Helm release.
// Helm v4 handles OCI charts natively without explicit registry client configuration.
func (c *Client) installRelease(ctx context.Context, settings *cli.EnvSettings, actionConfig *action.Configuration, releaseName, chartRef, namespace string, values map[string]interface{}, wait bool, timeoutSeconds int) error {
	client := action.NewInstall(actionConfig)
	client.ReleaseName = releaseName
	client.Namespace = namespace
	client.CreateNamespace = true
	if wait {
		client.WaitStrategy = kube.StatusWatcherStrategy
	}
	client.Timeout = time.Duration(timeoutSeconds) * time.Second

	// Locate chart (polymorphic: handles OCI and HTTP transparently)
	chartPath, err := client.ChartPathOptions.LocateChart(chartRef, settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	// Load chart
	chartLoaded, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Execute installation
	if _, err := client.Run(chartLoaded, values); err != nil {
		return fmt.Errorf("failed to install release: %w", err)
	}

	return nil
}

// upgradeRelease upgrades an existing Helm release.
// Helm v4 handles OCI charts natively without explicit registry client configuration.
func (c *Client) upgradeRelease(ctx context.Context, settings *cli.EnvSettings, actionConfig *action.Configuration, releaseName, chartRef, namespace string, values map[string]interface{}, wait bool, timeoutSeconds int, reuseValues bool) error {
	client := action.NewUpgrade(actionConfig)
	client.Namespace = namespace
	if wait {
		client.WaitStrategy = kube.StatusWatcherStrategy
	}
	client.Timeout = time.Duration(timeoutSeconds) * time.Second
	client.ReuseValues = reuseValues

	// Locate chart (polymorphic: handles OCI and HTTP transparently)
	chartPath, err := client.ChartPathOptions.LocateChart(chartRef, settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	// Load chart
	chartLoaded, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Execute upgrade
	if _, err := client.Run(releaseName, chartLoaded, values); err != nil {
		return fmt.Errorf("failed to upgrade release: %w", err)
	}

	return nil
}

// Uninstall uninstalls a Helm release.
func (c *Client) Uninstall(ctx context.Context, releaseName, namespace string) error {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(c.settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return fmt.Errorf("failed to initialize action config: %w", err)
	}

	client := action.NewUninstall(actionConfig)
	if _, err := client.Run(releaseName); err != nil {
		return fmt.Errorf("failed to uninstall release %s: %w", releaseName, err)
	}

	return nil
}

// ReleaseExists checks if a Helm release exists in the given namespace.
func (c *Client) ReleaseExists(ctx context.Context, releaseName, namespace string) (bool, error) {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(c.settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return false, fmt.Errorf("failed to initialize action config: %w", err)
	}

	// Use Status (lighter than History) to check if release exists
	client := action.NewStatus(actionConfig)
	_, err := client.Run(releaseName)
	if err != nil {
		// If status returns "not found", the release doesn't exist
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check release: %w", err)
	}

	return true, nil
}

// LoadValues loads Helm values from a YAML file and returns them as a map.
// If the file doesn't exist, returns an empty map (not an error).
// This allows for optional values files.
func LoadValues(valuesPath string) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		// File doesn't exist, return empty values (not an error)
		return make(map[string]interface{}), nil
	}

	// Read values file
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file %s: %w", valuesPath, err)
	}

	// Parse YAML
	var values map[string]interface{}
	if err := UnmarshalValues(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse values file %s: %w", valuesPath, err)
	}

	return values, nil
}

// UnmarshalValues parses YAML bytes into a values map.
func UnmarshalValues(data []byte, v *map[string]interface{}) error {
	return yaml.Unmarshal(data, v)
}

// MergeValues merges multiple value maps together.
// Later maps override earlier ones. Nested maps are merged recursively.
func MergeValues(base map[string]interface{}, overrides ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base values
	for k, v := range base {
		result[k] = v
	}

	// Apply each override
	for _, override := range overrides {
		for k, v := range override {
			// If both values are maps, merge recursively
			if existingMap, existingOk := result[k].(map[string]interface{}); existingOk {
				if overrideMap, overrideOk := v.(map[string]interface{}); overrideOk {
					result[k] = MergeValues(existingMap, overrideMap)
					continue
				}
			}
			// Otherwise, override the value
			result[k] = v
		}
	}

	return result
}
