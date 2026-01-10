// Package docker provides a centralized wrapper around the Docker SDK.
//
// This package uses the official Docker SDK (github.com/docker/docker/client)
// to manage containers. All Docker operations in NOVA should go through this
// package to maintain consistency and ease future refactoring.
package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/constants"
)

// Client wraps the Docker SDK client with NOVA-specific functionality.
type Client struct {
	cli      *client.Client
	host     string // Docker daemon host (e.g., "tcp://192.168.49.2:2376")
	certPath string // TLS certificate path for remote daemons
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client) error

// WithHost sets a custom Docker daemon host.
// This is useful for connecting to minikube's Docker daemon.
func WithHost(host, certPath string) ClientOption {
	return func(c *Client) error {
		c.host = host
		c.certPath = certPath
		return nil
	}
}

// NewClient creates a new Docker client.
// By default, it connects to the host's Docker daemon.
// Use WithHost option to connect to a remote daemon (e.g., minikube).
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{}

	// Apply options first to set host/certPath if provided
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Build client options
	clientOpts := []client.Opt{client.WithAPIVersionNegotiation()}

	if c.host != "" {
		// Use custom host
		clientOpts = append(clientOpts, client.WithHost(c.host))
		if c.certPath != "" {
			clientOpts = append(clientOpts, client.WithTLSClientConfig(
				c.certPath+"/ca.pem",
				c.certPath+"/cert.pem",
				c.certPath+"/key.pem",
			))
		}
	} else {
		// Use default FromEnv
		clientOpts = append(clientOpts, client.FromEnv)
	}

	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	c.cli = cli
	return c, nil
}

// Close closes the Docker client connection.
func (c *Client) Close() error {
	return c.cli.Close()
}

// ContainerConfig represents container configuration.
type ContainerConfig struct {
	Name               string
	Image              string
	User                    string            // User and group to run container as (e.g., "1000:1000")
	Ports                   map[string]string // hostPort:containerPort, e.g. "30053:53" or "80:80/tcp"
	Volumes                 map[string]string // host:container
	Env                     []string
	Capabilities            []string
	Privileged              bool              // Run container in privileged mode (required for NFS, mount operations)
	RestartPolicy           string            // "always", "unless-stopped", "on-failure", "no"
	Network                 string            // primary network mode (e.g., "bridge")
	StaticIP                string            // optional static IP for the primary network
	AdditionalNetworks      []string          // additional networks to connect to (e.g., ["minikube"])
	AdditionalNetworksIPs   map[string]string // optional static IPs for additional networks (network name -> IP)
}

// CreateAndStart creates and starts a container.
func (c *Client) CreateAndStart(ctx context.Context, cfg ContainerConfig) error {
	// Pull image if not present
	if err := c.ensureImage(ctx, cfg.Image); err != nil {
		return fmt.Errorf("failed to ensure image %s: %w", cfg.Image, err)
	}

	// Build port bindings
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}
	for hostPort, containerPort := range cfg.Ports {
		// Parse container port - it may be "53" or "53/tcp"
		portNum := containerPort
		protocol := "tcp"

		// Check if protocol is specified (e.g., "53/tcp" or "53/udp")
		for i, c := range containerPort {
			if c == '/' {
				portNum = containerPort[:i]
				protocol = containerPort[i+1:]
				break
			}
		}

		port, err := nat.NewPort(protocol, portNum)
		if err != nil {
			return fmt.Errorf("invalid port %s: %w", containerPort, err)
		}
		exposedPorts[port] = struct{}{}
		portBindings[port] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		}
	}

	// Build volume bindings
	binds := []string{}
	for hostPath, containerPath := range cfg.Volumes {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Build restart policy
	restartPolicy := container.RestartPolicy{}
	switch cfg.RestartPolicy {
	case "always":
		restartPolicy.Name = container.RestartPolicyAlways
	case "unless-stopped":
		restartPolicy.Name = container.RestartPolicyUnlessStopped
	case "on-failure":
		restartPolicy.Name = container.RestartPolicyOnFailure
	default:
		restartPolicy.Name = container.RestartPolicyDisabled
	}

	// Create container
	containerConfig := &container.Config{
		Image:        cfg.Image,
		User:         cfg.User,
		Env:          cfg.Env,
		ExposedPorts: exposedPorts,
	}

	hostConfig := &container.HostConfig{
		PortBindings:  portBindings,
		Binds:         binds,
		RestartPolicy: restartPolicy,
		CapAdd:        cfg.Capabilities,
		Privileged:    cfg.Privileged,
		NetworkMode:   container.NetworkMode(cfg.Network),
	}

	// Configure network settings (static IP if specified)
	var networkingConfig *network.NetworkingConfig
	if cfg.StaticIP != "" {
		networkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				cfg.Network: {
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: cfg.StaticIP,
					},
				},
			},
		}
	}

	resp, err := c.cli.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, cfg.Name)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Connect to additional networks (e.g., minikube network for routing)
	for _, netName := range cfg.AdditionalNetworks {
		// Check if a static IP is specified for this network
		endpointSettings := &network.EndpointSettings{}
		if cfg.AdditionalNetworksIPs != nil {
			if staticIP, ok := cfg.AdditionalNetworksIPs[netName]; ok {
				endpointSettings.IPAMConfig = &network.EndpointIPAMConfig{
					IPv4Address: staticIP,
				}
			}
		}

		if err := c.cli.NetworkConnect(ctx, netName, resp.ID, endpointSettings); err != nil {
			// Log warning but don't fail - network might not exist yet
			// This allows containers to start even if minikube isn't running
			continue
		}
	}

	return nil
}

// ensureImage pulls an image if it doesn't exist locally.
func (c *Client) ensureImage(ctx context.Context, imageName string) error {
	// Check if image exists
	_, err := c.cli.ImageInspect(ctx, imageName)
	if err == nil {
		return nil // Image exists
	}

	// Pull image
	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Read pull output (required for pull to complete)
	_, err = io.Copy(io.Discard, reader)
	return err
}

// Stop stops a running container.
func (c *Client) Stop(ctx context.Context, containerName string) error {
	timeout := 10
	if err := c.cli.ContainerStop(ctx, containerName, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerName, err)
	}
	return nil
}

// Remove removes a container (must be stopped first, or use force).
// Returns nil if the container doesn't exist (idempotent).
func (c *Client) Remove(ctx context.Context, containerName string, force bool) error {
	if err := c.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: force}); err != nil {
		// Ignore "not found" errors - container is already gone
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove container %s: %w", containerName, err)
	}
	return nil
}

// IsRunning checks if a container is running.
func (c *Client) IsRunning(ctx context.Context, containerName string) (bool, error) {
	containerJSON, err := c.cli.ContainerInspect(ctx, containerName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}
	return containerJSON.State.Running, nil
}

// Exists checks if a container exists (running or stopped).
func (c *Client) Exists(ctx context.Context, containerName string) (bool, error) {
	_, err := c.cli.ContainerInspect(ctx, containerName)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}
	return true, nil
}

// Logs retrieves logs from a container.
// Returns the logs as a string, or an error if the container doesn't exist.
func (c *Client) Logs(ctx context.Context, containerName string) (string, error) {
	// Get container logs
	reader, err := c.cli.ContainerLogs(ctx, containerName, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "1000", // Last 1000 lines
		Timestamps: true,
	})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return "", fmt.Errorf("container %s not found", containerName)
		}
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer reader.Close()

	// Read logs
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, reader); err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return buf.String(), nil
}

// PullOptimized pulls an image using the fastest available method.
// It will use skopeo if available and enabled, otherwise falls back to docker pull.
func (c *Client) PullOptimized(ctx context.Context, imageName string, useSkopeo bool, maxConcurrent int) error {
	// Check if image already exists
	_, err := c.cli.ImageInspect(ctx, imageName)
	if err == nil {
		ui.Debug("Image %s already exists locally", imageName)
		return nil
	}

	// Try skopeo if available and enabled
	if useSkopeo && isSkopeoCopyAvailable() {
		ui.Debug("Using skopeo for optimized pull (max-concurrent-layers=%d)", constants.SkopeoConcurrentLayers)
		if err := c.pullWithSkopeo(ctx, imageName); err != nil {
			ui.Warn("Skopeo pull failed, falling back to docker: %v", err)
			return c.pullWithDocker(ctx, imageName)
		}
		return nil
	}

	// Fallback to docker pull
	return c.pullWithDocker(ctx, imageName)
}

// pullWithSkopeo uses skopeo copy to pull an image directly into the Docker daemon.
// This is significantly faster for large images due to optimized layer downloads.
func (c *Client) pullWithSkopeo(ctx context.Context, imageName string) error {
	// Normalize image name (add docker.io prefix if needed)
	srcImage := imageName
	if !strings.Contains(srcImage, "/") {
		srcImage = "docker.io/library/" + srcImage
	} else if !strings.HasPrefix(srcImage, "docker.io/") && !strings.HasPrefix(srcImage, "ghcr.io/") && !strings.HasPrefix(srcImage, "quay.io/") {
		srcImage = "docker.io/" + srcImage
	}

	// Determine the destination daemon host
	daemonHost := "unix:///var/run/docker.sock"
	if c.host != "" {
		// Use custom host (e.g., minikube's Docker daemon)
		daemonHost = c.host
		ui.Debug("Using custom Docker daemon for skopeo: %s", daemonHost)
	}

	// Build skopeo command
	args := []string{
		"copy",
		"--override-os", "linux",
		"--override-arch", "amd64",
		"--dest-daemon-host", daemonHost,
	}

	// Add TLS cert path if using remote daemon
	if c.certPath != "" {
		args = append(args, "--dest-cert-dir", c.certPath)
	}

	args = append(args,
		fmt.Sprintf("docker://%s", srcImage),
		fmt.Sprintf("docker-daemon:%s", imageName),
	)

	cmd := exec.CommandContext(ctx, "skopeo", args...)

	// Capture output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo copy failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// pullWithDocker uses the standard Docker pull mechanism.
func (c *Client) pullWithDocker(ctx context.Context, imageName string) error {
	ui.Debug("Using docker pull for %s", imageName)

	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Read pull output (required for pull to complete)
	_, err = io.Copy(io.Discard, reader)
	return err
}

// isSkopeoCopyAvailable checks if skopeo binary is available in PATH.
func isSkopeoCopyAvailable() bool {
	_, err := exec.LookPath("skopeo")
	return err == nil
}

// RemoveNetwork removes a Docker network by name.
// Returns nil if the network doesn't exist (idempotent).
func (c *Client) RemoveNetwork(ctx context.Context, networkName string) error {
	if err := c.cli.NetworkRemove(ctx, networkName); err != nil {
		// Ignore "not found" errors - network is already gone
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove network %s: %w", networkName, err)
	}
	return nil
}

// CreateNetwork creates a Docker network with the specified name.
// Returns nil if the network already exists (idempotent).
func (c *Client) CreateNetwork(ctx context.Context, networkName string) error {
	// Check if network already exists
	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		if net.Name == networkName {
			// Network already exists
			return nil
		}
	}

	// Create network with default bridge driver
	_, err = c.cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
		Options: map[string]string{
			"com.docker.network.bridge.name": networkName,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", networkName, err)
	}

	return nil
}

// GetNetworkSubnet returns the subnet CIDR of a Docker network.
// Returns empty string if the network doesn't exist or has no subnet configured.
func (c *Client) GetNetworkSubnet(ctx context.Context, networkName string) (string, error) {
	// Inspect the network
	resource, err := c.cli.NetworkInspect(ctx, networkName, network.InspectOptions{})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return "", fmt.Errorf("network %s not found", networkName)
		}
		return "", fmt.Errorf("failed to inspect network %s: %w", networkName, err)
	}

	// Get the first subnet from IPAM config
	if len(resource.IPAM.Config) == 0 {
		return "", fmt.Errorf("network %s has no IPAM configuration", networkName)
	}

	subnet := resource.IPAM.Config[0].Subnet
	if subnet == "" {
		return "", fmt.Errorf("network %s has no subnet configured", networkName)
	}

	return subnet, nil
}

// GetNetworkGateway returns the gateway IP of a Docker network.
func (c *Client) GetNetworkGateway(ctx context.Context, networkName string) (string, error) {
	// Inspect the network
	resource, err := c.cli.NetworkInspect(ctx, networkName, network.InspectOptions{})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return "", fmt.Errorf("network %s not found", networkName)
		}
		return "", fmt.Errorf("failed to inspect network %s: %w", networkName, err)
	}

	// Get gateway from IPAM config
	if len(resource.IPAM.Config) == 0 {
		return "", fmt.Errorf("network %s has no IPAM configuration", networkName)
	}

	gateway := resource.IPAM.Config[0].Gateway
	if gateway == "" {
		return "", fmt.Errorf("network %s has no gateway configured", networkName)
	}

	return gateway, nil
}
