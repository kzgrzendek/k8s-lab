// Package tier2 handles the deployment of NOVA Tier 2 (Platform Services).
package tier2

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/tools/crypto"
	"github.com/kzgrzendek/nova/internal/tools/helm"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// KeycloakUser represents a Keycloak user with credentials.
type KeycloakUser struct {
	Username string
	Password string
	Group    string
}

// DeployResult contains the results of a Tier 2 deployment.
// All credentials are centralized in Keycloak.
type DeployResult struct {
	// KeycloakUsers contains all Keycloak users with their credentials
	KeycloakUsers []KeycloakUser
}

// Use namespace constants from the constants package
var (
	kyvernoNamespace         = constants.NamespaceKyverno
	keycloakNamespace        = constants.NamespaceKeycloak
	victorialogsNamespace    = constants.NamespaceVictoriaLogs
	victoriametricsNamespace = constants.NamespaceVictoriaMetrics
)

// deploymentResult stores credentials during deployment for later retrieval.
var deploymentResult *DeployResult

// DeployTier2 deploys tier 2: Platform services (Kyverno, Hubble, Victoria Stack).
// Returns DeployResult containing generated credentials.
func DeployTier2(ctx context.Context, cfg *config.Config) (*DeployResult, error) {
	ui.Header("Tier 2: Platform Services")

	// Initialize result to track credentials
	deploymentResult = &DeployResult{
		KeycloakUsers: []KeycloakUser{},
	}

	// Add Tier 2 Helm repositories
	if err := addTier2HelmRepos(ctx); err != nil {
		return nil, fmt.Errorf("failed to add Tier 2 Helm repositories: %w", err)
	}

	// Define deployment steps
	steps := []string{
		"Kyverno Policy Engine",
		"Keycloak Operator",
		"Keycloak PostgreSQL Database",
		"Keycloak Theme",
		"Keycloak IAM Instance",
		"Hubble Network Observability",
		"VictoriaLogs Server",
		"VictoriaLogs Collector",
		"VictoriaMetrics Stack",
	}

	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Step 1: Deploy Kyverno
	if err := deployKyverno(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}
	currentStep++

	// Step 2: Deploy Keycloak Operator
	if err := deployKeycloakOperator(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}
	currentStep++

	// Step 3: Deploy Keycloak PostgreSQL
	if err := deployKeycloakPostgreSQL(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}
	currentStep++

	// Step 4: Deploy Keycloak Theme
	if err := deployKeycloakTheme(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}
	currentStep++

	// Step 5: Deploy Keycloak Instance
	keycloakAdminPassword, err := deployKeycloakInstance(ctx, cfg, progress, currentStep)
	if err != nil {
		return nil, err
	}
	// Store all Keycloak users with their credentials
	// admin has a random generated password, developer and user have static passwords
	deploymentResult.KeycloakUsers = []KeycloakUser{
		{Username: "admin", Password: keycloakAdminPassword, Group: "ADMIN"},
		{Username: "developer", Password: "developer", Group: "DEVELOPER"},
		{Username: "user", Password: "user", Group: "USER"},
	}
	currentStep++

	// Step 5: Deploy Hubble
	if err := deployHubble(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}
	currentStep++

	// Step 6: Deploy VictoriaLogs Server
	if err := deployVictoriaLogsServer(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}
	currentStep++

	// Step 7: Deploy VictoriaLogs Collector
	if err := deployVictoriaLogsCollector(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}
	currentStep++

	// Step 8: Deploy VictoriaMetrics Stack
	if err := deployVictoriaMetricsStack(ctx, cfg, progress, currentStep); err != nil {
		return nil, err
	}

	progress.Complete()

	ui.Header("Tier 2 Deployment Complete")
	ui.Success("Platform services are running")
	ui.Info("Keycloak: https://keycloak.%s", cfg.DNS.Domain)
	ui.Info("Hubble: https://hubble.%s", cfg.DNS.Domain)
	ui.Info("Grafana: https://grafana.%s", cfg.DNS.Domain)

	return deploymentResult, nil
}

// deployKyverno deploys Kyverno policy engine.
func deployKyverno(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	return deployWithProgress(ctx, progress, step, shared.HelmDeploymentOptions{
		ReleaseName:     "kyverno",
		ChartRef:        "kyverno/kyverno",
		Namespace:       kyvernoNamespace,
		ValuesPath:      "resources/core/deployment/tier2/kyverno/values.yaml",
		Wait:            true,
		TimeoutSeconds:  300,
		CreateNamespace: true,
	}, nil)
}

// deployKeycloakOperator deploys Keycloak operator CRDs and controller.
func deployKeycloakOperator(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	progress.StartStep(step)

	// Create namespace
	if err := k8s.CreateNamespace(ctx, keycloakNamespace); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create keycloak namespace: %w", err)
	}

	// Label namespace
	if err := k8s.LabelNamespace(ctx, keycloakNamespace, "service-type", "keycloak"); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to label keycloak namespace: %w", err)
	}

	if err := k8s.LabelNamespace(ctx, keycloakNamespace, "trust-manager/inject-ca-secret", "enabled"); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to label keycloak namespace for CA injection: %w", err)
	}

	// Apply Keycloak CRDs and Operator (all with -n keycloak as per script)
	ui.Info("Installing Keycloak CRDs...")
	if err := k8s.ApplyURLWithNamespace(ctx, constants.ManifestKeycloakCRD, keycloakNamespace); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to apply Keycloak CRD: %w", err)
	}

	if err := k8s.ApplyURLWithNamespace(ctx, constants.ManifestKeycloakRealmImportCRD, keycloakNamespace); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to apply KeycloakRealmImport CRD: %w", err)
	}

	ui.Info("Installing Keycloak Operator...")
	if err := k8s.ApplyURLWithNamespace(ctx, constants.ManifestKeycloakOperator, keycloakNamespace); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to apply Keycloak Operator: %w", err)
	}

	// Wait for operator to be ready
	ui.Info("Waiting for Keycloak Operator...")
	if err := k8s.WaitForDeploymentReady(ctx, keycloakNamespace, "keycloak-operator", 300); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("keycloak operator failed to become ready: %w", err)
	}

	progress.CompleteStep(step)
	return nil
}

// deployKeycloakPostgreSQL deploys PostgreSQL database for Keycloak.
func deployKeycloakPostgreSQL(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	progress.StartStep(step)

	// Generate PostgreSQL credentials
	postgresPassword, err := crypto.GenerateRandomPassword(32)
	if err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to generate postgres password: %w", err)
	}

	dbData := map[string]interface{}{
		"PostgresUser":     "admin",
		"PostgresPassword": postgresPassword,
	}

	// Create Keycloak DB secret
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/secrets/keycloak-db-secret.yaml", dbData); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create keycloak db secret: %w", err)
	}

	// Deploy PostgreSQL StatefulSet
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/postgresql/statefulset.yaml", dbData); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to deploy postgresql statefulset: %w", err)
	}

	// Deploy PostgreSQL Service
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/postgresql/service.yaml", nil); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to deploy postgresql service: %w", err)
	}

	// Wait for PostgreSQL to be ready
	if err := k8s.WaitForDeploymentReady(ctx, keycloakNamespace, "postgresql-db", 300); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("postgresql failed to become ready: %w", err)
	}

	progress.CompleteStep(step)
	return nil
}

// Theme JAR path relative to project root
const keycloakThemeJARPath = "resources/core/deployment/tier2/keycloak/theme/dist_keycloak/keycloak-theme-for-kc-all-other-versions.jar"

// deployKeycloakTheme installs the NOVA Keycloak theme by copying the JAR to a PVC.
// The theme is persisted so it survives nova stop/start cycles.
func deployKeycloakTheme(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	progress.StartStep(step)

	const pvcName = "keycloak-theme-pvc"
	const helperPodName = "theme-copy-helper"

	// Check if PVC already exists with the theme (skip if already installed)
	if k8s.PVCExists(ctx, keycloakNamespace, pvcName) {
		ui.Info("Keycloak theme PVC already exists - skipping installation")
		progress.CompleteStep(step)
		return nil
	}

	// Create PVC for theme storage
	ui.Info("Creating theme PVC...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/theme-pvc/theme-pvc.yaml", nil); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create theme PVC: %w", err)
	}

	// Create helper pod to mount the PVC
	ui.Info("Creating helper pod for theme copy...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/theme-pvc/theme-copy-pod.yaml", nil); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create theme copy helper pod: %w", err)
	}

	// Wait for helper pod to be ready
	ui.Info("Waiting for helper pod...")
	if err := k8s.WaitForPodReady(ctx, keycloakNamespace, helperPodName, 120); err != nil {
		// Clean up the pod on failure
		_ = k8s.DeletePod(ctx, keycloakNamespace, helperPodName)
		progress.FailStep(step, err)
		return fmt.Errorf("helper pod failed to become ready: %w", err)
	}

	// Copy the theme JAR to the PVC via the helper pod
	ui.Info("Copying theme JAR to PVC...")
	if err := k8s.CopyToPod(ctx, keycloakThemeJARPath, keycloakNamespace, helperPodName, "/theme/nova-theme.jar"); err != nil {
		// Clean up the pod on failure
		_ = k8s.DeletePod(ctx, keycloakNamespace, helperPodName)
		progress.FailStep(step, err)
		return fmt.Errorf("failed to copy theme JAR: %w", err)
	}

	// Delete the helper pod (we don't need it anymore)
	ui.Info("Cleaning up helper pod...")
	if err := k8s.DeletePod(ctx, keycloakNamespace, helperPodName); err != nil {
		ui.Warn("Failed to delete helper pod: %v", err)
		// Continue anyway, not a critical error
	}

	progress.CompleteStep(step)
	return nil
}

// deployKeycloakInstance deploys Keycloak IAM instance and realm configuration.
// Returns the generated admin password.
func deployKeycloakInstance(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) (string, error) {
	progress.StartStep(step)

	// Generate Keycloak admin credentials
	keycloakAdminPassword, err := crypto.GenerateRandomPassword(32)
	if err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to generate keycloak admin password: %w", err)
	}

	// Generate Hubble OIDC client secret
	hubbleClientSecret, err := crypto.GenerateRandomPassword(32)
	if err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to generate hubble client secret: %w", err)
	}

	adminData := map[string]interface{}{
		"KeycloakAdminUser":     "admin",
		"KeycloakAdminPassword": keycloakAdminPassword,
	}

	domainData := map[string]interface{}{
		"Domain": cfg.DNS.Domain,
	}

	// Create secrets
	ui.Info("Creating Keycloak secrets...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/secrets/keycloak-admin-secret.yaml", adminData); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to create keycloak admin secret: %w", err)
	}

	// Create certificates
	ui.Info("Creating Keycloak certificates...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/certificates/keycloak-tls.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to create keycloak certificate: %w", err)
	}

	// Deploy Keycloak instance
	ui.Info("Deploying Keycloak instance...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/keycloaks/keycloak.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to deploy keycloak instance: %w", err)
	}

	// Wait for Keycloak CRD to be Ready (like the script does)
	ui.Info("Waiting for Keycloak to be ready...")
	if err := k8s.WaitForCondition(ctx, keycloakNamespace, "keycloaks.k8s.keycloak.org/keycloak", "Ready", 600); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("keycloak failed to become ready: %w", err)
	}

	// Import Keycloak realm
	ui.Info("Importing Keycloak realm...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/keycloakrealmimports/nova.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to import keycloak realm: %w", err)
	}

	// Wait for realm import to be Done
	ui.Info("Waiting for realm import to complete...")
	if err := k8s.WaitForCondition(ctx, keycloakNamespace, "keycloakrealmimports/nova-import", "Done", 300); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("keycloak realm import failed: %w", err)
	}

	// Create Keycloak TLS route
	ui.Info("Creating Keycloak TLS route...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/tlsroutes/keycloak.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to create keycloak tls route: %w", err)
	}

	// Create Hubble OIDC secret (for later Hubble deployment)
	hubbleData := map[string]interface{}{
		"HubbleClientSecret": hubbleClientSecret,
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/hubble/secrets/hubble-oidc.yaml", hubbleData); err != nil {
		progress.FailStep(step, err)
		return "", fmt.Errorf("failed to create hubble oidc secret: %w", err)
	}

	progress.CompleteStep(step)

	return keycloakAdminPassword, nil
}

// deployHubble enables Hubble UI and metrics on the existing Cilium installation.
func deployHubble(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	progress.StartStep(step)

	// Label kube-system namespace (as per script)
	if err := k8s.LabelNamespace(ctx, "kube-system", "service-type", "nova"); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to label kube-system namespace: %w", err)
	}

	// Hubble is deployed by upgrading Cilium with additional values
	client := helm.NewClient("kube-system")

	// Check if Cilium is installed
	installed, err := client.ReleaseExists(ctx, "cilium", "kube-system")
	if err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to check cilium installation: %w", err)
	}

	if !installed {
		err := fmt.Errorf("cilium must be installed before enabling hubble")
		progress.FailStep(step, err)
		return err
	}

	// Upgrade Cilium with Hubble values
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "cilium",
		ChartRef:       "cilium/cilium",
		Namespace:      "kube-system",
		ValuesPath:     "resources/core/deployment/tier2/hubble/values.yaml",
		Wait:           true,
		TimeoutSeconds: 300,
		ReuseValues:    true, // Keep existing Cilium values
	}); err != nil {
		progress.FailStep(step, err)
		return err
	}

	// Deploy Hubble additional resources
	domainData := map[string]interface{}{
		"Domain": cfg.DNS.Domain,
	}

	// Create Keycloak backend for Hubble OIDC
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/hubble/backends/keycloak.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create hubble keycloak backend: %w", err)
	}

	// Create BackendTLSPolicy for Keycloak
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/hubble/backendtlspolicies/keycloak.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create hubble backend tls policy: %w", err)
	}

	// Create SecurityPolicy for OIDC auth
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/hubble/securitypolicies/hubble-oidc-auth.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create hubble security policy: %w", err)
	}

	// Create Hubble HTTP route
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/hubble/httproutes/httproute.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create hubble http route: %w", err)
	}

	progress.CompleteStep(step)
	return nil
}

// deployVictoriaLogsServer deploys VictoriaLogs log aggregation server.
func deployVictoriaLogsServer(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	return deployWithProgress(ctx, progress, step, shared.HelmDeploymentOptions{
		ReleaseName:     "vls",
		ChartRef:        "vm/victoria-logs-single",
		Namespace:       victorialogsNamespace,
		ValuesPath:      "resources/core/deployment/tier2/victorialogs/vlogs-values.yaml",
		Wait:            true,
		TimeoutSeconds:  300,
		CreateNamespace: true,
	}, nil)
}

// deployVictoriaLogsCollector deploys VictoriaLogs collector for log collection.
func deployVictoriaLogsCollector(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	return deployWithProgress(ctx, progress, step, shared.HelmDeploymentOptions{
		ReleaseName:    "collector",
		ChartRef:       "vm/victoria-logs-collector",
		Namespace:      victorialogsNamespace,
		ValuesPath:     "resources/core/deployment/tier2/victorialogs/collector-values.yaml",
		Wait:           true,
		TimeoutSeconds: 300,
	}, nil)
}

// deployVictoriaMetricsStack deploys VictoriaMetrics with Grafana.
// Grafana uses Keycloak OIDC for authentication, so no local password is needed.
func deployVictoriaMetricsStack(ctx context.Context, cfg *config.Config, progress *ui.StepProgress, step int) error {
	progress.StartStep(step)

	// Create namespace
	if err := k8s.CreateNamespace(ctx, victoriametricsNamespace); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// Label namespace (as per script)
	if err := k8s.LabelNamespace(ctx, victoriametricsNamespace, "service-type", "nova"); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to label victoriametrics namespace: %w", err)
	}

	if err := k8s.LabelNamespace(ctx, victoriametricsNamespace, "trust-manager/inject-ca-secret", "enabled"); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to label victoriametrics namespace for CA injection: %w", err)
	}

	// Deploy VictoriaMetrics stack with dynamic values
	// Structure matches the Helm values hierarchy for proper merging
	dynamicValues := map[string]interface{}{
		"grafana": map[string]interface{}{
			"grafana.ini": map[string]interface{}{
				"server": map[string]interface{}{
					"domain": fmt.Sprintf("grafana.%s", cfg.DNS.Domain),
				},
			},
		},
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "vmks",
		ChartRef:       "vm/victoria-metrics-k8s-stack",
		Namespace:      victoriametricsNamespace,
		ValuesPath:     "resources/core/deployment/tier2/victoriametrics/values.yaml",
		Values:         dynamicValues,
		Wait:           true,
		TimeoutSeconds: 600,
	}); err != nil {
		progress.FailStep(step, err)
		return err
	}

	// Create Grafana HTTPRoute
	domainData := map[string]interface{}{
		"Domain": cfg.DNS.Domain,
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/victoriametrics/httproutes/grafana.yaml", domainData); err != nil {
		progress.FailStep(step, err)
		return fmt.Errorf("failed to create grafana http route: %w", err)
	}

	progress.CompleteStep(step)

	return nil
}

// deployWithProgress is a helper to deploy a Helm chart with progress tracking.
func deployWithProgress(ctx context.Context, progress *ui.StepProgress, step int, opts shared.HelmDeploymentOptions, dynamicValuesFn func(context.Context) (map[string]interface{}, error)) error {
	progress.StartStep(step)

	if dynamicValuesFn != nil {
		dynamicValues, err := dynamicValuesFn(ctx)
		if err != nil {
			progress.FailStep(step, err)
			return err
		}
		if opts.Values == nil {
			opts.Values = dynamicValues
		} else {
			for k, v := range dynamicValues {
				opts.Values[k] = v
			}
		}
	}

	if err := shared.DeployHelmChart(ctx, opts); err != nil {
		progress.FailStep(step, err)
		return err
	}

	progress.CompleteStep(step)
	return nil
}

// addTier2HelmRepos adds Helm repositories required for Tier 2 deployment.
func addTier2HelmRepos(ctx context.Context) error {
	repos := map[string]string{
		"kyverno": constants.HelmRepoKyverno,
		"vm":      constants.HelmRepoVictoriaMetrics,
	}

	helmClient := helm.NewClient("")

	ui.Info("Adding %d Helm repositories...", len(repos))
	for name, url := range repos {
		ui.Debug("\t  - Adding %s repository", name)
		if err := helmClient.AddRepository(ctx, name, url); err != nil {
			return fmt.Errorf("failed to add %s repository (%s): %w", name, url, err)
		}
	}

	// Update repos
	ui.Info("Updating repository indexes...")
	if err := helmClient.UpdateRepositories(ctx); err != nil {
		return fmt.Errorf("failed to update Helm repository indexes: %w", err)
	}

	return nil
}

// GetCredentials retrieves Tier 2 credentials from Kubernetes secrets.
// Returns nil if secrets don't exist (tier 2 not deployed).
func GetCredentials(ctx context.Context) *DeployResult {
	// Try to get Keycloak admin credentials from secret
	keycloakSecret, err := k8s.GetSecretData(ctx, keycloakNamespace, "keycloak-admin-secret")
	if err != nil {
		return nil
	}

	adminPassword := ""
	if password, ok := keycloakSecret["password"]; ok {
		adminPassword = password
	}

	if adminPassword == "" {
		return nil
	}

	// Build result with all Keycloak users
	// admin password comes from secret, developer and user have static passwords
	return &DeployResult{
		KeycloakUsers: []KeycloakUser{
			{Username: "admin", Password: adminPassword, Group: "ADMIN"},
			{Username: "developer", Password: "developer", Group: "DEVELOPER"},
			{Username: "user", Password: "user", Group: "USER"},
		},
	}
}

// IsDeployed checks if Tier 2 is deployed by checking for key components.
func IsDeployed(ctx context.Context) bool {
	// Check for Keycloak secret as indicator of tier 2 deployment
	return k8s.SecretExists(ctx, keycloakNamespace, "keycloak-admin-secret")
}
