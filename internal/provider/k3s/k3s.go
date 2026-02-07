package k3s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const binaryName = "k3d"

func EnsureInstalled() error {
	if _, err := exec.LookPath(binaryName); err != nil {
		return fmt.Errorf("k3d is not installed; install from https://k3d.io/")
	}
	return nil
}

func ClusterExists(ctx context.Context, name string) (bool, error) {
	clusters, err := ListClusters(ctx)
	if err != nil {
		return false, err
	}
	for _, cluster := range clusters {
		if cluster == name {
			return true, nil
		}
	}
	return false, nil
}

func ListClusters(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, binaryName, "cluster", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			return nil, fmt.Errorf("list k3d clusters: %w", err)
		}
		return nil, fmt.Errorf("list k3d clusters: %s", errMsg)
	}
	var clusters []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "NAME") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			clusters = append(clusters, fields[0])
		}
	}
	return clusters, nil
}

type CreateOptions struct {
	Name        string
	Image       string
	IngressPort int
}

func CreateCluster(ctx context.Context, opts CreateOptions) error {
	args := []string{"cluster", "create", opts.Name}
	if opts.Image != "" {
		args = append(args, "--image", opts.Image)
	}
	args = append(args, "--k3s-arg", "--disable=traefik@server:0")
	args = append(args, "--k3s-arg", "--disable=servicelb@server:0")
	if opts.IngressPort > 0 {
		args = append(args, "--port", fmt.Sprintf("%d:443@loadbalancer", opts.IngressPort))
	}
	cmd := exec.CommandContext(ctx, binaryName, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create k3d cluster: %w", err)
	}
	return nil
}

func DeleteCluster(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, binaryName, "cluster", "delete", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delete k3d cluster: %w", err)
	}
	return nil
}

func GetKubeconfig(ctx context.Context, name string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binaryName, "kubeconfig", "get", name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("get k3d kubeconfig: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
