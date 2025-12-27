// Package resources provides access to embedded resources (Helm values, manifests, configs).
package resources

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
)

// FS holds all embedded resources.
// The go:embed directive will be uncommented when resources are added.
// For now, this is a placeholder that will be populated as tiers are implemented.
//
//go:embed all:placeholder.txt
var FS embed.FS

// GetHelmValues returns the Helm values file for a component.
func GetHelmValues(tier, component string) ([]byte, error) {
	path := fmt.Sprintf("helm/%s/%s.yaml", tier, component)
	return FS.ReadFile(path)
}

// GetManifests returns all manifest files from a directory.
func GetManifests(tier, component string) ([][]byte, error) {
	dir := fmt.Sprintf("manifests/%s/%s", tier, component)

	var manifests [][]byte
	err := fs.WalkDir(FS, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := path.Ext(p)
		if ext == ".yaml" || ext == ".yml" {
			data, err := FS.ReadFile(p)
			if err != nil {
				return err
			}
			manifests = append(manifests, data)
		}
		return nil
	})

	return manifests, err
}

// GetNginxTemplate returns the NGINX configuration template.
func GetNginxTemplate() ([]byte, error) {
	return FS.ReadFile("nginx/nginx.conf.tmpl")
}

// GetBind9Config returns Bind9 configuration files.
func GetBind9Config() (map[string][]byte, error) {
	configs := make(map[string][]byte)

	// Read named.conf
	data, err := FS.ReadFile("bind9/named.conf")
	if err != nil {
		return nil, err
	}
	configs["named.conf"] = data

	// Read zone files
	err = fs.WalkDir(FS, "bind9/zones", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := FS.ReadFile(p)
		if err != nil {
			return err
		}
		configs[path.Base(p)] = data
		return nil
	})

	return configs, err
}

// FileExists checks if an embedded file exists.
func FileExists(path string) bool {
	_, err := FS.ReadFile(path)
	return err == nil
}
