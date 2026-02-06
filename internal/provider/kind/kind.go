package kind

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const binaryName = "kind"

func EnsureInstalled() error {
	if _, err := exec.LookPath(binaryName); err != nil {
		return fmt.Errorf("kind is not installed; install from https://kind.sigs.k8s.io/")
	}
	return nil
}

func ClusterExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, binaryName, "get", "clusters")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			return false, fmt.Errorf("list kind clusters: %w", err)
		}
		return false, fmt.Errorf("list kind clusters: %s", errMsg)
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func ListClusters(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, binaryName, "get", "clusters")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			return nil, fmt.Errorf("list kind clusters: %w", err)
		}
		return nil, fmt.Errorf("list kind clusters: %s", errMsg)
	}
	var clusters []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			clusters = append(clusters, name)
		}
	}
	return clusters, nil
}

type CreateOptions struct {
	Name       string
	NodeImage  string
	ConfigPath string
}

func CreateCluster(ctx context.Context, opts CreateOptions) error {
	args := []string{"create", "cluster", "--name", opts.Name}
	if opts.NodeImage != "" {
		args = append(args, "--image", opts.NodeImage)
	}
	if opts.ConfigPath != "" {
		args = append(args, "--config", opts.ConfigPath)
	}
	cmd := exec.CommandContext(ctx, binaryName, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create kind cluster: %w", err)
	}
	return nil
}

func DeleteCluster(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, binaryName, "delete", "cluster", "--name", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delete kind cluster: %w", err)
	}
	return nil
}

func GetKubeconfig(ctx context.Context, name string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binaryName, "get", "kubeconfig", "--name", name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("get kind kubeconfig: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
