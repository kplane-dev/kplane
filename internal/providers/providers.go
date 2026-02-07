package providers

import (
	"fmt"

	"github.com/kplane-dev/kplane/internal/managementprovider"
	k3sprovider "github.com/kplane-dev/kplane/internal/managementprovider/k3s"
	kindprovider "github.com/kplane-dev/kplane/internal/managementprovider/kind"
)

func New(name string) (managementprovider.Provider, error) {
	if name == "" || name == "kind" {
		return kindprovider.New(), nil
	}
	if name == "k3s" || name == "k3d" {
		return k3sprovider.New(), nil
	}
	return nil, fmt.Errorf("unsupported provider %q", name)
}
