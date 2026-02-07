package cli

import (
	"fmt"
	"strings"

	"github.com/kplane-dev/kplane/internal/kubeconfig"
	"github.com/kplane-dev/kplane/internal/kubectl"
	"github.com/kplane-dev/kplane/internal/providers"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage kubeconfig",
	}
	cmd.AddCommand(newUseContextCommand())
	cmd.AddCommand(newSetProviderCommand())
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

			clusterProvider, err := providers.New(profile.Provider)
			if err != nil {
				return err
			}
			name := normalizeContextName(args[0], clusterProvider.ContextPrefix())
			if strings.HasPrefix(name, clusterProvider.ContextPrefix()) {
				if err := clusterProvider.EnsureInstalled(); err != nil {
					return err
				}
				clusterName := strings.TrimPrefix(name, clusterProvider.ContextPrefix())
				exists, err := clusterProvider.ClusterExists(cmd.Context(), clusterName)
				if err != nil {
					return err
				}
				if !exists {
					return fmt.Errorf("%s cluster %q not found", clusterProvider.Name(), clusterName)
				}
			}
			if strings.HasPrefix(name, "kplane-") {
				if err := kubectl.EnsureInstalled(); err != nil {
					return err
				}
				managementCtx := clusterProvider.ContextName(profile.ClusterName)
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

func normalizeContextName(name, providerPrefix string) string {
	if strings.HasPrefix(name, "kplane-") {
		return name
	}
	if providerPrefix != "" && strings.HasPrefix(name, providerPrefix) {
		return name
	}
	return "kplane-" + name
}

func newSetProviderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-provider <name>",
		Short: "Set the management plane provider for the current profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerName := args[0]
			clusterProvider, err := providers.New(providerName)
			if err != nil {
				return err
			}
			cfg := mustConfig()
			if err := setProfileProvider(cfg, clusterProvider.Name()); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "provider set to %s\n", clusterProvider.Name())
			return nil
		},
	}
	return cmd
}
