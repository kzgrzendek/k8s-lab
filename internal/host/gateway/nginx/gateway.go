// Package nginx manages the NGINX gateway for NOVA.
package nginx

import (
	"context"
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

// Templates for NGINX configuration files.
const (
	nginxConfTemplate = `user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log notice;
pid /run/nginx.pid;

events {
    worker_connections 1024;
}

stream {
    upstream minikube_ingress {
        server {{ .MinikubeIP }}:30443;
    }

    server {
        listen 443;
        listen [::]:443;
        proxy_pass minikube_ingress;
        ssl_preread on;  # Required to pass SNI for TLS passthrough
    }
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                    '$status $body_bytes_sent "$http_referer" '
                    '"$http_user_agent" "$http_x_forwarded_for"';

    access_log /var/log/nginx/access.log main;

    sendfile on;
    keepalive_timeout 65;

    include /etc/nginx/conf.d/*.conf;
}
`

	defaultConfTemplate = `server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name *.{{ .Domain }} *.{{ .AuthDomain }};

    return 307 https://$host$request_uri;
}
`
)

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

			return []shared.TemplateConfig{
				{
					Name:       "nginx.conf",
					Content:    nginxConfTemplate,
					OutputFile: "nginx.conf",
					Data: struct{ MinikubeIP string }{
						MinikubeIP: minikubeIP,
					},
				},
				{
					Name:       "default.conf",
					Content:    defaultConfTemplate,
					OutputFile: "conf.d/default.conf",
					Data: struct {
						Domain     string
						AuthDomain string
					}{
						Domain:     cfg.DNS.Domain,
						AuthDomain: cfg.DNS.AuthDomain,
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
					"80/tcp":  "80/tcp",  // HTTP
					"443/tcp": "443/tcp", // HTTPS
				},
				Volumes: map[string]string{
					nginxConfPath: "/etc/nginx/nginx.conf",
					confdDir:      "/etc/nginx/conf.d",
				},
				RestartPolicy:      "unless-stopped",
				AdditionalNetworks: []string{"minikube"}, // Connect to minikube for TLS passthrough
			}
		},
	})
}

// Start starts the NGINX gateway container.
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
