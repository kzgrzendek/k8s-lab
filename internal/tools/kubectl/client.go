// Package k8s provides a centralized wrapper around the Kubernetes client-go SDK.
//
// This package uses the official Kubernetes client-go SDK (k8s.io/client-go)
// to manage Kubernetes resources. All kubectl operations in NOVA should go
// through this package to maintain consistency and ease future refactoring.
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/tools/exec"
)

// getClient creates a Kubernetes client using the default kubeconfig.
// This is a helper function used by all operations.
func getClient() (*kubernetes.Clientset, error) {
	// Get kubeconfig path
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		kubeconfig = kubeconfigEnv
	}

	// Build config from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Suppress Kubernetes client-go warnings (e.g., deprecation warnings)
	// These warnings go directly to stderr and can't be captured by ephemeral output
	config.WarningHandler = rest.NoWarnings{}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, nil
}

// TaintNode adds a taint to a Kubernetes node.
func TaintNode(ctx context.Context, nodeName, key, value, effect string) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	// Get the node
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Create the taint
	taint := corev1.Taint{
		Key:    key,
		Value:  value,
		Effect: corev1.TaintEffect(effect),
	}

	// Check if taint already exists and update it, or add new taint
	taintExists := false
	for i, existingTaint := range node.Spec.Taints {
		if existingTaint.Key == key {
			node.Spec.Taints[i] = taint
			taintExists = true
			break
		}
	}

	if !taintExists {
		node.Spec.Taints = append(node.Spec.Taints, taint)
	}

	// Update the node
	_, err = clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to taint node %s: %w", nodeName, err)
	}

	return nil
}

// LabelNode adds or removes a label on a Kubernetes node.
// For adding: label should be "key=value"
// For removing: set remove=true and label should be just "key"
func LabelNode(ctx context.Context, nodeName, label string, remove bool) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	// Get the node
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	if remove {
		// Remove the label
		delete(node.Labels, label)
	} else {
		// Parse label (key=value format)
		// For compatibility with old code, also support just key
		key := label
		value := ""

		// Try to split by '='
		for i, c := range label {
			if c == '=' {
				key = label[:i]
				value = label[i+1:]
				break
			}
		}

		node.Labels[key] = value
	}

	// Update the node
	_, err = clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to label node %s: %w", nodeName, err)
	}

	return nil
}

// LabelAllNodes applies a label to all nodes in the cluster.
func LabelAllNodes(ctx context.Context, label string) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	// Get all nodes
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Parse label key and value
	key := label
	value := ""
	for i, c := range label {
		if c == '=' {
			key = label[:i]
			value = label[i+1:]
			break
		}
	}

	// Apply label to each node
	for _, node := range nodes.Items {
		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		node.Labels[key] = value

		_, err := clientset.CoreV1().Nodes().Update(ctx, &node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to label node %s: %w", node.Name, err)
		}
	}

	return nil
}

// GetNodes returns a list of all node names.
func GetNodes(ctx context.Context) ([]string, error) {
	clientset, err := getClient()
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	nodeNames := make([]string, len(nodes.Items))
	for i, node := range nodes.Items {
		nodeNames[i] = node.Name
	}

	return nodeNames, nil
}

// GetNodesByRole returns nodes with a specific role label.
func GetNodesByRole(ctx context.Context, role string) ([]string, error) {
	clientset, err := getClient()
	if err != nil {
		return nil, err
	}

	labelSelector := fmt.Sprintf("node-role.kubernetes.io/%s", role)
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes by role %s: %w", role, err)
	}

	nodeNames := make([]string, len(nodes.Items))
	for i, node := range nodes.Items {
		nodeNames[i] = node.Name
	}

	return nodeNames, nil
}

// IsNodeControlPlane checks if a node is a control-plane node.
func IsNodeControlPlane(ctx context.Context, nodeName string) (bool, error) {
	clientset, err := getClient()
	if err != nil {
		return false, err
	}

	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Check if the control-plane label exists
	_, hasLabel := node.Labels["node-role.kubernetes.io/control-plane"]
	return hasLabel, nil
}

// CreateNamespace creates a namespace if it doesn't exist (idempotent).
func CreateNamespace(ctx context.Context, name string) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		// Check if namespace already exists
		if _, getErr := clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{}); getErr == nil {
			return nil // Namespace already exists
		}
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	return nil
}

// ApplyURL applies a Kubernetes manifest from a URL using kubectl.
// This is useful for installing CRDs and other resources from remote manifests.
// The client-go SDK doesn't have a built-in way to apply arbitrary manifests,
// so we use kubectl apply as a CLI command.
func ApplyURL(ctx context.Context, url string) error {
	// Use ephemeral output for kubectl apply
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "kubectl", "apply", "-f", url).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		// Keep error output visible
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to apply manifest from %s: %w", url, err)
	}
	return nil
}

// ApplyURLWithNamespace applies a Kubernetes manifest from a URL to a specific namespace.
// This is useful when the manifest doesn't specify a namespace but needs to be deployed
// to a particular namespace (e.g., operators that don't hardcode their namespace).
func ApplyURLWithNamespace(ctx context.Context, url, namespace string) error {
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "kubectl", "apply", "-f", url, "-n", namespace).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to apply manifest from %s to namespace %s: %w", url, namespace, err)
	}
	return nil
}

// ApplyYAML applies a Kubernetes manifest from a YAML file using kubectl.
// This is useful for installing resources from local manifest files.
// The client-go SDK doesn't have a built-in way to apply arbitrary manifests,
// so we use kubectl apply as a CLI command.
func ApplyYAML(ctx context.Context, path string) error {
	// Use ephemeral output for kubectl apply
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "kubectl", "apply", "-f", path).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		// Keep error output visible
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to apply manifest from %s: %w", path, err)
	}
	return nil
}

// ApplyYAMLContent applies Kubernetes manifest from YAML content string using kubectl.
// This is useful when you have generated YAML content (e.g., from templates) that needs to be applied.
func ApplyYAMLContent(ctx context.Context, yamlContent string) error {
	// Use ephemeral output for kubectl apply
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "kubectl", "apply", "-f", "-").
		WithStdin(strings.NewReader(yamlContent)).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		// Keep error output visible
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to apply manifest from content: %w", err)
	}
	return nil
}

// PatchConfigMap patches a ConfigMap using kubectl patch with a YAML file.
func PatchConfigMap(ctx context.Context, namespace, name, patchFile string) error {
	// Use ephemeral output for kubectl patch
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "kubectl", "patch", "configmap", name,
		"--namespace", namespace,
		"--patch-file", patchFile).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		// Keep error output visible
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to patch configmap %s in namespace %s: %w", name, namespace, err)
	}
	return nil
}

// CreateSecret creates a generic Kubernetes secret (idempotent).
// If the secret already exists, it will be deleted and recreated.
func CreateSecret(ctx context.Context, namespace, name string, data map[string]string) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	// Convert string data to byte data
	secretData := make(map[string][]byte)
	for key, value := range data {
		secretData[key] = []byte(value)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	// Try to create the secret
	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		// If secret already exists, delete and recreate it
		if _, getErr := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{}); getErr == nil {
			// Delete existing secret
			if delErr := clientset.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); delErr != nil {
				return fmt.Errorf("failed to delete existing secret %s: %w", name, delErr)
			}
			// Recreate secret
			_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to recreate secret %s: %w", name, err)
			}
			return nil
		}
		return fmt.Errorf("failed to create secret %s: %w", name, err)
	}

	return nil
}

// LabelNamespace adds or updates a label on a Kubernetes namespace.
func LabelNamespace(ctx context.Context, name, key, value string) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	// Get the namespace
	namespace, err := clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", name, err)
	}

	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	namespace.Labels[key] = value

	// Update the namespace
	_, err = clientset.CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to label namespace %s: %w", name, err)
	}

	return nil
}

// WaitForDeploymentReady waits for a StatefulSet or Deployment to be ready.
// This function checks for StatefulSet first, then Deployment.
// timeoutSeconds specifies how long to wait before giving up.
func WaitForDeploymentReady(ctx context.Context, namespace, name string, timeoutSeconds int) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	checkInterval := 5 * time.Second

	for time.Now().Before(deadline) {
		// Check if it's a StatefulSet
		statefulset, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			// Check if StatefulSet is ready
			if statefulset.Status.ReadyReplicas == *statefulset.Spec.Replicas {
				return nil
			}
			ui.Debug("Waiting for StatefulSet %s/%s to be ready (%d/%d replicas ready)", namespace, name, statefulset.Status.ReadyReplicas, *statefulset.Spec.Replicas)
			time.Sleep(checkInterval)
			continue
		}

		// Check if it's a Deployment
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			// Check if Deployment is ready
			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				return nil
			}
			ui.Debug("Waiting for Deployment %s/%s to be ready (%d/%d replicas ready)", namespace, name, deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
			time.Sleep(checkInterval)
			continue
		}

		// Neither StatefulSet nor Deployment found
		return fmt.Errorf("neither StatefulSet nor Deployment named %s found in namespace %s", name, namespace)
	}

	return fmt.Errorf("timeout waiting for %s/%s to be ready after %d seconds", namespace, name, timeoutSeconds)
}

// WaitForEndpoints waits for a Service's endpoints to have at least one address.
// timeoutSeconds specifies how long to wait before giving up.
func WaitForEndpoints(ctx context.Context, namespace, name string, timeoutSeconds int) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	checkInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			// Check if endpoints have at least one address
			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) > 0 {
					return nil
				}
			}
		}

		time.Sleep(checkInterval)
	}

	return fmt.Errorf("timeout waiting for endpoints %s/%s after %d seconds", namespace, name, timeoutSeconds)
}

// WaitForSecret waits for a Secret to exist in a namespace.
// timeoutSeconds specifies how long to wait before giving up.
func WaitForSecret(ctx context.Context, namespace, name string, timeoutSeconds int) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	checkInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		_, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			return nil
		}

		time.Sleep(checkInterval)
	}

	return fmt.Errorf("timeout waiting for secret %s/%s after %d seconds", namespace, name, timeoutSeconds)
}

// GetSecretData retrieves data from a Kubernetes secret.
// Returns a map of key-value pairs from the secret's data field.
// Values are decoded from base64 automatically by the Kubernetes API.
func GetSecretData(ctx context.Context, namespace, name string) (map[string]string, error) {
	clientset, err := getClient()
	if err != nil {
		return nil, err
	}

	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	// Convert byte data to string data
	result := make(map[string]string)
	for key, value := range secret.Data {
		result[key] = string(value)
	}

	return result, nil
}

// SecretExists checks if a secret exists in a namespace.
func SecretExists(ctx context.Context, namespace, name string) bool {
	clientset, err := getClient()
	if err != nil {
		return false
	}

	_, err = clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

// WaitForCondition waits for a resource to have a specific condition.
// This uses kubectl wait for CRD conditions that aren't accessible via client-go.
// resource should be in the format "resource/name" (e.g., "keycloaks.k8s.keycloak.org/keycloak")
// condition should be the condition name (e.g., "Ready", "Done")
// timeoutSeconds specifies how long to wait before giving up.
func WaitForCondition(ctx context.Context, namespace, resource, condition string, timeoutSeconds int) error {
	args := []string{
		"wait",
		"--for=condition=" + condition,
		resource,
		"--timeout=" + fmt.Sprintf("%ds", timeoutSeconds),
	}

	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	if err := exec.Run(ctx, "kubectl", args...); err != nil {
		return fmt.Errorf("failed waiting for %s condition=%s: %w", resource, condition, err)
	}

	return nil
}

// RestartDeployment restarts a deployment using kubectl rollout restart.
func RestartDeployment(ctx context.Context, namespace, name string) error {
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "kubectl", "-n", namespace, "rollout", "restart", "deployment/"+name).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to restart deployment %s/%s: %w", namespace, name, err)
	}
	return nil
}

// WaitForPodReady waits for a specific pod to be in Running state with all containers ready.
// timeoutSeconds specifies how long to wait before giving up.
func WaitForPodReady(ctx context.Context, namespace, name string, timeoutSeconds int) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(timeout)
	checkInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			if pod.Status.Phase == corev1.PodRunning {
				// Check if all containers are ready
				allReady := true
				for _, containerStatus := range pod.Status.ContainerStatuses {
					if !containerStatus.Ready {
						allReady = false
						break
					}
				}
				if allReady && len(pod.Status.ContainerStatuses) > 0 {
					return nil
				}
			}
			ui.Debug("Waiting for pod %s/%s to be ready (phase: %s)", namespace, name, pod.Status.Phase)
		}

		time.Sleep(checkInterval)
	}

	return fmt.Errorf("timeout waiting for pod %s/%s to be ready after %d seconds", namespace, name, timeoutSeconds)
}

// CopyToPod copies a local file to a path inside a pod using kubectl cp.
// destPath should be in the format "namespace/podname:path/in/container"
func CopyToPod(ctx context.Context, localPath, namespace, podName, containerPath string) error {
	dest := fmt.Sprintf("%s/%s:%s", namespace, podName, containerPath)

	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "kubectl", "cp", localPath, dest).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to copy %s to %s: %w", localPath, dest, err)
	}
	return nil
}

// DeletePod deletes a pod in a namespace.
func DeletePod(ctx context.Context, namespace, name string) error {
	clientset, err := getClient()
	if err != nil {
		return err
	}

	err = clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s/%s: %w", namespace, name, err)
	}

	return nil
}

// PodExists checks if a pod exists in a namespace.
func PodExists(ctx context.Context, namespace, name string) bool {
	clientset, err := getClient()
	if err != nil {
		return false
	}

	_, err = clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

// PVCExists checks if a PersistentVolumeClaim exists in a namespace.
func PVCExists(ctx context.Context, namespace, name string) bool {
	clientset, err := getClient()
	if err != nil {
		return false
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

// CreateKubectlContext creates a new kubectl context using a service account token.
// It sets up credentials, cluster info, and context in the user's kubeconfig.
func CreateKubectlContext(ctx context.Context, contextName, namespace, serviceAccountName string) error {
	// Get the service account token from the associated secret
	secretName := serviceAccountName + "-token"
	secretData, err := GetSecretData(ctx, namespace, secretName)
	if err != nil {
		return fmt.Errorf("failed to get service account token: %w", err)
	}

	token, ok := secretData["token"]
	if !ok {
		return fmt.Errorf("token not found in secret %s/%s", namespace, secretName)
	}

	// Get the current cluster server URL
	config, err := clientcmd.BuildConfigFromFlags("", getKubeconfigPath())
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clusterName := contextName + "-cluster"
	userName := contextName + "-user"

	// Set cluster (reuse existing CA from minikube)
	if err := exec.Run(ctx, "kubectl", "config", "set-cluster", clusterName,
		"--server="+config.Host,
		"--insecure-skip-tls-verify=true"); err != nil {
		return fmt.Errorf("failed to set cluster: %w", err)
	}

	// Set credentials
	if err := exec.Run(ctx, "kubectl", "config", "set-credentials", userName,
		"--token="+token); err != nil {
		return fmt.Errorf("failed to set credentials: %w", err)
	}

	// Set context
	if err := exec.Run(ctx, "kubectl", "config", "set-context", contextName,
		"--cluster="+clusterName,
		"--user="+userName,
		"--namespace="+namespace); err != nil {
		return fmt.Errorf("failed to set context: %w", err)
	}

	return nil
}

// getKubeconfigPath returns the path to the kubeconfig file.
func getKubeconfigPath() string {
	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		return kubeconfigEnv
	}
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

// ContextExists checks if a kubectl context exists in the kubeconfig.
func ContextExists(ctx context.Context, contextName string) bool {
	err := exec.Run(ctx, "kubectl", "config", "get-contexts", contextName)
	return err == nil
}

// SwitchContext switches the current kubectl context.
func SwitchContext(ctx context.Context, contextName string) error {
	return exec.Run(ctx, "kubectl", "config", "use-context", contextName)
}

// RenameContext renames a kubectl context.
func RenameContext(ctx context.Context, oldName, newName string) error {
	return exec.Run(ctx, "kubectl", "config", "rename-context", oldName, newName)
}
