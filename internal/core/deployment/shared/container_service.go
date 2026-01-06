package shared

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/tools/docker"
)

// ContainerService defines the interface for managing containerized host services.
// This interface is implemented by services like Bind9 DNS and NGINX gateway.
type ContainerService interface {
	// Start starts the service container
	Start(ctx context.Context, cfg *config.Config) error
	// Stop stops the service container
	Stop(ctx context.Context) error
	// Delete removes the service container and cleans up resources
	Delete(ctx context.Context) error
	// IsRunning checks if the service container is running
	IsRunning(ctx context.Context) bool
}

// TemplateConfig represents a configuration file template.
type TemplateConfig struct {
	// Name is the template name (for error messages)
	Name string
	// Content is the template content
	Content string
	// OutputFile is the output filename (relative to config dir)
	OutputFile string
	// Data is the data to pass to the template
	Data any
}

// ContainerServiceConfig holds configuration for a container-based service.
type ContainerServiceConfig struct {
	// ContainerName is the name of the Docker container
	ContainerName string
	// Image is the Docker image to use
	Image string
	// ConfigDir is the directory to store configuration files
	ConfigDir string
	// BuildContainer is a function that builds the container configuration
	BuildContainer func(cfg *config.Config, configDir string) docker.ContainerConfig
	// BuildTemplates is a function that builds the configuration templates
	BuildTemplates func(cfg *config.Config) []TemplateConfig
}

// ContainerBasedService implements ContainerService using Docker.
// This is a generic implementation that can be used for any service that:
//   - Runs in a Docker container
//   - Requires configuration files generated from templates
//   - Follows a standard lifecycle (Start/Stop/Delete/IsRunning)
type ContainerBasedService struct {
	config ContainerServiceConfig
}

// NewContainerService creates a new container-based service.
func NewContainerService(config ContainerServiceConfig) ContainerService {
	return &ContainerBasedService{config: config}
}

// Start starts the service container.
// It performs the following steps:
//  1. Creates Docker client
//  2. Checks if container exists and removes it if needed
//  3. Creates configuration directory
//  4. Generates configuration files from templates
//  5. Creates and starts the container
func (s *ContainerBasedService) Start(ctx context.Context, cfg *config.Config) error {
	return WithDockerClient(ctx, func(client *docker.Client) error {
		// Check if container exists
		exists, err := client.Exists(ctx, s.config.ContainerName)
		if err != nil {
			return fmt.Errorf("failed to check if container exists: %w", err)
		}

		// Remove existing container if it exists
		if exists {
			if err := client.Remove(ctx, s.config.ContainerName, true); err != nil {
				return fmt.Errorf("failed to remove existing container: %w", err)
			}
		}

		// Create config directory
		configDir := filepath.Join(config.ConfigDir(), s.config.ConfigDir)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Build and generate configuration files from templates
		if s.config.BuildTemplates != nil {
			templates := s.config.BuildTemplates(cfg)
			for _, tmpl := range templates {
				outputPath := filepath.Join(configDir, tmpl.OutputFile)
				// Create subdirectories if needed
				if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
					return fmt.Errorf("failed to create directory for %s: %w", tmpl.Name, err)
				}
				if err := GenerateTemplateFile(tmpl.Content, outputPath, tmpl.Data); err != nil {
					return fmt.Errorf("failed to generate %s: %w", tmpl.Name, err)
				}
			}
		}

		// Build container configuration
		containerConfig := s.config.BuildContainer(cfg, configDir)

		// Create and start container
		if err := client.CreateAndStart(ctx, containerConfig); err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}

		return nil
	})
}

// Stop stops the service container.
func (s *ContainerBasedService) Stop(ctx context.Context) error {
	return WithDockerClient(ctx, func(client *docker.Client) error {
		if err := client.Stop(ctx, s.config.ContainerName); err != nil {
			return fmt.Errorf("failed to stop %s: %w", s.config.ContainerName, err)
		}
		return nil
	})
}

// Delete removes the service container and cleans up resources.
func (s *ContainerBasedService) Delete(ctx context.Context) error {
	return WithDockerClient(ctx, func(client *docker.Client) error {
		if err := client.Remove(ctx, s.config.ContainerName, true); err != nil {
			return fmt.Errorf("failed to delete %s: %w", s.config.ContainerName, err)
		}
		return nil
	})
}

// IsRunning checks if the service container is running.
func (s *ContainerBasedService) IsRunning(ctx context.Context) bool {
	var running bool
	_ = WithDockerClient(ctx, func(client *docker.Client) error {
		var err error
		running, err = client.IsRunning(ctx, s.config.ContainerName)
		return err
	})
	return running
}

// WithDockerClient creates a Docker client, executes a function, and ensures cleanup.
// This helper eliminates the repeated pattern of:
//
//	client, err := docker.NewClient()
//	if err != nil { return err }
//	defer client.Close()
func WithDockerClient(ctx context.Context, fn func(*docker.Client) error) error {
	client, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer client.Close()
	return fn(client)
}

// GenerateTemplateFile generates a file from a template.
// This utility eliminates repeated template generation code.
func GenerateTemplateFile(templateContent, outputPath string, data any) error {
	// Parse template
	tmpl, err := template.New(filepath.Base(outputPath)).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outputPath, err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
