package cli

import (
	"fmt"

	kindprovider "github.com/kplane-dev/kplane/internal/provider/kind"
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
			if provider != "kind" {
				return fmt.Errorf("unsupported provider %q", provider)
			}
			if err := kindprovider.EnsureInstalled(); err != nil {
				return err
			}
			return kindprovider.DeleteCluster(cmd.Context(), clusterName)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cluster provider (kind)")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "Cluster name")

	return cmd
}
