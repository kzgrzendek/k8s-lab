// Package nginx manages the NGINX gateway for NOVA.
package nginx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/tools/docker"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

const (
	containerName  = constants.ContainerNginx
	containerImage = constants.ImageNginx
)

// loadTemplate reads a template file from the resources directory.
func loadTemplate(filename string) (string, error) {
	content, err := os.ReadFile(filepath.Join("resources/host/nginx/templates", filename))
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", filename, err)
	}
	return string(content), nil
}

// service is the singleton NGINX service instance.
var service shared.ContainerService

// init initializes the NGINX service with its configuration.
func init() {
	service = shared.NewContainerService(shared.ContainerServiceConfig{
		ContainerName: containerName,
		Image:         containerImage,
		ConfigDir:     "nginx",
		BuildTemplates: func(cfg *config.Config) []shared.TemplateConfig {
			// Get Minikube IP for nginx.conf template
			// Note: This will be called during Start(), so context is available
			minikubeIP, err := minikube.GetIP(context.Background())
			if err != nil {
				// If we can't get the IP, use a placeholder - the error will be caught during Start()
				minikubeIP = "localhost"
			}

			// Load template from file
			nginxConfTmpl, err := loadTemplate("nginx.conf.tmpl")
			if err != nil {
				panic(fmt.Sprintf("failed to load nginx.conf template: %v", err))
			}

			return []shared.TemplateConfig{
				{
					Name:       "nginx.conf",
					Content:    nginxConfTmpl,
					OutputFile: "nginx.conf",
					Data: struct{ MinikubeIP string }{
						MinikubeIP: minikubeIP,
					},
				},
			}
		},
		BuildContainer: func(cfg *config.Config, configDir string) docker.ContainerConfig {
			nginxConfPath := filepath.Join(configDir, "nginx.conf")
			confdDir := filepath.Join(configDir, "conf.d")

			return docker.ContainerConfig{
				Name:  containerName,
				Image: containerImage,
				Ports: map[string]string{
					"443/tcp": "443/tcp", // HTTPS
				},
				Volumes: map[string]string{
					nginxConfPath: "/etc/nginx/nginx.conf",
					confdDir:      "/etc/nginx/conf.d",
				},
				RestartPolicy:      "unless-stopped",
				AdditionalNetworks: []string{"nova"}, // Connect to nova network for TLS passthrough
			}
		},
	})
}

// Start starts the NGINX gateway container.
// This function fetches the minikube IP at runtime using minikube.GetIP().
func Start(ctx context.Context, cfg *config.Config) error {
	return service.Start(ctx, cfg)
}

// Stop stops the NGINX gateway container.
func Stop(ctx context.Context) error {
	return service.Stop(ctx)
}

// Delete removes the NGINX gateway container.
func Delete(ctx context.Context) error {
	return service.Delete(ctx)
}

// IsRunning checks if the NGINX container is running.
func IsRunning(ctx context.Context) bool {
	return service.IsRunning(ctx)
}
