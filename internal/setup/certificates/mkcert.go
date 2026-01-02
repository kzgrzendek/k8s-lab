package pki

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallRootCA runs mkcert -install to install the local Root CA.
func InstallRootCA() error {
	// Verify mkcert is available
	mkcertPath, err := exec.LookPath("mkcert")
	if err != nil {
		return fmt.Errorf("mkcert not found in PATH - please install it first")
	}

	// Run mkcert -install
	cmd := exec.Command(mkcertPath, "-install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert -install failed: %w", err)
	}

	return nil
}

// GetCARoot returns the path to the mkcert CA root directory.
func GetCARoot() (string, error) {
	cmd := exec.Command("mkcert", "-CAROOT")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get CAROOT: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCAPaths returns the paths to the CA certificate and key.
func GetCAPaths() (certPath, keyPath string, err error) {
	caRoot, err := GetCARoot()
	if err != nil {
		return "", "", err
	}

	certPath = filepath.Join(caRoot, "rootCA.pem")
	keyPath = filepath.Join(caRoot, "rootCA-key.pem")

	// Verify files exist
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("CA cert not found at %s - run mkcert -install first", certPath)
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("CA key not found at %s - run mkcert -install first", keyPath)
	}

	return certPath, keyPath, nil
}

// IsInstalled checks if mkcert CA is installed.
func IsInstalled() (bool, error) {
	caRoot, err := GetCARoot()
	if err != nil {
		return false, err
	}

	certPath := filepath.Join(caRoot, "rootCA.pem")
	keyPath := filepath.Join(caRoot, "rootCA-key.pem")

	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	return certErr == nil && keyErr == nil, nil
}

// GetCAInfo returns information about the installed CA.
func GetCAInfo() (string, error) {
	caRoot, err := GetCARoot()
	if err != nil {
		return "", err
	}

	certPath := filepath.Join(caRoot, "rootCA.pem")

	// Read certificate
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read CA cert: %w", err)
	}

	// Use openssl to get cert info
	cmd := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-subject", "-issuer", "-dates")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to just showing the path and size
		return fmt.Sprintf("CA Root: %s\nCertificate size: %d bytes", caRoot, len(certData)), nil
	}

	return fmt.Sprintf("CA Root: %s\n%s", caRoot, string(output)), nil
}
