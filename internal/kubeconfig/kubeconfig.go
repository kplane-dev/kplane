package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func MergeAndWrite(path string, kubeconfigData []byte, setCurrent bool) error {
	newConfig, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}

	existing := clientcmdapi.NewConfig()
	if data, err := os.ReadFile(path); err == nil {
		existing, err = clientcmd.Load(data)
		if err != nil {
			return fmt.Errorf("parse existing kubeconfig: %w", err)
		}
	}

	merged := mergeConfigs(existing, newConfig, setCurrent)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create kubeconfig dir: %w", err)
	}

	out, err := clientcmd.Write(*merged)
	if err != nil {
		return fmt.Errorf("serialize kubeconfig: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write kubeconfig: %w", err)
	}
	return nil
}

func RewriteServer(kubeconfigData []byte, server string) ([]byte, error) {
	cfg, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	if server != "" {
		for _, cluster := range cfg.Clusters {
			cluster.Server = server
		}
	}
	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return nil, fmt.Errorf("serialize kubeconfig: %w", err)
	}
	return out, nil
}

func RenameContext(kubeconfigData []byte, name string) ([]byte, error) {
	cfg, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	if name == "" {
		return kubeconfigData, nil
	}

	// Rename clusters
	newClusters := map[string]*clientcmdapi.Cluster{}
	for oldName, cluster := range cfg.Clusters {
		newClusters[name] = cluster
		if cfg.CurrentContext == oldName {
			cfg.CurrentContext = name
		}
	}
	cfg.Clusters = newClusters

	// Rename users
	newUsers := map[string]*clientcmdapi.AuthInfo{}
	for oldName, user := range cfg.AuthInfos {
		newUsers[name] = user
		if cfg.CurrentContext == oldName {
			cfg.CurrentContext = name
		}
	}
	cfg.AuthInfos = newUsers

	// Rename contexts
	newContexts := map[string]*clientcmdapi.Context{}
	for _, ctx := range cfg.Contexts {
		ctx.Cluster = name
		ctx.AuthInfo = name
		newContexts[name] = ctx
	}
	cfg.Contexts = newContexts
	cfg.CurrentContext = name

	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return nil, fmt.Errorf("serialize kubeconfig: %w", err)
	}
	return out, nil
}

func SetCurrentContext(path, name string) error {
	if name == "" {
		return fmt.Errorf("context name is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read kubeconfig: %w", err)
	}
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}
	if _, ok := cfg.Contexts[name]; !ok {
		return fmt.Errorf("context %q not found in kubeconfig", name)
	}
	cfg.CurrentContext = name
	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return fmt.Errorf("serialize kubeconfig: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write kubeconfig: %w", err)
	}
	return nil
}

func ListContexts(path string) ([]string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read kubeconfig: %w", err)
	}
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return nil, "", fmt.Errorf("parse kubeconfig: %w", err)
	}
	names := make([]string, 0, len(cfg.Contexts))
	for name := range cfg.Contexts {
		names = append(names, name)
	}
	return names, cfg.CurrentContext, nil
}

func mergeConfigs(base, incoming *clientcmdapi.Config, setCurrent bool) *clientcmdapi.Config {
	if base == nil {
		base = clientcmdapi.NewConfig()
	}
	if incoming == nil {
		return base
	}

	for name, cluster := range incoming.Clusters {
		base.Clusters[name] = cluster
	}
	for name, auth := range incoming.AuthInfos {
		base.AuthInfos[name] = auth
	}
	for name, ctx := range incoming.Contexts {
		base.Contexts[name] = ctx
	}
	if setCurrent && incoming.CurrentContext != "" {
		base.CurrentContext = incoming.CurrentContext
	}
	return base
}
