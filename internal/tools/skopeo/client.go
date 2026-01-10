// Package skopeo provides a client for efficient image copying operations.
// Skopeo is used to copy container images between registries with minimal
// RAM usage by streaming layers rather than loading entire images into memory.
//
// This client runs skopeo inside a Docker container on the nova network,
// allowing direct access to the local registry without port exposure.
package skopeo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	pki "github.com/kzgrzendek/nova/internal/setup/certificates"
)

// Client provides methods for skopeo operations.
type Client struct{}

// NewClient creates a new skopeo client.
func NewClient() *Client {
	return &Client{}
}

// CopyToRegistryOptions contains options for copying images to a registry.
type CopyToRegistryOptions struct {
	// SourceImage is the full source image reference (e.g., "ghcr.io/llm-d/image:tag")
	SourceImage string

	// DestRegistry is the destination registry host (e.g., "registry.local:5000")
	DestRegistry string

	// DestImage is the image name in the destination registry (e.g., "llm-d/image:tag")
	DestImage string

	// InsecureDestRegistry allows using an insecure (HTTP) destination registry
	InsecureDestRegistry bool

	// SkipTLSVerify skips TLS verification for the destination registry
	SkipTLSVerify bool
}

// CopyToRegistry copies an image from a source registry to the local registry.
// This operation streams image layers and uses minimal RAM (~300MB).
// Runs skopeo inside a Docker container on the nova network with TLS support.
func (c *Client) CopyToRegistry(ctx context.Context, opts CopyToRegistryOptions) error {
	sourceRef := fmt.Sprintf("docker://%s", opts.SourceImage)
	destRef := fmt.Sprintf("docker://%s/%s", opts.DestRegistry, opts.DestImage)

	ui.Debug("Copying image via skopeo (Docker container with TLS):")
	ui.Debug("  Source: %s", sourceRef)
	ui.Debug("  Destination: %s", destRef)

	// Get the registry container IP for DNS resolution in skopeo container
	registryIP, err := getRegistryContainerIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get registry container IP: %w", err)
	}
	ui.Debug("  Registry IP: %s", registryIP)

	// Get mkcert CA certificate for mounting (if TLS verification is enabled)
	var combinedCAPath string
	if !opts.SkipTLSVerify {
		caCertPath, _, err := pki.GetCAPaths()
		if err != nil {
			return fmt.Errorf("failed to get mkcert CA certificate: %w", err)
		}

		// Create a combined CA bundle with system CAs + our custom CA
		// This ensures skopeo can verify both the source registry (ghcr.io, etc.)
		// and our local registry
		combinedCAPath, err = createCombinedCABundle(caCertPath)
		if err != nil {
			return fmt.Errorf("failed to create combined CA bundle: %w", err)
		}
		defer os.Remove(combinedCAPath)

		ui.Debug("Using combined CA bundle: %s", combinedCAPath)
	}

	// Build skopeo arguments
	skopeoArgs := []string{
		"copy",
		"--retry-times", "3",
	}

	// Add destination TLS options
	if opts.SkipTLSVerify {
		skopeoArgs = append(skopeoArgs, "--dest-tls-verify=false")
	}

	if opts.InsecureDestRegistry {
		skopeoArgs = append(skopeoArgs, "--dest-no-creds")
	}

	// Add source and destination
	skopeoArgs = append(skopeoArgs, sourceRef, destRef)

	// Run skopeo inside a Docker container on the nova network
	// Add DNS mapping for registry.local to resolve to the registry container IP
	dockerArgs := []string{
		"run",
		"--rm",              // Remove container after completion
		"--network", "nova", // Connect to nova network
		"--add-host", fmt.Sprintf("registry.local:%s", registryIP), // Map domain to container IP
	}

	// Mount the combined CA bundle and set environment variable for TLS verification
	// Using SSL_CERT_FILE to specify the CA bundle location (includes system CAs + custom CA)
	if !opts.SkipTLSVerify {
		mountSpec := fmt.Sprintf("type=bind,source=%s,target=/etc/ssl/certs/combined-ca-bundle.crt,readonly", combinedCAPath)
		dockerArgs = append(dockerArgs, "--mount", mountSpec)
		dockerArgs = append(dockerArgs, "-e", "SSL_CERT_FILE=/etc/ssl/certs/combined-ca-bundle.crt")
	}

	dockerArgs = append(dockerArgs, "quay.io/skopeo/stable:latest")
	dockerArgs = append(dockerArgs, skopeoArgs...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	// Capture output only for error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Only show output on error
		return fmt.Errorf("skopeo copy failed: %w\nOutput:\n%s", err, string(output))
	}

	ui.Debug("Image copied successfully to registry")
	return nil
}

// Inspect retrieves detailed information about an image without downloading it.
func (c *Client) Inspect(ctx context.Context, imageRef string) (string, error) {
	cmd := exec.CommandContext(ctx, "skopeo", "inspect", fmt.Sprintf("docker://%s", imageRef))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to inspect image %s: %w\nOutput: %s", imageRef, err, string(output))
	}

	return string(output), nil
}

// createCombinedCABundle creates a temporary CA bundle that includes both system CAs
// and the custom mkcert CA. This allows skopeo to verify both public registries
// (like ghcr.io) and our local registry.
func createCombinedCABundle(customCAPath string) (string, error) {
	// Read the custom CA certificate
	customCA, err := os.ReadFile(customCAPath)
	if err != nil {
		return "", fmt.Errorf("failed to read custom CA: %w", err)
	}

	// Try to read the system CA bundle from common locations
	systemCALocations := []string{
		"/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu
		"/etc/pki/tls/certs/ca-bundle.crt",   // RHEL/CentOS
		"/etc/ssl/ca-bundle.pem",             // OpenSUSE
		"/etc/ssl/cert.pem",                  // Alpine
	}

	var systemCA []byte
	for _, location := range systemCALocations {
		systemCA, err = os.ReadFile(location)
		if err == nil {
			ui.Debug("Using system CA bundle from: %s", location)
			break
		}
	}

	// If we couldn't find system CAs, just use the custom CA
	if systemCA == nil {
		ui.Debug("Could not find system CA bundle, using only custom CA")
		systemCA = []byte{}
	}

	// Create a temporary file with combined CAs
	tmpFile, err := os.CreateTemp("", "combined-ca-*.crt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	// Write system CAs first, then custom CA
	if _, err := tmpFile.Write(systemCA); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write system CAs: %w", err)
	}

	// Add a newline separator if system CAs exist
	if len(systemCA) > 0 {
		if _, err := tmpFile.Write([]byte("\n")); err != nil {
			os.Remove(tmpFile.Name())
			return "", fmt.Errorf("failed to write separator: %w", err)
		}
	}

	if _, err := tmpFile.Write(customCA); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write custom CA: %w", err)
	}

	return tmpFile.Name(), nil
}

// getRegistryContainerIP retrieves the IP address of the registry container on the nova network.
// This IP is used to configure DNS resolution in the skopeo container.
func getRegistryContainerIP(ctx context.Context) (string, error) {
	// Use docker inspect to get the container's IP on the nova network
	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.NetworkSettings.Networks.nova.IPAddress}}",
		"nova-registry")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to inspect registry container: %w", err)
	}

	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", fmt.Errorf("registry container not found on nova network")
	}

	return ip, nil
}
