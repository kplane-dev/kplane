package kind

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const binaryName = "kind"

func EnsureInstalled() error {
	if _, err := resolveBinary(); err != nil {
		return fmt.Errorf("kind is not installed; install from https://kind.sigs.k8s.io/")
	}
	if err := validateKind(); err != nil {
		return err
	}
	return nil
}

func validateKind() error {
	bin, err := resolveBinary()
	if err != nil {
		return fmt.Errorf("kind is not installed; install from https://kind.sigs.k8s.io/")
	}
	if isGoenvShim(bin) {
		return fmt.Errorf("kind is not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
	}
	cmd := exec.Command(bin, "version")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if isGoenvMissing(msg) {
			return fmt.Errorf("kind is not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
		}
		if msg == "" {
			return fmt.Errorf("kind is not installed; install from https://kind.sigs.k8s.io/")
		}
		return fmt.Errorf("kind is not available: %s", msg)
	}
	return nil
}

func isGoenvMissing(msg string) bool {
	return strings.Contains(msg, "goenv:") && strings.Contains(msg, "command not found")
}

func isGoenvShim(path string) bool {
	return strings.Contains(path, string(filepath.Separator)+".goenv"+string(filepath.Separator)+"shims"+string(filepath.Separator))
}

func resolveBinary() (string, error) {
	pathEnv := os.Getenv("PATH")
	for _, dir := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if dir == "" || strings.Contains(dir, string(filepath.Separator)+".goenv"+string(filepath.Separator)+"shims") {
			continue
		}
		candidate := filepath.Join(dir, binaryName)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return exec.LookPath(binaryName)
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0o111 != 0
}

func ClusterExists(ctx context.Context, name string) (bool, error) {
	bin, err := resolveBinary()
	if err != nil {
		return false, fmt.Errorf("list kind clusters: kind not installed; install from https://kind.sigs.k8s.io/")
	}
	if isGoenvShim(bin) {
		return false, fmt.Errorf("list kind clusters: kind not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
	}
	cmd := exec.CommandContext(ctx, bin, "get", "clusters")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if isGoenvMissing(errMsg) {
			return false, fmt.Errorf("list kind clusters: kind not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
		}
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
	bin, err := resolveBinary()
	if err != nil {
		return nil, fmt.Errorf("list kind clusters: kind not installed; install from https://kind.sigs.k8s.io/")
	}
	if isGoenvShim(bin) {
		return nil, fmt.Errorf("list kind clusters: kind not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
	}
	cmd := exec.CommandContext(ctx, bin, "get", "clusters")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if isGoenvMissing(errMsg) {
			return nil, fmt.Errorf("list kind clusters: kind not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
		}
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
	bin, err := resolveBinary()
	if err != nil {
		return fmt.Errorf("create kind cluster: kind not installed; install from https://kind.sigs.k8s.io/")
	}
	if isGoenvShim(bin) {
		return fmt.Errorf("create kind cluster: kind not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
	}
	args := []string{"create", "cluster", "--name", opts.Name}
	if opts.NodeImage != "" {
		args = append(args, "--image", opts.NodeImage)
	}
	if opts.ConfigPath != "" {
		args = append(args, "--config", opts.ConfigPath)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create kind cluster: %w", err)
	}
	return nil
}

func DeleteCluster(ctx context.Context, name string) error {
	bin, err := resolveBinary()
	if err != nil {
		return fmt.Errorf("delete kind cluster: kind not installed; install from https://kind.sigs.k8s.io/")
	}
	if isGoenvShim(bin) {
		return fmt.Errorf("delete kind cluster: kind not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
	}
	cmd := exec.CommandContext(ctx, bin, "delete", "cluster", "--name", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delete kind cluster: %w", err)
	}
	return nil
}

func GetKubeconfig(ctx context.Context, name string) ([]byte, error) {
	bin, err := resolveBinary()
	if err != nil {
		return nil, fmt.Errorf("get kind kubeconfig: kind not installed; install from https://kind.sigs.k8s.io/")
	}
	if isGoenvShim(bin) {
		return nil, fmt.Errorf("get kind kubeconfig: kind not installed (goenv shim active); install via brew install kind or ensure kind is available in your goenv")
	}
	cmd := exec.CommandContext(ctx, bin, "get", "kubeconfig", "--name", name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("get kind kubeconfig: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
