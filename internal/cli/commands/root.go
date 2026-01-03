// Package cmd implements the nova CLI commands using Cobra.
package commands

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd is the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "nova",
	Short: "NOVA - Native Operator for Versatile AI",
	Long: `NOVA (Native Operator for Versatile AI) is a CLI tool for managing
a GPU-powered Kubernetes lab for AI/ML development on your local machine.

With a single command, deploy a production-like, multi-tier Kubernetes
environment optimized for AI/ML workloads including:

  • 3-node Minikube cluster with GPU support
  • Infrastructure: Cilium CNI, Cert-Manager, Envoy Gateway, NVIDIA GPU Operator
  • Platform: Keycloak (IAM), Kyverno (policies), Victoria Metrics/Logs
  • Applications: llm-d (LLM serving), Open WebUI, HELIX`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		ui.Error("%v", err)
		return err
	}
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		fmt.Sprintf("config file (default is %s)", config.DefaultConfigPath()))
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"enable verbose output")

	// Bind flags to viper
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Add subcommands
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newDeleteCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newExportLogsCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newKubectlCmd())
	rootCmd.AddCommand(newHelmCmd())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(config.ConfigDir())
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("NOVA")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil && verbose {
		ui.Info("Using config file: %s", viper.ConfigFileUsed())
	}
}
