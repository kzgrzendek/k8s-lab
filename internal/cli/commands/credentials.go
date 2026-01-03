package commands

import (
	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier2"
)

// displayKeycloakCredentials shows all Keycloak user credentials organized by realm and role.
func displayKeycloakCredentials(credentials *tier2.DeployResult, cfg *config.Config) {
	ui.Info("")
	ui.Header("Credentials")
	ui.Info("URL: https://%s", cfg.DNS.AuthDomain)
	ui.Info("")

	// Find and display each credential type explicitly
	var clusterAdmin, admin, developer, user *tier2.KeycloakUser
	for i := range credentials.KeycloakUsers {
		u := &credentials.KeycloakUsers[i]
		switch u.Username {
		case "cluster-admin":
			clusterAdmin = u
		case "admin":
			admin = u
		case "developer":
			developer = u
		case "user":
			user = u
		}
	}

	// Display Cluster Admin (Master Realm)
	if clusterAdmin != nil {
		ui.Info("Cluster Admin (Master Realm):")
		ui.Info("  %s / %s", clusterAdmin.Username, clusterAdmin.Password)
		ui.Info("")
	}

	// Display Nova Realm Users
	ui.Info("Nova Realm:")
	if admin != nil {
		ui.Info("  Admin:     %s / %s", admin.Username, admin.Password)
	}
	if developer != nil {
		ui.Info("  Developer: %s / %s", developer.Username, developer.Password)
	}
	if user != nil {
		ui.Info("  User:      %s / %s", user.Username, user.Password)
	}
}
