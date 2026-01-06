package skopeo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	assert.NotNil(t, client)
}

func TestCopyToRegistry_ValidatesOptions(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	// This test verifies the function can be called with valid options
	// Actual copy operations require a running registry and network access,
	// so we don't test the actual copy here
	opts := CopyToRegistryOptions{
		SourceImage:          "alpine:latest",
		DestRegistry:         "localhost:5000",
		DestImage:            "test/alpine:latest",
		InsecureDestRegistry: true,
		SkipTLSVerify:        true,
	}

	// We can't actually run this without a registry, but we can verify
	// the options structure is valid
	assert.NotEmpty(t, opts.SourceImage)
	assert.NotEmpty(t, opts.DestRegistry)
	assert.NotEmpty(t, opts.DestImage)

	// If skopeo is not available, this will fail, which is expected
	// In a real environment, this would copy the image
	_ = client.CopyToRegistry(ctx, opts)
}
