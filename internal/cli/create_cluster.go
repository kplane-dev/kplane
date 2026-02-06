package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/kplane-dev/kplane/internal/kubeconfig"
	"github.com/kplane-dev/kplane/internal/kubectl"
	"github.com/spf13/cobra"
)

func newCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create resources",
	}
	cmd.AddCommand(newCreateClusterCommand())
	return cmd
}

func newCreateClusterCommand() *cobra.Command {
	var (
		name           string
		className      string
		endpoint       string
		getCredentials bool
		setCurrent     bool
		kubeconfigOut  string
		timeout        time.Duration
		namespace      string
		managementCtx  string
	)

	cmd := &cobra.Command{
		Use:   "cluster <name>",
		Short: "Create a ControlPlane resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustConfig()
			profile, err := cfg.ActiveProfile()
			if err != nil {
				return err
			}

			if name == "" {
				name = args[0]
			}
			if className == "" {
				className = "starter"
			}
			if namespace == "" {
				namespace = profile.Namespace
			}
			if kubeconfigOut == "" {
				kubeconfigOut = profile.KubeconfigPath
			}
			if timeout == 0 {
				timeout = 5 * time.Minute
			}
			if managementCtx == "" {
				managementCtx = fmt.Sprintf("kind-%s", profile.ClusterName)
			}

			if err := kubectl.EnsureInstalled(); err != nil {
				return err
			}

			internalEndpoint := defaultInternalEndpoint(namespace, name)
			externalEndpoint, err := resolveExternalEndpoint(cmd.Context(), managementCtx, namespace, name, endpoint)
			if err != nil {
				return err
			}
			manifest := renderControlPlaneManifest(name, className, internalEndpoint, externalEndpoint)
			if err := kubectl.Apply(cmd.Context(), kubectl.ApplyOptions{
				Context: managementCtx,
				Stdin:   []byte(manifest),
			}); err != nil {
				return err
			}

			if !getCredentials {
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "waiting for controlplane to reconcile...")
			if err := waitForControlPlaneReady(cmd.Context(), managementCtx, name, timeout); err != nil {
				return err
			}
			secretName := "apiserver-kubeconfig"
			secretNamespace := namespace
			kubeconfigData, err := kubectl.GetSecretData(cmd.Context(), managementCtx, secretName, secretNamespace, "kubeconfig")
			if err != nil {
				return err
			}
			kubeconfigData, err = kubeconfig.RewriteServer(kubeconfigData, externalEndpoint)
			if err != nil {
				return err
			}
			kubeconfigData, err = kubeconfig.RenameContext(kubeconfigData, fmt.Sprintf("kplane-%s", name))
			if err != nil {
				return err
			}
			if err := kubeconfig.MergeAndWrite(kubeconfigOut, kubeconfigData, setCurrent); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated kubeconfig (current context=%t)\n", setCurrent)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "ControlPlane name")
	cmd.Flags().StringVar(&className, "class", "", "ControlPlaneClass name")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "ControlPlane endpoint URL")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Management namespace (used for default endpoint)")
	cmd.Flags().BoolVar(&getCredentials, "get-credentials", true, "Fetch and merge kubeconfig for the control plane")
	cmd.Flags().BoolVar(&setCurrent, "set-current", true, "Set current kubeconfig context")
	cmd.Flags().StringVar(&kubeconfigOut, "kubeconfig", "", "Kubeconfig path to update")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Wait timeout for controlplane readiness")
	cmd.Flags().StringVar(&managementCtx, "management-context", "", "Kubeconfig context for management plane (kind-<cluster>)")
	return cmd
}

func renderControlPlaneManifest(name, className, internalEndpoint, externalEndpoint string) string {
	return fmt.Sprintf(`apiVersion: controlplane.kplane.dev/v1alpha1
kind: ControlPlaneEndpoint
metadata:
  name: %s-endpoint
spec:
  endpoint: %s
  externalEndpoint: %s
---
apiVersion: controlplane.kplane.dev/v1alpha1
kind: ControlPlane
metadata:
  name: %s
spec:
  classRef:
    name: %s
  endpointRef:
    name: %s-endpoint
`, name, internalEndpoint, externalEndpoint, name, className, name)
}

func defaultInternalEndpoint(namespace, controlPlaneName string) string {
	if namespace == "" {
		namespace = "kplane-system"
	}
	return fmt.Sprintf("https://kplane-apiserver.%s.svc.cluster.local:6443/clusters/%s/control-plane", namespace, controlPlaneName)
}

func defaultExternalEndpoint(_ context.Context, _ string, ingressPort int, controlPlaneName string) (string, error) {
	return fmt.Sprintf("https://127.0.0.1:%d/clusters/%s/control-plane", ingressPort, controlPlaneName), nil
}

const (
	defaultIngressPort = 8443
	ingressConfigName  = "kplane-management"
)

func resolveExternalEndpoint(ctx context.Context, managementCtx, namespace, controlPlaneName, provided string) (string, error) {
	if provided != "" {
		return provided, nil
	}
	ingressPort := resolveIngressPortFromCluster(ctx, managementCtx, namespace)
	return defaultExternalEndpoint(ctx, managementCtx, ingressPort, controlPlaneName)
}

func resolveIngressPortFromCluster(ctx context.Context, managementCtx, namespace string) int {
	if namespace == "" {
		namespace = "kplane-system"
	}
	portStr, err := kubectl.GetJSONPath(ctx, managementCtx, "configmap", ingressConfigName, namespace, "{.data.ingressPort}")
	if err != nil || portStr == "" {
		return defaultIngressPort
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultIngressPort
	}
	return port
}

func waitForControlPlaneReady(ctx context.Context, managementCtx, controlPlaneName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastLog := time.Time{}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for controlplane to be ready")
		case <-ticker.C:
			ready, err := kubectl.GetJSONPath(ctx, managementCtx, "controlplane", controlPlaneName, "", "{.status.conditions[?(@.type==\"Ready\")].status}")
			if err != nil {
				logWaitStatus(ctx, managementCtx, controlPlaneName, &lastLog)
				continue
			}
			if ready == "True" {
				return nil
			}
			logWaitStatus(ctx, managementCtx, controlPlaneName, &lastLog)
		}
	}
}

func logWaitStatus(ctx context.Context, managementCtx, controlPlaneName string, lastLog *time.Time) {
	if lastLog != nil && time.Since(*lastLog) < 10*time.Second {
		return
	}
	endpoint, _ := kubectl.GetJSONPath(ctx, managementCtx, "controlplane", controlPlaneName, "", "{.status.endpoint}")
	ready, _ := kubectl.GetJSONPath(ctx, managementCtx, "controlplane", controlPlaneName, "", "{.status.conditions[?(@.type==\"Ready\")].status}")
	if endpoint == "" && ready == "" {
		fmt.Println("waiting for controlplane to reconcile...")
	} else {
		fmt.Printf("controlplane status: ready=%s endpoint=%s\n", ready, endpoint)
	}
	if lastLog != nil {
		*lastLog = time.Now()
	}
}
