package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "simple not found",
			err:      NewNotFound("kubectl"),
			expected: "kubectl not found",
		},
		{
			name:     "not found with path",
			err:      NewNotFoundAt("config file", "/etc/nova/config.yaml"),
			expected: "config file not found at /etc/nova/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}
			if !IsNotFound(tt.err) {
				t.Error("IsNotFound() should return true")
			}
		})
	}
}

func TestNotAvailableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "simple not available",
			err:      NewNotAvailable("docker"),
			expected: "docker not available",
		},
		{
			name:     "not available with message",
			err:      NewNotAvailableWithMessage("helm", "please install Helm 3.x"),
			expected: "helm not available: please install Helm 3.x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}
			if !IsNotAvailable(tt.err) {
				t.Error("IsNotAvailable() should return true")
			}
		})
	}
}

func TestAlreadyExistsError(t *testing.T) {
	err := NewAlreadyExists("namespace", "kube-system")
	expected := "namespace kube-system already exists"

	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	if !IsAlreadyExists(err) {
		t.Error("IsAlreadyExists() should return true")
	}
}

func TestNotRunningError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "simple not running",
			err:      NewNotRunning("minikube"),
			expected: "minikube is not running",
		},
		{
			name:     "not running with message",
			err:      NewNotRunningWithMessage("docker daemon", "please start Docker Desktop"),
			expected: "docker daemon is not running: please start Docker Desktop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}
			if !IsNotRunning(tt.err) {
				t.Error("IsNotRunning() should return true")
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "validation without value",
			err:      NewValidation("cpus", "must be at least 2"),
			expected: "validation failed for cpus: must be at least 2",
		},
		{
			name:     "validation with value",
			err:      NewValidationWithValue("memory", "512", "must be at least 2048"),
			expected: "validation failed for memory=512: must be at least 2048",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}
			if !IsValidation(tt.err) {
				t.Error("IsValidation() should return true")
			}
		})
	}
}

func TestDeploymentError(t *testing.T) {
	baseErr := errors.New("connection timeout")

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "deployment without tier",
			err:      NewDeployment("Cilium", baseErr),
			expected: "failed to deploy Cilium",
		},
		{
			name:     "deployment with tier",
			err:      NewDeploymentWithTier("Falco", "1", baseErr),
			expected: "failed to deploy Falco (tier 1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}
			if !IsDeployment(tt.err) {
				t.Error("IsDeployment() should return true")
			}
			// Check that we can unwrap to the base error
			if !errors.Is(tt.err, baseErr) {
				t.Error("errors.Is() should find the wrapped error")
			}
		})
	}
}

func TestConfigurationError(t *testing.T) {
	err := NewConfiguration("dns.domain", "invalid domain format")
	expected := "configuration error in dns.domain: invalid domain format"

	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	if !IsConfiguration(err) {
		t.Error("IsConfiguration() should return true")
	}
}

func TestErrorWrapping(t *testing.T) {
	baseErr := errors.New("base error")
	wrappedErr := &NotFoundError{
		Resource: "kubectl",
		Err:      baseErr,
	}

	// Test that errors.Is works
	if !errors.Is(wrappedErr, baseErr) {
		t.Error("errors.Is should find the wrapped error")
	}

	// Test that errors.As works
	var notFoundErr *NotFoundError
	if !errors.As(wrappedErr, &notFoundErr) {
		t.Error("errors.As should extract the NotFoundError")
	}
}

func TestErrorChaining(t *testing.T) {
	// Create a chain of errors
	baseErr := errors.New("connection refused")
	deployErr := &DeploymentError{
		Component: "Cilium",
		Tier:      "1",
		Err:       baseErr,
	}
	wrappedErr := fmt.Errorf("tier1 deployment failed: %w", deployErr)

	// Test that we can find errors deep in the chain
	if !errors.Is(wrappedErr, baseErr) {
		t.Error("errors.Is should find the base error in the chain")
	}

	var foundDeployErr *DeploymentError
	if !errors.As(wrappedErr, &foundDeployErr) {
		t.Error("errors.As should find DeploymentError in the chain")
	}
	if foundDeployErr.Component != "Cilium" {
		t.Errorf("expected Component=Cilium, got %s", foundDeployErr.Component)
	}
}
