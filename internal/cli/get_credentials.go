package cli

import (
	"fmt"
	"time"

	"github.com/kplane-dev/kplane/internal/kubeconfig"
	"github.com/kplane-dev/kplane/internal/kubectl"
	"github.com/kplane-dev/kplane/internal/providers"
	"github.com/spf13/cobra"
)

func newGetCredentialsCommand() *cobra.Command {
	var (
		provider      string
		clusterName   string
		kubeconfigOut string
		setCurrent    bool
		region        string
		project       string
	)

	cmd := &cobra.Command{
		Use:   "get-credentials <cluster-name>",
		Short: "Update kubeconfig for a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustConfig()
			profile, err := cfg.ActiveProfile()
			if err != nil {
				return err
			}

			if clusterName == "" {
				clusterName = args[0]
			}
			if provider == "" {
				provider = profile.Provider
			}
			if kubeconfigOut == "" {
				kubeconfigOut = profile.KubeconfigPath
			}

			clusterProvider, err := providers.New(provider)
			if err != nil {
				return err
			}
			if err := clusterProvider.EnsureInstalled(); err != nil {
				return err
			}
			if err := kubectl.EnsureInstalled(); err != nil {
				return err
			}

			_ = region
			_ = project

			if exists, err := clusterProvider.ClusterExists(cmd.Context(), clusterName); err != nil {
				return err
			} else if exists {
				kubeconfigData, err := clusterProvider.GetKubeconfig(cmd.Context(), clusterName)
				if err != nil {
					return err
				}
				return kubeconfig.MergeAndWrite(kubeconfigOut, kubeconfigData, setCurrent)
			}

			managementCtx := clusterProvider.ContextName(profile.ClusterName)
			timeout := 5 * time.Minute
			ready, err := kubectl.GetJSONPath(cmd.Context(), managementCtx, "controlplane", clusterName, "", "{.status.conditions[?(@.type==\"Ready\")].status}")
			if err != nil || ready != "True" {
				fmt.Fprintln(cmd.OutOrStdout(), "waiting for controlplane to reconcile...")
				if err := waitForControlPlaneReady(cmd.Context(), managementCtx, clusterName, timeout, nil); err != nil {
					return err
				}
			}

			externalEndpoint, err := defaultExternalEndpoint(cmd.Context(), managementCtx, resolveIngressPortFromCluster(cmd.Context(), managementCtx, profile.Namespace), clusterName)
			if err != nil {
				return err
			}
			secretName, secretNamespace, err := controlPlaneKubeconfigRef(cmd.Context(), managementCtx, clusterName, profile.Namespace)
			if err != nil {
				return err
			}
			kubeconfigData, err := kubectl.GetSecretData(cmd.Context(), managementCtx, secretName, secretNamespace, "kubeconfig")
			if err != nil {
				return err
			}
			kubeconfigData, err = kubeconfig.RewriteServer(kubeconfigData, externalEndpoint)
			if err != nil {
				return err
			}
			kubeconfigData, err = kubeconfig.RenameContext(kubeconfigData, fmt.Sprintf("kplane-%s", clusterName))
			if err != nil {
				return err
			}
			return kubeconfig.MergeAndWrite(kubeconfigOut, kubeconfigData, setCurrent)
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cluster provider (default: kind)")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "Cluster name")
	cmd.Flags().StringVar(&kubeconfigOut, "kubeconfig", "", "Kubeconfig path to update")
	cmd.Flags().BoolVar(&setCurrent, "set-current", true, "Set current kubeconfig context")
	cmd.Flags().StringVar(&region, "region", "", "Region (provider specific)")
	cmd.Flags().StringVar(&project, "project", "", "Project (provider specific)")

	return cmd
}
