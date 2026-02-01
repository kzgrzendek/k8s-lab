// Package shared provides shared utilities for deployment operations.
package shared

import "github.com/kzgrzendek/nova/internal/core/config"

// ResourcesAsHelmValues converts ResourceRequirements to a Helm-compatible map structure.
// The returned map can be merged into Helm values for the "resources" key.
func ResourcesAsHelmValues(r config.ResourceRequirements) map[string]any {
	result := make(map[string]any)

	// Build requests
	requests := make(map[string]any)
	if r.Requests.CPU != "" {
		requests["cpu"] = r.Requests.CPU
	}
	if r.Requests.Memory != "" {
		requests["memory"] = r.Requests.Memory
	}
	if len(requests) > 0 {
		result["requests"] = requests
	}

	// Build limits
	limits := make(map[string]any)
	if r.Limits.CPU != "" {
		limits["cpu"] = r.Limits.CPU
	}
	if r.Limits.Memory != "" {
		limits["memory"] = r.Limits.Memory
	}
	if len(limits) > 0 {
		result["limits"] = limits
	}

	return result
}

// MergeResourcesIntoValues adds resources to a Helm values map at the specified path.
// The path is a dot-separated string (e.g., "controller.resources").
// If the path is empty, resources are added at the root "resources" key.
func MergeResourcesIntoValues(values map[string]any, r config.ResourceRequirements, path string) map[string]any {
	if values == nil {
		values = make(map[string]any)
	}

	resources := ResourcesAsHelmValues(r)
	if len(resources) == 0 {
		return values
	}

	if path == "" {
		values["resources"] = resources
		return values
	}

	// Navigate/create the path
	current := values
	parts := splitPath(path)
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - set the resources
			current[part] = resources
		} else {
			// Intermediate part - navigate or create
			if _, ok := current[part]; !ok {
				current[part] = make(map[string]any)
			}
			if next, ok := current[part].(map[string]any); ok {
				current = next
			} else {
				// Can't navigate further, create new map
				newMap := make(map[string]any)
				current[part] = newMap
				current = newMap
			}
		}
	}

	return values
}

// splitPath splits a dot-separated path into parts.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for i, c := range path {
		if c == '.' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}
