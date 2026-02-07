package cli

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/kplane-dev/kplane/internal/config"
	"github.com/kplane-dev/kplane/internal/kubeconfig"
	"github.com/kplane-dev/kplane/internal/kubectl"
	managementprovider "github.com/kplane-dev/kplane/internal/managementprovider"
	"github.com/kplane-dev/kplane/internal/providers"
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
		quiet         bool
		noColor       bool
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
			showNext := profile.UI.UpHintCount < 3
			ui := NewUI(cmd.OutOrStdout(), profile.UI.Enabled && !quiet, profile.UI.Color && !noColor)
			if ui.Enabled() {
				printBanner(cmd.OutOrStdout())
			}

			applyUpDefaults(&provider, &clusterName, &namespace, &apiserverImg, &operatorImg, &etcdImg, &stackVersion, &crdSource, &kubeconfigOut, &setCurrent, profile)

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

			ctx := cmd.Context()
			exists, err := clusterProvider.ClusterExists(ctx, clusterName)
			if err != nil {
				return err
			}
			var ingressPort int
			providerName := clusterProvider.Name()
			if !exists {
				if err := ui.Step(providerName+": creating management cluster "+clusterName, func() error {
					var err error
					ingressPort, err = resolveIngressPort(resolveIngressPortSetting(providerName, profile))
					if err != nil {
						return err
					}
					createOpts, err := buildCreateOptions(providerName, profile, ingressPort)
					if err != nil {
						return err
					}
					return clusterProvider.CreateCluster(ctx, managementprovider.CreateClusterOptions{
						Name:        clusterName,
						NodeImage:   createOpts.NodeImage,
						ConfigPath:  createOpts.ConfigPath,
						IngressPort: ingressPort,
					})
				}); err != nil {
					return err
				}
			} else {
				if ui.Enabled() {
					ui.Infof("%s: reusing existing cluster %s", providerName, clusterName)
				}
				ingressPort = resolveIngressPortFromCluster(ctx, clusterProvider.ContextName(clusterName), namespace)
			}

			if err := ui.Step("kubeconfig: updating", func() error {
				kubeconfigData, err := clusterProvider.GetKubeconfig(ctx, clusterName)
				if err != nil {
					return err
				}
				return kubeconfig.MergeAndWrite(kubeconfigOut, kubeconfigData, setCurrent)
			}); err != nil {
				return err
			}

			contextName := clusterProvider.ContextName(clusterName)
			if err := ui.Step("nodes: labeling ingress-ready", func() error {
				return kubectl.LabelNodes(ctx, contextName, map[string]string{"ingress-ready": "true"})
			}); err != nil {
				return err
			}
			resolvedVersion, err := resolveStackVersion(stackVersion)
			if err != nil {
				return err
			}

			switch resolvedVersion {
			case "latest":
				if err := ui.Step("stack: installing management plane", func() error {
					return stacklatest.Install(cmd.Context(), stacklatest.InstallOptions{
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
							msg := fmt.Sprintf(format, args...)
							if ui.Enabled() {
								ui.Infof("stack: %s", msg)
							} else {
								fmt.Fprintf(cmd.OutOrStdout(), "stack: %s\n", msg)
							}
						},
					})
				}); err != nil {
					return err
				}
				if err := ui.Step("ingress: recording port", func() error {
					return applyIngressConfig(cmd.Context(), contextName, namespace, ingressPort)
				}); err != nil {
					return err
				}
				if ui.Enabled() {
					ui.Successf("ready: management plane is up")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "ready: management plane is up")
				}
				if showNext {
					if ui.Enabled() {
						ui.Infof("")
						ui.Successf("next: create a control plane")
						ui.Successf("  kplane create cluster <name>")
					} else {
						fmt.Fprintln(cmd.OutOrStdout())
						fmt.Fprintln(cmd.OutOrStdout(), "next: create a control plane")
						fmt.Fprintln(cmd.OutOrStdout(), "  kplane create cluster <name>")
					}
				}
				if err := setProfileProvider(cfg, providerName); err != nil {
					return err
				}
				_ = markUICompletion(cfg, true, false)
				return nil
			default:
				return fmt.Errorf("unsupported stack version %q", resolvedVersion)
			}
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Cluster provider (default: kind)")
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
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Disable progress output")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")

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

type createOptions struct {
	NodeImage  string
	ConfigPath string
}

func resolveIngressPortSetting(providerName string, profile config.Profile) int {
	switch providerName {
	case "k3s":
		return profile.K3s.IngressPort
	default:
		return profile.Kind.IngressPort
	}
}

func buildCreateOptions(providerName string, profile config.Profile, ingressPort int) (createOptions, error) {
	switch providerName {
	case "k3s":
		return createOptions{NodeImage: profile.K3s.Image}, nil
	default:
		configPath := profile.Kind.ConfigPath
		if configPath == "" {
			var err error
			configPath, err = writeKindConfig(ingressPort)
			if err != nil {
				return createOptions{}, err
			}
		}
		return createOptions{
			NodeImage:  profile.Kind.NodeImage,
			ConfigPath: configPath,
		}, nil
	}
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
