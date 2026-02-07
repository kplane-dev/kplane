package kind

import (
	"context"

	"github.com/kplane-dev/kplane/internal/provider"
)

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "kind"
}

func (p *Provider) ContextPrefix() string {
	return "kind-"
}

func (p *Provider) ContextName(clusterName string) string {
	return p.ContextPrefix() + clusterName
}

func (p *Provider) EnsureInstalled() error {
	return EnsureInstalled()
}

func (p *Provider) ClusterExists(ctx context.Context, name string) (bool, error) {
	return ClusterExists(ctx, name)
}

func (p *Provider) ListClusters(ctx context.Context) ([]string, error) {
	return ListClusters(ctx)
}

func (p *Provider) CreateCluster(ctx context.Context, opts provider.CreateClusterOptions) error {
	return CreateCluster(ctx, CreateOptions{
		Name:       opts.Name,
		NodeImage:  opts.NodeImage,
		ConfigPath: opts.ConfigPath,
	})
}

func (p *Provider) DeleteCluster(ctx context.Context, name string) error {
	return DeleteCluster(ctx, name)
}

func (p *Provider) GetKubeconfig(ctx context.Context, name string) ([]byte, error) {
	return GetKubeconfig(ctx, name)
}
