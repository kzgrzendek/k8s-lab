package cmd

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newKubectlCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "kubectl",
		Short:              "Run kubectl against the NOVA cluster",
		Long:               `Runs kubectl with the NOVA cluster context (via minikube kubectl).`,
		DisableFlagParsing: true,
		RunE:               runKubectl,
	}
}

func runKubectl(cmd *cobra.Command, args []string) error {
	// Build minikube kubectl command
	minikubeArgs := append([]string{"kubectl", "--"}, args...)

	// Find minikube binary
	minikubePath, err := exec.LookPath("minikube")
	if err != nil {
		return ui.Errorf("minikube not found in PATH: %w", err)
	}

	// Replace current process with minikube kubectl
	// This preserves stdin/stdout/stderr and signals
	return syscall.Exec(minikubePath, append([]string{"minikube"}, minikubeArgs...), os.Environ())
}
