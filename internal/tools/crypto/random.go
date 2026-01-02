// Package crypto provides cryptographic utilities for NOVA.
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateRandomPassword generates a cryptographically secure random password.
// The length parameter specifies the number of random bytes (the actual password
// will be longer due to base64 encoding).
func GenerateRandomPassword(length int) (string, error) {
	if length < 16 {
		length = 16 // Minimum 16 bytes for security
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Use URL-safe base64 encoding to ensure compatibility with Kubernetes secrets
	// Remove padding to avoid '=' characters
	password := base64.RawURLEncoding.EncodeToString(bytes)
	return password, nil
}

// GenerateRandomBytes generates cryptographically secure random bytes.
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// GenerateRandomHex generates a random hexadecimal string.
func GenerateRandomHex(length int) (string, error) {
	bytes, err := GenerateRandomBytes(length)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}
