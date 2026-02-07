package provider

import "context"

type CreateClusterOptions struct {
	Name        string
	NodeImage   string
	ConfigPath  string
	IngressPort int
}

type Provider interface {
	Name() string
	ContextPrefix() string
	ContextName(clusterName string) string
	EnsureInstalled() error
	ClusterExists(ctx context.Context, name string) (bool, error)
	ListClusters(ctx context.Context) ([]string, error)
	CreateCluster(ctx context.Context, opts CreateClusterOptions) error
	DeleteCluster(ctx context.Context, name string) error
	GetKubeconfig(ctx context.Context, name string) ([]byte, error)
}
