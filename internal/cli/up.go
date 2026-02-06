package cli

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/kplane-dev/kplane/internal/config"
	"github.com/kplane-dev/kplane/internal/kubeconfig"
	"github.com/kplane-dev/kplane/internal/kubectl"
	kindprovider "github.com/kplane-dev/kplane/internal/provider/kind"
	stacklatest "github.com/kplane-dev/kplane/internal/stack/latest"
	"github.com/spf13/cobra"
)

func newUpCommand() *cobra.Command {
	var (
		provider      string
		clusterName   string
		namespace     string
		apiserverImg  string
		operatorImg   string
		etcdImg       string
		stackVersion  string
		crdSource     string
		installCRDs   bool
		kubeconfigOut string
		setCurrent    bool
	)

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create or reuse a management plane on a local cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := mustConfig()
			profile, err := cfg.ActiveProfile()
			if err != nil {
				return err
			}

			applyUpDefaults(&provider, &clusterName, &namespace, &apiserverImg, &operatorImg, &etcdImg, &stackVersion, &crdSource, &kubeconfigOut, &setCurrent, profile)

			if provider != "kind" {
				return fmt.Errorf("unsupported provider %q", provider)
			}
			if err := kindprovider.EnsureInstalled(); err != nil {
				return err
			}
			if err := kubectl.EnsureInstalled(); err != nil {
				return err
			}

			ctx := cmd.Context()
			exists, err := kindprovider.ClusterExists(ctx, clusterName)
			if err != nil {
				return err
			}
			var ingressPort int
			if !exists {
				ingressPort, err = resolveIngressPort(profile.Kind.IngressPort)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "kind: creating management cluster %s\n", clusterName)
				configPath := profile.Kind.ConfigPath
				if configPath == "" {
					configPath, err = writeKindConfig(ingressPort)
					if err != nil {
						return err
					}
				}
				if err := kindprovider.CreateCluster(ctx, kindprovider.CreateOptions{
					Name:       clusterName,
					NodeImage:  profile.Kind.NodeImage,
					ConfigPath: configPath,
				}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "kind: reusing existing cluster %s\n", clusterName)
				ingressPort = resolveIngressPortFromCluster(ctx, fmt.Sprintf("kind-%s", clusterName), namespace)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "kubeconfig: updating (set current=%t)\n", setCurrent)
			kubeconfigData, err := kindprovider.GetKubeconfig(ctx, clusterName)
			if err != nil {
				return err
			}
			if err := kubeconfig.MergeAndWrite(kubeconfigOut, kubeconfigData, setCurrent); err != nil {
				return err
			}

			contextName := fmt.Sprintf("kind-%s", clusterName)
			if err := kubectl.LabelNodes(ctx, contextName, map[string]string{"ingress-ready": "true"}); err != nil {
				return err
			}
			resolvedVersion, err := resolveStackVersion(stackVersion)
			if err != nil {
				return err
			}

			switch resolvedVersion {
			case "latest":
				fmt.Fprintf(cmd.OutOrStdout(), "stack: installing %s\n", resolvedVersion)
				if err := stacklatest.Install(cmd.Context(), stacklatest.InstallOptions{
					Context:   contextName,
					Namespace: namespace,
					Images: stacklatest.Images{
						Apiserver: apiserverImg,
						Operator:  operatorImg,
						Etcd:      etcdImg,
					},
					CRDSource:   crdSource,
					InstallCRDs: installCRDs,
					Logf: func(format string, args ...any) {
						fmt.Fprintf(cmd.OutOrStdout(), "stack: %s\n", fmt.Sprintf(format, args...))
					},
				}); err != nil {
					return err
				}
				if err := applyIngressConfig(cmd.Context(), contextName, namespace, ingressPort); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "ready: management plane is up")
				fmt.Fprintf(cmd.OutOrStdout(), "next: create a control plane with `kplane create cluster <name>`\n")
				return nil
			default:
				return fmt.Errorf("unsupported stack version %q", resolvedVersion)
			}
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cluster provider (kind)")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "Cluster name")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace for kplane system")
	cmd.Flags().StringVar(&apiserverImg, "apiserver-image", "", "Apiserver image")
	cmd.Flags().StringVar(&operatorImg, "operator-image", "", "Controlplane-operator image")
	cmd.Flags().StringVar(&etcdImg, "etcd-image", "", "Etcd image")
	cmd.Flags().StringVar(&stackVersion, "stack-version", "", "Stack version to install")
	cmd.Flags().StringVar(&crdSource, "crd-source", "", "CRD source (kustomize URL or path)")
	cmd.Flags().BoolVar(&installCRDs, "install-crds", true, "Install CRDs before deploying operator")
	cmd.Flags().StringVar(&kubeconfigOut, "kubeconfig", "", "Kubeconfig path to update")
	cmd.Flags().BoolVar(&setCurrent, "set-current", true, "Set current kubeconfig context")

	return cmd
}

func applyUpDefaults(provider, clusterName, namespace, apiserverImg, operatorImg, etcdImg, stackVersion, crdSource, kubeconfigOut *string, setCurrent *bool, profile config.Profile) {
	if *provider == "" {
		*provider = profile.Provider
	}
	if *clusterName == "" {
		*clusterName = profile.ClusterName
	}
	if *namespace == "" {
		*namespace = profile.Namespace
	}
	if *apiserverImg == "" {
		*apiserverImg = profile.Images.Apiserver
	}
	if *operatorImg == "" {
		*operatorImg = profile.Images.Operator
	}
	if *etcdImg == "" {
		*etcdImg = profile.Images.Etcd
	}
	if *stackVersion == "" {
		*stackVersion = profile.StackVersion
	}
	if *crdSource == "" {
		*crdSource = profile.CRDSource
	}
	if *kubeconfigOut == "" {
		*kubeconfigOut = profile.KubeconfigPath
	}
	_ = setCurrent
}

func resolveStackVersion(requested string) (string, error) {
	if requested == "" || requested == "latest" {
		return latestStackVersion(), nil
	}
	return "", fmt.Errorf("unsupported stack version %q (use latest)", requested)
}

func latestStackVersion() string {
	return "latest"
}

func writeKindConfig(ingressPort int) (string, error) {
	content := fmt.Sprintf(`kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 443
        hostPort: %d
        listenAddress: "127.0.0.1"
`, ingressPort)
	file, err := os.CreateTemp("", "kplane-kind-config-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create kind config: %w", err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write kind config: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close kind config: %w", err)
	}
	return file.Name(), nil
}

func resolveIngressPort(requested int) (int, error) {
	if requested > 0 {
		if err := ensurePortAvailable(requested); err != nil {
			return 0, err
		}
		return requested, nil
	}

	const defaultPort = 8443
	if err := ensurePortAvailable(defaultPort); err == nil {
		return defaultPort, nil
	}
	return findFreePort()
}

func applyIngressConfig(ctx context.Context, contextName, namespace string, port int) error {
	if namespace == "" {
		namespace = "kplane-system"
	}
	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  ingressPort: "%d"
`, ingressConfigName, namespace, port)
	return kubectl.Apply(ctx, kubectl.ApplyOptions{
		Context: contextName,
		Stdin:   []byte(manifest),
	})
}

func ensurePortAvailable(port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ingress port %d is already in use", port)
	}
	_ = listener.Close()
	return nil
}

func findFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find free ingress port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
