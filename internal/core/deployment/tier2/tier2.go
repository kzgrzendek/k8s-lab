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

	// Initialize result
	deploymentResult = &DeployResult{
		KeycloakUsers: []KeycloakUser{},
	}

	// Add Helm repos
	if err := addTier2HelmRepos(ctx); err != nil {
		return nil, fmt.Errorf("failed to add Tier 2 Helm repositories: %w", err)
	}

	steps := []string{
		"Kyverno Policy Engine",
		"Keycloak Operator",
		"Keycloak PostgreSQL Database",
		"Keycloak IAM Instance",
		"Keycloak Theme",
		"Hubble Network Observability",
		"VictoriaLogs Server",
		"VictoriaLogs Collector",
		"VictoriaMetrics Stack",
	}
	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Helper wrapper for step execution
	runStep := func(name string, fn func() error) error {
		progress.StartStep(currentStep)
		if err := fn(); err != nil {
			progress.FailStep(currentStep, err)
			return err
		}
		progress.CompleteStep(currentStep)
		currentStep++
		return nil
	}

	// 1. Kyverno
	if err := runStep("Kyverno", func() error {
		return deployKyverno(ctx)
	}); err != nil {
		return nil, err
	}

	// 2. Keycloak Operator
	if err := runStep("Keycloak Operator", func() error {
		return deployKeycloakOperator(ctx)
	}); err != nil {
		return nil, err
	}

	// 3. Keycloak PostgreSQL
	if err := runStep("Keycloak PostgreSQL", func() error {
		return deployKeycloakPostgreSQL(ctx)
	}); err != nil {
		return nil, err
	}

	// 4. Keycloak Theme (must come before Keycloak instance)
	if err := runStep("Keycloak Theme", func() error {
		// Create PVC for theme storage
		if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier2/keycloak/pvc/theme-pvc.yaml"); err != nil {
			return fmt.Errorf("failed to create theme PVC: %w", err)
		}

		// Deploy theme to PVC using helper pod
		return deployKeycloakTheme(ctx)
	}); err != nil {
		return nil, err
	}

	// 5. Keycloak Instance (after theme is ready)
	if err := runStep("Keycloak Instance", func() error {
		pwd, err := deployKeycloakInstance(ctx, cfg)
		if err != nil {
			return err
		}
		deploymentResult.KeycloakUsers = []KeycloakUser{
			{Username: "cluster-admin", Password: pwd, Group: "MASTER_ADMIN"},
			{Username: "admin", Password: "admin", Group: "ADMIN"},
			{Username: "developer", Password: "developer", Group: "DEVELOPER"},
			{Username: "user", Password: "user", Group: "USER"},
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// 6. Hubble
	if err := runStep("Hubble", func() error {
		return deployHubble(ctx, cfg)
	}); err != nil {
		return nil, err
	}

	// 7. VictoriaLogs Server
	if err := runStep("VictoriaLogs Server", func() error {
		return deployVictoriaLogsServer(ctx)
	}); err != nil {
		return nil, err
	}

	// 8. VictoriaLogs Collector
	if err := runStep("VictoriaLogs Collector", func() error {
		return deployVictoriaLogsCollector(ctx)
	}); err != nil {
		return nil, err
	}

	// 9. VictoriaMetrics Stack
	if err := runStep("VictoriaMetrics Stack", func() error {
		return deployVictoriaMetricsStack(ctx, cfg)
	}); err != nil {
		return nil, err
	}

	progress.Complete()
	ui.Header("Tier 2 Deployment Complete")
	ui.Success("Platform services are running")
	ui.Info("Keycloak: https://%s", cfg.DNS.AuthDomain)
	ui.Info("Hubble: https://hubble.%s", cfg.DNS.Domain)
	ui.Info("Grafana: https://grafana.%s", cfg.DNS.Domain)

	return deploymentResult, nil
}

// deployKyverno deploys Kyverno policy engine.
func deployKyverno(ctx context.Context) error {
	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "kyverno",
		ChartRef:        "kyverno/kyverno",
		Namespace:       kyvernoNamespace,
		ValuesPath:      "resources/core/deployment/tier2/kyverno/values.yaml",
		Wait:            true,
		TimeoutSeconds:  300,
		CreateNamespace: true,
	})
}

// deployKeycloakOperator deploys Keycloak operator CRDs and controller.
func deployKeycloakOperator(ctx context.Context) error {
	// Ensure namespace exists and is labeled
	if err := shared.EnsureNamespace(ctx, keycloakNamespace, map[string]string{
		"service-type":                   "keycloak",
		"trust-manager/inject-ca-secret": "enabled",
	}); err != nil {
		return err
	}

	// Apply CRDs and Operator
	manifests := []string{
		constants.ManifestKeycloakCRD,
		constants.ManifestKeycloakRealmImportCRD,
		constants.ManifestKeycloakOperator,
	}

	for _, url := range manifests {
		ui.Info("Applying %s...", url)
		if err := k8s.ApplyURLWithNamespace(ctx, url, keycloakNamespace); err != nil {
			return fmt.Errorf("failed to apply %s: %w", url, err)
		}
	}

	ui.Info("Waiting for Keycloak Operator to be ready...")
	if err := k8s.WaitForDeploymentReady(ctx, keycloakNamespace, "keycloak-operator", 300); err != nil {
		return fmt.Errorf("keycloak operator not ready: %w", err)
	}

	return nil
}

// deployKeycloakPostgreSQL deploys PostgreSQL database for Keycloak.
func deployKeycloakPostgreSQL(ctx context.Context) error {
	var pwd string
	if k8s.SecretExists(ctx, keycloakNamespace, "keycloak-db-secret") {
		data, err := k8s.GetSecretData(ctx, keycloakNamespace, "keycloak-db-secret")
		if err == nil && data["password"] != "" {
			pwd = data["password"]
		}
	}

	if pwd == "" {
		var err error
		pwd, err = crypto.GenerateRandomPassword(32)
		if err != nil {
			return err
		}
	}

	// Create/Update database secret
	if err := k8s.CreateSecret(ctx, keycloakNamespace, "keycloak-db-secret", map[string]string{
		"username": "admin",
		"password": pwd,
	}); err != nil {
		return fmt.Errorf("failed to create keycloak db secret: %w", err)
	}

	resources := []string{
		"resources/core/deployment/tier2/keycloak/postgresql/statefulset.yaml",
		"resources/core/deployment/tier2/keycloak/postgresql/service.yaml",
	}

	for _, res := range resources {
		if err := shared.ApplyTemplate(ctx, res, nil); err != nil {
			return fmt.Errorf("failed to apply %s: %w", res, err)
		}
	}

	if err := k8s.WaitForDeploymentReady(ctx, keycloakNamespace, "postgresql-db", 300); err != nil {
		return fmt.Errorf("postgresql not ready: %w", err)
	}

	return nil
}

// deployKeycloakTheme installs the NOVA Keycloak theme by copying it to the PVC.
// This should be called before deploying the Keycloak instance.
func deployKeycloakTheme(ctx context.Context) error {
	const jarPath = "resources/core/deployment/tier2/keycloak/theme/dist_keycloak/keycloak-theme-for-kc-all-other-versions.jar"
	const helperPodName = "keycloak-theme-copy-helper"

	// Create a temporary helper pod with the PVC mounted
	ui.Info("Creating helper pod to copy theme...")
	helperPodYAML := `apiVersion: v1
kind: Pod
metadata:
  name: ` + helperPodName + `
  namespace: ` + keycloakNamespace + `
spec:
  containers:
  - name: helper
    image: busybox:latest
    command: ["sleep", "3600"]
    volumeMounts:
    - name: nova-theme
      mountPath: /theme
  volumes:
  - name: nova-theme
    persistentVolumeClaim:
      claimName: keycloak-theme-pvc
  restartPolicy: Never
`

	if err := k8s.ApplyYAMLContent(ctx, helperPodYAML); err != nil {
		return fmt.Errorf("failed to create helper pod: %w", err)
	}

	// Clean up helper pod when done
	defer func() {
		ui.Info("Cleaning up helper pod...")
		k8s.DeletePod(ctx, keycloakNamespace, helperPodName)
	}()

	// Wait for helper pod to be ready
	ui.Info("Waiting for helper pod to be ready...")
	if err := k8s.WaitForPodReady(ctx, keycloakNamespace, helperPodName, 60); err != nil {
		return fmt.Errorf("helper pod not ready: %w", err)
	}

	// Copy theme JAR to helper pod (which writes to the PVC)
	ui.Info("Copying theme JAR to PVC...")
	if err := k8s.CopyToPod(ctx, jarPath, keycloakNamespace, helperPodName, "/theme/nova-theme.jar"); err != nil {
		return fmt.Errorf("failed to copy theme to PVC: %w", err)
	}

	ui.Info("Theme successfully copied to PVC")
	return nil
}

// deployKeycloakInstance deploys Keycloak IAM instance.
// Returns the generated admin password.
func deployKeycloakInstance(ctx context.Context, cfg *config.Config) (string, error) {
	var adminPwd string

	adminUserExists := false
	if k8s.SecretExists(ctx, keycloakNamespace, "keycloak-admin-secret") {
		data, err := k8s.GetSecretData(ctx, keycloakNamespace, "keycloak-admin-secret")
		if err == nil && data["password"] != "" {
			adminPwd = data["password"]
			adminUserExists = true
		}
	}

	if adminPwd == "" {
		var err error
		adminPwd, err = crypto.GenerateRandomPassword(10)
		if err != nil {
			return "", err
		}
	}

	data := map[string]interface{}{
		"KeycloakAdminUser":     "cluster-admin",
		"KeycloakAdminPassword": adminPwd,
		"Domain":                cfg.DNS.Domain,
		"AuthDomain":            cfg.DNS.AuthDomain,
	}

	// Create bootstrap admin secret (temporary admin for initial setup)
	bootstrapPwd, err := crypto.GenerateRandomPassword(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate bootstrap password: %w", err)
	}
	if err := k8s.CreateSecret(ctx, keycloakNamespace, "keycloak-bootstrap-admin", map[string]string{
		"username": "admin",
		"password": bootstrapPwd,
	}); err != nil {
		return "", fmt.Errorf("failed to create keycloak-bootstrap-admin secret: %w", err)
	}

	// Create cluster-admin secret for master realm
	if err := k8s.CreateSecret(ctx, keycloakNamespace, "keycloak-admin-secret", map[string]string{
		"username": "cluster-admin",
		"password": adminPwd,
	}); err != nil {
		return "", fmt.Errorf("failed to create keycloak-admin-secret: %w", err)
	}

	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/certificates/keycloak-tls.yaml", data); err != nil {
		return "", err
	}

	ui.Info("Deploying Keycloak instance...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/keycloaks/keycloak.yaml", data); err != nil {
		return "", err
	}

	if err := k8s.WaitForCondition(ctx, keycloakNamespace, "keycloaks.k8s.keycloak.org/keycloak", "Ready", 600); err != nil {
		return "", err
	}

	if !adminUserExists {
		ui.Info("Creating cluster-admin user via Job...")
		if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier2/keycloak/jobs/create-admin-user.yaml"); err != nil {
			return "", fmt.Errorf("failed to create admin user job: %w", err)
		}

		// Wait for the job to complete
		if err := k8s.WaitForCondition(ctx, keycloakNamespace, "job/keycloak-create-admin", "Complete", 120); err != nil {
			return "", fmt.Errorf("failed to create admin user: %w", err)
		}
	} else {
		ui.Info("Cluster admin user already exists (secret found), skipping user creation job.")
	}

	ui.Info("Importing Nova Realm...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/keycloakrealmimports/nova.yaml", data); err != nil {
		return "", err
	}
	if err := k8s.WaitForCondition(ctx, keycloakNamespace, "keycloakrealmimports/nova-import", "Done", 300); err != nil {
		return "", err
	}

	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/keycloak/tlsroutes/keycloak.yaml", data); err != nil {
		return "", err
	}

	return adminPwd, nil
}

// deployHubble enables Hubble UI.
func deployHubble(ctx context.Context, cfg *config.Config) error {
	if err := k8s.LabelNamespace(ctx, "kube-system", "service-type", "nova"); err != nil {
		return err
	}

	// Verify Cilium presence
	client := helm.NewClient("kube-system")
	if exists, err := client.ReleaseExists(ctx, "cilium", "kube-system"); err != nil || !exists {
		return fmt.Errorf("cilium not found: %w", err)
	}

	// Create OIDC secret for Hubble in kube-system
	if err := k8s.CreateSecret(ctx, "kube-system", "oidc", map[string]string{
		"client-id":     "hubble",
		"client-secret": "74ZnkKBH4DrfB6ywE4Pdwtk0JFJ8DHLA",
	}); err != nil {
		return fmt.Errorf("failed to create oidc secret: %w", err)
	}

	// Upgrade Cilium
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "cilium",
		ChartRef:       "cilium/cilium",
		Namespace:      "kube-system",
		ValuesPath:     "resources/core/deployment/tier2/hubble/values.yaml",
		Wait:           true,
		TimeoutSeconds: 300,
		ReuseValues:    true,
	}); err != nil {
		return err
	}

	// Apply Hubble resources
	data := map[string]interface{}{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}
	resources := []string{
		"resources/core/deployment/tier2/hubble/backends/keycloak.yaml",
		"resources/core/deployment/tier2/hubble/backendtlspolicies/keycloak.yaml",
		"resources/core/deployment/tier2/hubble/securitypolicies/hubble-oidc-auth.yaml",
		"resources/core/deployment/tier2/hubble/httproutes/httproute.yaml",
	}

	for _, res := range resources {
		if err := shared.ApplyTemplate(ctx, res, data); err != nil {
			return err
		}
	}

	return nil
}

// deployVictoriaLogsServer deploys VLS.
func deployVictoriaLogsServer(ctx context.Context) error {
	if err := shared.EnsureNamespace(ctx, victorialogsNamespace, map[string]string{
		"service-type": "nova",
	}); err != nil {
		return err
	}

	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "vls",
		ChartRef:       "vm/victoria-logs-single",
		Namespace:      victorialogsNamespace,
		ValuesPath:     "resources/core/deployment/tier2/victorialogs/vlogs-values.yaml",
		Wait:           true,
		TimeoutSeconds: 300,
	})
}

// deployVictoriaLogsCollector deploys VLC.
func deployVictoriaLogsCollector(ctx context.Context) error {
	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "collector",
		ChartRef:       "vm/victoria-logs-collector",
		Namespace:      victorialogsNamespace,
		ValuesPath:     "resources/core/deployment/tier2/victorialogs/collector-values.yaml",
		Wait:           true,
		TimeoutSeconds: 300,
	})
}

// deployVictoriaMetricsStack deploys VM Stack with OIDC.
func deployVictoriaMetricsStack(ctx context.Context, cfg *config.Config) error {
	if err := shared.EnsureNamespace(ctx, victoriametricsNamespace, map[string]string{
		"service-type":                   "nova",
		"trust-manager/inject-ca-secret": "enabled",
	}); err != nil {
		return err
	}

	// Create OIDC secret for Grafana
	if err := k8s.CreateSecret(ctx, victoriametricsNamespace, "oidc", map[string]string{
		"client-id":     "grafana",
		"client-secret": "OLANqOyLmQ7deODliaxm42CHttBu6jnl",
	}); err != nil {
		return fmt.Errorf("failed to create oidc secret: %w", err)
	}

	// Create/Update Grafana admin secret
	var grafanaAdminPwd string
	if k8s.SecretExists(ctx, victoriametricsNamespace, "grafana-admin") {
		data, err := k8s.GetSecretData(ctx, victoriametricsNamespace, "grafana-admin")
		if err == nil && data["admin-password"] != "" {
			grafanaAdminPwd = data["admin-password"]
		}
	}

	if grafanaAdminPwd == "" {
		var err error
		grafanaAdminPwd, err = crypto.GenerateRandomPassword(32)
		if err != nil {
			return err
		}
	}

	if err := k8s.CreateSecret(ctx, victoriametricsNamespace, "grafana-admin", map[string]string{
		"admin-user":     "admin",
		"admin-password": grafanaAdminPwd,
	}); err != nil {
		return fmt.Errorf("failed to create grafana-admin secret: %w", err)
	}

	// Deploy GPU dashboard ConfigMap before Grafana starts
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier2/victoriametrics/dashboards/gpu-overview.yaml"); err != nil {
		return fmt.Errorf("failed to deploy GPU dashboard: %w", err)
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName: "vmks",
		ChartRef:    "vm/victoria-metrics-k8s-stack",
		Namespace:   victoriametricsNamespace,
		ValuesPath:  "resources/core/deployment/tier2/victoriametrics/values.yaml",
		TemplateData: map[string]interface{}{
			"Domain":     cfg.DNS.Domain,
			"AuthDomain": cfg.DNS.AuthDomain,
		},
		Wait:           true,
		TimeoutSeconds: 600,
	}); err != nil {
		return err
	}

	data := map[string]interface{}{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/victoriametrics/httproutes/grafana.yaml", data); err != nil {
		return err
	}

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
	// cluster-admin is for master realm, others are for nova realm
	return &DeployResult{
		KeycloakUsers: []KeycloakUser{
			{Username: "cluster-admin", Password: adminPassword, Group: "MASTER_ADMIN"},
			{Username: "admin", Password: "admin", Group: "ADMIN"},
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
