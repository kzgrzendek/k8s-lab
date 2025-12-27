package pki

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kzgrzendek/nova/internal/config"
)

// GenerateKubernetesSecret creates a Kubernetes TLS secret YAML from the mkcert CA.
// The secret will be used by cert-manager to issue certificates.
func GenerateKubernetesSecret(outputPath string) error {
	// Get CA cert and key paths
	certPath, keyPath, err := GetCAPaths()
	if err != nil {
		return err
	}

	// Read certificate
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read CA cert: %w", err)
	}

	// Read key
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA key: %w", err)
	}

	// Base64 encode for Kubernetes secret
	certB64 := base64.StdEncoding.EncodeToString(certData)
	keyB64 := base64.StdEncoding.EncodeToString(keyData)

	// Generate secret YAML
	secretYAML := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: nova-ca-secret
  namespace: cert-manager
type: kubernetes.io/tls
data:
  tls.crt: %s
  tls.key: %s
`, certB64, keyB64)

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, []byte(secretYAML), 0600); err != nil {
		return fmt.Errorf("failed to write secret YAML: %w", err)
	}

	return nil
}

// GetDefaultSecretPath returns the default path for the CA secret YAML.
func GetDefaultSecretPath() string {
	return filepath.Join(config.ConfigDir(), "ca-secret.yaml")
}
