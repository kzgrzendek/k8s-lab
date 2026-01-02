package status

import (
	"context"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
)

func TestNewChecker(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	checker := NewChecker(ctx, cfg)

	if checker == nil {
		t.Fatal("expected checker to be non-nil")
	}
	if checker.ctx != ctx {
		t.Error("expected context to be set")
	}
	if checker.cfg != cfg {
		t.Error("expected config to be set")
	}
}

func TestGetSystemStatus(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Minikube: config.MinikubeConfig{
			Nodes:  3,
			CPUs:   4,
			Memory: 8192,
			GPUs:   "none",
		},
		DNS: config.DNSConfig{
			Domain:     "k8s.test",
			AuthDomain: "auth.k8s.test",
			Bind9Port:  5353,
		},
	}

	checker := NewChecker(ctx, cfg)
	status, err := checker.GetSystemStatus()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if status == nil {
		t.Fatal("expected status to be non-nil")
	}

	if status.Config != cfg {
		t.Error("expected config to be set in status")
	}

	if status.Cluster == nil {
		t.Error("expected cluster status to be set")
	}

	if status.HostServices == nil {
		t.Error("expected host services status to be set")
	}

	// Deployments may be nil if cluster is not running
	if status.Cluster.Running && status.Deployments == nil {
		t.Error("expected deployments status when cluster is running")
	}
}

func TestGetHostServicesStatus(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		DNS: config.DNSConfig{
			Bind9Port: 5353,
		},
	}

	checker := NewChecker(ctx, cfg)
	services := checker.GetHostServicesStatus()

	if services == nil {
		t.Fatal("expected services to be non-nil")
	}

	// Bind9 should be checked
	if services.Bind9.Name != "Bind9 DNS" {
		t.Errorf("expected Bind9 name to be 'Bind9 DNS', got: %s", services.Bind9.Name)
	}

	// NGINX should be checked
	if services.NGINX.Name != "NGINX Gateway" {
		t.Errorf("expected NGINX name to be 'NGINX Gateway', got: %s", services.NGINX.Name)
	}

	// Status should be either running or stopped
	validStatuses := map[string]bool{"running": true, "stopped": true}
	if !validStatuses[services.Bind9.Status] {
		t.Errorf("expected Bind9 status to be 'running' or 'stopped', got: %s", services.Bind9.Status)
	}
	if !validStatuses[services.NGINX.Status] {
		t.Errorf("expected NGINX status to be 'running' or 'stopped', got: %s", services.NGINX.Status)
	}
}

func TestComponentStatus(t *testing.T) {
	comp := ComponentStatus{
		Name:    "Test Component",
		Status:  "running",
		Details: "test details",
		Healthy: true,
	}

	if comp.Name != "Test Component" {
		t.Errorf("expected name 'Test Component', got: %s", comp.Name)
	}
	if comp.Status != "running" {
		t.Errorf("expected status 'running', got: %s", comp.Status)
	}
	if comp.Details != "test details" {
		t.Errorf("expected details 'test details', got: %s", comp.Details)
	}
	if !comp.Healthy {
		t.Error("expected healthy to be true")
	}
}

func TestClusterStatusNotRunning(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Minikube: config.MinikubeConfig{
			GPUs: "nvidia",
		},
	}

	checker := NewChecker(ctx, cfg)
	cluster, err := checker.GetClusterStatus()

	// This should not error even if cluster is not running
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cluster == nil {
		t.Fatal("expected cluster status to be non-nil")
	}

	// GPU config should be set
	if cluster.GPU != "nvidia" {
		t.Errorf("expected GPU to be 'nvidia', got: %s", cluster.GPU)
	}
}
