// Package bind9 manages the Bind9 DNS server for NOVA domains.
package bind9

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
	containerName = "nova-bind9-dns"
	containerImage = "ubuntu/bind9:latest"
	dnsPort = "30053"
)

// Start starts the Bind9 DNS server container.
func Start(ctx context.Context, cfg *config.Config) error {
	// Check if container already exists and remove it
	if IsRunning(ctx) {
		if err := exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run(); err != nil {
			return fmt.Errorf("failed to remove existing bind9 container: %w", err)
		}
	}

	// Ensure configuration directory exists
	configDir := filepath.Join(config.ConfigDir(), "bind9")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create bind9 config directory: %w", err)
	}
	zonesDir := filepath.Join(configDir, "zones")
	if err := os.MkdirAll(zonesDir, 0755); err != nil {
		return fmt.Errorf("failed to create bind9 zones directory: %w", err)
	}

	// Generate configuration files
	if err := generateNamedConf(cfg, configDir); err != nil {
		return fmt.Errorf("failed to generate named.conf: %w", err)
	}
	if err := generateZoneFile(cfg, zonesDir, cfg.DNS.Domain); err != nil {
		return fmt.Errorf("failed to generate zone file for %s: %w", cfg.DNS.Domain, err)
	}
	if err := generateZoneFile(cfg, zonesDir, cfg.DNS.AuthDomain); err != nil {
		return fmt.Errorf("failed to generate zone file for %s: %w", cfg.DNS.AuthDomain, err)
	}

	// Start bind9 container
	namedConfPath := filepath.Join(configDir, "named.conf")
	domainZonePath := filepath.Join(zonesDir, fmt.Sprintf("db.%s", cfg.DNS.Domain))
	authZonePath := filepath.Join(zonesDir, fmt.Sprintf("db.%s", cfg.DNS.AuthDomain))

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--cap-add=NET_ADMIN",
		"-p", fmt.Sprintf("%s:53/tcp", dnsPort),
		"-p", fmt.Sprintf("%s:53/udp", dnsPort),
		"-e", "BIND9_USER=bind",
		"-v", fmt.Sprintf("%s:/etc/bind/named.conf:ro", namedConfPath),
		"-v", fmt.Sprintf("%s:/etc/bind/zones/db.%s:ro", domainZonePath, cfg.DNS.Domain),
		"-v", fmt.Sprintf("%s:/etc/bind/zones/db.%s:ro", authZonePath, cfg.DNS.AuthDomain),
		containerImage,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start bind9 container: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Stop stops the Bind9 DNS server container.
func Stop(ctx context.Context) error {
	if !IsRunning(ctx) {
		return nil
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop bind9 container: %w", err)
	}
	return nil
}

// Delete removes the Bind9 DNS server container.
func Delete(ctx context.Context) error {
	if !IsRunning(ctx) && !containerExists(ctx) {
		return nil
	}

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete bind9 container: %w", err)
	}
	return nil
}

// IsRunning checks if the Bind9 container is running.
func IsRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return string(output)[:4] == "true"
}

// containerExists checks if the Bind9 container exists (running or stopped).
func containerExists(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerName)
	return cmd.Run() == nil
}

// generateNamedConf generates the named.conf file.
func generateNamedConf(cfg *config.Config, configDir string) error {
	tmpl := `options {
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

	t, err := template.New("named.conf").Parse(tmpl)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(configDir, "named.conf"))
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

// generateZoneFile generates a zone file for a domain.
func generateZoneFile(cfg *config.Config, zonesDir, domain string) error {
	tmpl := `$TTL    604800
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

	t, err := template.New("zone").Parse(tmpl)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(zonesDir, fmt.Sprintf("db.%s", domain)))
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, struct {
		Domain string
	}{
		Domain: domain,
	})
}
