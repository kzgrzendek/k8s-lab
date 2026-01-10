// Package bind9 manages the Bind9 DNS server for NOVA domains.
package bind9

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/tools/docker"
)

const (
	containerName  = constants.ContainerBind9
	containerImage = constants.ImageBind9
)

var (
	dnsPort = strconv.Itoa(constants.Bind9Port)
)

// loadTemplate reads a template file from the resources directory.
func loadTemplate(filename string) (string, error) {
	content, err := os.ReadFile(filepath.Join("resources/host/bind9/templates", filename))
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", filename, err)
	}
	return string(content), nil
}

// service is the singleton Bind9 service instance.
var service shared.ContainerService

// init initializes the Bind9 service with its configuration.
func init() {
	service = shared.NewContainerService(shared.ContainerServiceConfig{
		ContainerName: containerName,
		Image:         containerImage,
		ConfigDir:     "bind9",
		BuildTemplates: func(cfg *config.Config) []shared.TemplateConfig {
			// Load templates from files
			namedConfTmpl, err := loadTemplate("named.conf.tmpl")
			if err != nil {
				panic(fmt.Sprintf("failed to load named.conf template: %v", err))
			}

			zoneFileTmpl, err := loadTemplate("db.zone.tmpl")
			if err != nil {
				panic(fmt.Sprintf("failed to load zone template: %v", err))
			}

			return []shared.TemplateConfig{
				{
					Name:       "named.conf",
					Content:    namedConfTmpl,
					OutputFile: "named.conf",
					Data: struct {
						Domain     string
						AuthDomain string
					}{
						Domain:     cfg.DNS.Domain,
						AuthDomain: cfg.DNS.AuthDomain,
					},
				},
				{
					Name:       fmt.Sprintf("db.%s", cfg.DNS.Domain),
					Content:    zoneFileTmpl,
					OutputFile: fmt.Sprintf("zones/db.%s", cfg.DNS.Domain),
					Data:       struct{ Domain string }{Domain: cfg.DNS.Domain},
				},
				{
					Name:       fmt.Sprintf("db.%s", cfg.DNS.AuthDomain),
					Content:    zoneFileTmpl,
					OutputFile: fmt.Sprintf("zones/db.%s", cfg.DNS.AuthDomain),
					Data:       struct{ Domain string }{Domain: cfg.DNS.AuthDomain},
				},
			}
		},
		BuildContainer: func(cfg *config.Config, configDir string) docker.ContainerConfig {
			zonesDir := filepath.Join(configDir, "zones")
			namedConfPath := filepath.Join(configDir, "named.conf")
			domainZonePath := filepath.Join(zonesDir, fmt.Sprintf("db.%s", cfg.DNS.Domain))
			authZonePath := filepath.Join(zonesDir, fmt.Sprintf("db.%s", cfg.DNS.AuthDomain))

			return docker.ContainerConfig{
				Name:  containerName,
				Image: containerImage,
				// bind9 needs to run as root to bind port 53 and drop privileges to bind user
				Ports: map[string]string{
					dnsPort + "/tcp": "53/tcp",
					dnsPort + "/udp": "53/udp",
				},
				Volumes: map[string]string{
					namedConfPath:  "/etc/bind/named.conf",
					domainZonePath: fmt.Sprintf("/etc/bind/zones/db.%s", cfg.DNS.Domain),
					authZonePath:   fmt.Sprintf("/etc/bind/zones/db.%s", cfg.DNS.AuthDomain),
				},
				Env: []string{
					"BIND9_USER=bind",
				},
				RestartPolicy: "unless-stopped",
			}
		},
	})
}

// Start starts the Bind9 DNS server container.
func Start(ctx context.Context, cfg *config.Config) error {
	return service.Start(ctx, cfg)
}

// Stop stops the Bind9 DNS server container.
func Stop(ctx context.Context) error {
	return service.Stop(ctx)
}

// Delete removes the Bind9 DNS server container.
func Delete(ctx context.Context) error {
	return service.Delete(ctx)
}

// IsRunning checks if the Bind9 container is running.
func IsRunning(ctx context.Context) bool {
	return service.IsRunning(ctx)
}
