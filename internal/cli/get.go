package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/kplane-dev/kplane/internal/kubeconfig"
	"github.com/kplane-dev/kplane/internal/kubectl"
	"github.com/kplane-dev/kplane/internal/providers"
	"github.com/spf13/cobra"
)

func newGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get resources",
	}
	cmd.AddCommand(newGetClustersCommand())
	return cmd
}

func newGetClustersCommand() *cobra.Command {
	var kubeconfigPath string

	cmd := &cobra.Command{
		Use:   "clusters",
		Short: "List kplane and management contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustConfig()
			profile, err := cfg.ActiveProfile()
			if err != nil {
				return err
			}
			if kubeconfigPath == "" {
				kubeconfigPath = profile.KubeconfigPath
			}

			_, current, err := kubeconfig.ListContexts(kubeconfigPath)
			if err != nil {
				return err
			}

			clusterProvider, err := providers.New(profile.Provider)
			if err != nil {
				return err
			}
			if err := clusterProvider.EnsureInstalled(); err != nil {
				return err
			}

			clusters, err := clusterProvider.ListClusters(cmd.Context())
			if err != nil {
				return err
			}
			sort.Strings(clusters)

			for _, cluster := range clusters {
				ctxName := clusterProvider.ContextName(cluster)
				marker := " "
				if ctxName == current {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", marker, ctxName)
			}

			managementCtx := clusterProvider.ContextName(profile.ClusterName)
			controlplanes, err := listControlPlanes(cmd.Context(), managementCtx)
			if err != nil {
				return err
			}
			sort.Strings(controlplanes)
			for _, name := range controlplanes {
				ctxName := "kplane-" + name
				marker := " "
				if ctxName == current {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", marker, ctxName)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Kubeconfig path to read")
	return cmd
}

func isKplaneContext(name, providerPrefix, managementCluster string) bool {
	if strings.HasPrefix(name, "kplane-") {
		return true
	}
	if providerPrefix != "" && managementCluster != "" && name == providerPrefix+managementCluster {
		return true
	}
	return false
}

func listControlPlanes(ctx context.Context, managementCtx string) ([]string, error) {
	out, err := kubectl.GetJSONPath(ctx, managementCtx, "controlplanes", "", "", "{.items[*].metadata.name}")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	return strings.Fields(out), nil
}
