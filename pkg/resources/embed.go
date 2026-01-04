// Package resources provides access to embedded resources (Helm values, manifests, configs).
package resources

import (
	"embed"
)

// FS holds all embedded resources.
// The go:embed directive will be uncommented when resources are added.
// For now, this is a placeholder that will be populated as tiers are implemented.
//
//go:embed all:placeholder.txt
var FS embed.FS
