// Package bind9 manages the Bind9 DNS server for NOVA domains.
package bind9

import (
	"context"
	"fmt"
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

// Templates for Bind9 configuration files.
const (
	namedConfTemplate = `options {
    directory "/var/cache/bind";
    listen-on { any; };
    forwarders {
        1.1.1.1;
        1.0.0.1;
    };
    allow-query { any; };
};

zone "{{ .Domain }}" {
    type master;
    file "/etc/bind/zones/db.{{ .Domain }}";
};

zone "{{ .AuthDomain }}" {
    type master;
    file "/etc/bind/zones/db.{{ .AuthDomain }}";
};

logging {
    channel query_log {
        stderr;
        severity info;
        print-time yes;
        print-category yes;
        print-severity yes;
    };
    category queries { query_log; };
};
`

	zoneFileTemplate = `$TTL    604800
@       IN      SOA     ns1.{{ .Domain }}. admin.{{ .Domain }}. (
                              2025101601         ; Serial
                              604800             ; Refresh
                              86400              ; Retry
                              2419200            ; Expire
                              604800 )           ; Negative Cache TTL
;
@       IN      NS      ns1.{{ .Domain }}.
@       IN      A       127.0.0.1
*       IN      A       127.0.0.1
ns1     IN      A       127.0.0.1
`
)

// service is the singleton Bind9 service instance.
var service shared.ContainerService

// init initializes the Bind9 service with its configuration.
func init() {
	service = shared.NewContainerService(shared.ContainerServiceConfig{
		ContainerName: containerName,
		Image:         containerImage,
		ConfigDir:     "bind9",
		BuildTemplates: func(cfg *config.Config) []shared.TemplateConfig {
			return []shared.TemplateConfig{
				{
					Name:       "named.conf",
					Content:    namedConfTemplate,
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
					Content:    zoneFileTemplate,
					OutputFile: fmt.Sprintf("zones/db.%s", cfg.DNS.Domain),
					Data:       struct{ Domain string }{Domain: cfg.DNS.Domain},
				},
				{
					Name:       fmt.Sprintf("db.%s", cfg.DNS.AuthDomain),
					Content:    zoneFileTemplate,
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
				Capabilities:  []string{"NET_ADMIN"},
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
