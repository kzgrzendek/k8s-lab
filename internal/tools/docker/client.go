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
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Client wraps the Docker SDK client with NOVA-specific functionality.
type Client struct {
	cli *client.Client
}

// NewClient creates a new Docker client.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close closes the Docker client connection.
func (c *Client) Close() error {
	return c.cli.Close()
}

// ContainerConfig represents container configuration.
type ContainerConfig struct {
	Name               string
	Image              string
	Ports              map[string]string // hostPort:containerPort, e.g. "30053:53" or "80:80/tcp"
	Volumes            map[string]string // host:container
	Env                []string
	Capabilities       []string
	RestartPolicy      string   // "always", "unless-stopped", "on-failure", "no"
	Network            string   // primary network mode (e.g., "bridge")
	AdditionalNetworks []string // additional networks to connect to (e.g., ["minikube"])
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
		Env:          cfg.Env,
		ExposedPorts: exposedPorts,
	}

	hostConfig := &container.HostConfig{
		PortBindings:  portBindings,
		Binds:         binds,
		RestartPolicy: restartPolicy,
		CapAdd:        cfg.Capabilities,
		NetworkMode:   container.NetworkMode(cfg.Network),
	}

	resp, err := c.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, cfg.Name)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Connect to additional networks (e.g., minikube network for routing)
	for _, netName := range cfg.AdditionalNetworks {
		if err := c.cli.NetworkConnect(ctx, netName, resp.ID, &network.EndpointSettings{}); err != nil {
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
