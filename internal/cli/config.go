package cli

import (
	"fmt"
	"strings"

	"github.com/kplane-dev/kplane/internal/kubeconfig"
	"github.com/kplane-dev/kplane/internal/kubectl"
	kindprovider "github.com/kplane-dev/kplane/internal/provider/kind"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage kubeconfig",
	}
	cmd.AddCommand(newUseContextCommand())
	return cmd
}

func newUseContextCommand() *cobra.Command {
	var kubeconfigPath string

	cmd := &cobra.Command{
		Use:   "use-context <name>",
		Short: "Set the current kubeconfig context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustConfig()
			profile, err := cfg.ActiveProfile()
			if err != nil {
				return err
			}
			if kubeconfigPath == "" {
				kubeconfigPath = profile.KubeconfigPath
			}

			name := normalizeContextName(args[0])
			if strings.HasPrefix(name, "kind-") && profile.Provider == "kind" {
				if err := kindprovider.EnsureInstalled(); err != nil {
					return err
				}
				clusterName := strings.TrimPrefix(name, "kind-")
				exists, err := kindprovider.ClusterExists(cmd.Context(), clusterName)
				if err != nil {
					return err
				}
				if !exists {
					return fmt.Errorf("kind cluster %q not found", clusterName)
				}
			}
			if strings.HasPrefix(name, "kplane-") {
				if err := kubectl.EnsureInstalled(); err != nil {
					return err
				}
				managementCtx := fmt.Sprintf("kind-%s", profile.ClusterName)
				controlPlaneName := strings.TrimPrefix(name, "kplane-")
				if controlPlaneName == "" {
					return fmt.Errorf("controlplane name is required")
				}
				if _, err := kubectl.GetJSONPath(cmd.Context(), managementCtx, "controlplane", controlPlaneName, "", "{.metadata.name}"); err != nil {
					return fmt.Errorf("controlplane %q not found in management cluster", controlPlaneName)
				}
			}
			if err := kubeconfig.SetCurrentContext(kubeconfigPath, name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "current context set to %s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Kubeconfig path to update")
	return cmd
}

func normalizeContextName(name string) string {
	if strings.HasPrefix(name, "kplane-") {
		return name
	}
	if strings.HasPrefix(name, "kind-") {
		return name
	}
	return "kplane-" + name
}
