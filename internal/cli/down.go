package cli

import (
	"github.com/kplane-dev/kplane/internal/providers"
	"github.com/spf13/cobra"
)

func newDownCommand() *cobra.Command {
	var (
		provider    string
		clusterName string
	)

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Delete the local management cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustConfig()
			profile, err := cfg.ActiveProfile()
			if err != nil {
				return err
			}
			if provider == "" {
				provider = profile.Provider
			}
			if clusterName == "" {
				clusterName = profile.ClusterName
			}
			clusterProvider, err := providers.New(provider)
			if err != nil {
				return err
			}
			if err := clusterProvider.EnsureInstalled(); err != nil {
				return err
			}
			return clusterProvider.DeleteCluster(cmd.Context(), clusterName)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cluster provider (default: kind)")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "Cluster name")

	return cmd
}
