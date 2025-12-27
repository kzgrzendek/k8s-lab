// Package nginx manages the NGINX gateway for NOVA.
package nginx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/kzgrzendek/nova/internal/config"
)

const (
	containerName  = "nova-nginx-gateway"
	containerImage = "nginx:stable-alpine3.21-perl"
)

// Start starts the NGINX gateway container.
func Start(ctx context.Context, cfg *config.Config) error {
	// Check if container already exists and remove it
	if IsRunning(ctx) {
		if err := exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run(); err != nil {
			return fmt.Errorf("failed to remove existing nginx container: %w", err)
		}
	}

	// Get Minikube IP address
	minikubeIP, err := getMinikubeIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get minikube IP: %w", err)
	}

	// Ensure configuration directory exists
	configDir := filepath.Join(config.ConfigDir(), "nginx")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create nginx config directory: %w", err)
	}
	confdDir := filepath.Join(configDir, "conf.d")
	if err := os.MkdirAll(confdDir, 0755); err != nil {
		return fmt.Errorf("failed to create nginx conf.d directory: %w", err)
	}

	// Generate configuration files
	if err := generateNginxConf(cfg, configDir, minikubeIP); err != nil {
		return fmt.Errorf("failed to generate nginx.conf: %w", err)
	}
	if err := generateDefaultConf(cfg, confdDir); err != nil {
		return fmt.Errorf("failed to generate default.conf: %w", err)
	}

	// Start nginx container
	nginxConfPath := filepath.Join(configDir, "nginx.conf")
	confdDirPath := confdDir

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", "minikube",
		"-p", "80:80",
		"-p", "443:443",
		"-v", fmt.Sprintf("%s:/etc/nginx/nginx.conf:ro", nginxConfPath),
		"-v", fmt.Sprintf("%s:/etc/nginx/conf.d:ro", confdDirPath),
		containerImage,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start nginx container: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Stop stops the NGINX gateway container.
func Stop(ctx context.Context) error {
	if !IsRunning(ctx) {
		return nil
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop nginx container: %w", err)
	}
	return nil
}

// Delete removes the NGINX gateway container.
func Delete(ctx context.Context) error {
	if !IsRunning(ctx) && !containerExists(ctx) {
		return nil
	}

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete nginx container: %w", err)
	}
	return nil
}

// IsRunning checks if the NGINX container is running.
func IsRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return string(output)[:4] == "true"
}

// containerExists checks if the NGINX container exists (running or stopped).
func containerExists(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerName)
	return cmd.Run() == nil
}

// getMinikubeIP retrieves the Minikube cluster IP address.
func getMinikubeIP(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "minikube", "ip")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output[:len(output)-1]), nil // Remove trailing newline
}

// generateNginxConf generates the main nginx.conf file.
func generateNginxConf(cfg *config.Config, configDir, minikubeIP string) error {
	tmpl := `user nginx;
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

	t, err := template.New("nginx.conf").Parse(tmpl)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(configDir, "nginx.conf"))
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, struct {
		MinikubeIP string
	}{
		MinikubeIP: minikubeIP,
	})
}

// generateDefaultConf generates the default.conf file for HTTP to HTTPS redirect.
func generateDefaultConf(cfg *config.Config, confdDir string) error {
	tmpl := `server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name *.{{ .Domain }} *.{{ .AuthDomain }};

    return 307 https://$host$request_uri;
}
`

	t, err := template.New("default.conf").Parse(tmpl)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(confdDir, "default.conf"))
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, struct {
		Domain     string
		AuthDomain string
	}{
		Domain:     cfg.DNS.Domain,
		AuthDomain: cfg.DNS.AuthDomain,
	})
}
