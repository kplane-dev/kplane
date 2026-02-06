package kubectl

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const binaryName = "kubectl"

func EnsureInstalled() error {
	if _, err := exec.LookPath(binaryName); err != nil {
		return fmt.Errorf("kubectl is not installed; install from https://kubernetes.io/docs/tasks/tools/")
	}
	return nil
}

type ApplyOptions struct {
	Context string
	Stdin   []byte
	Path    string
}

func Apply(ctx context.Context, opts ApplyOptions) error {
	args := []string{"apply"}
	if opts.Context != "" {
		args = append(args, "--context", opts.Context)
	}
	if opts.Path != "" {
		args = append(args, "-f", opts.Path)
	} else {
		args = append(args, "-f", "-")
	}
	cmd := exec.CommandContext(ctx, binaryName, args...)
	if opts.Stdin != nil {
		cmd.Stdin = bytes.NewReader(opts.Stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func ApplyKustomize(ctx context.Context, contextName, path string) error {
	args := []string{"apply"}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	args = append(args, "-k", path)
	cmd := exec.CommandContext(ctx, binaryName, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply -k: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func ApplyURL(ctx context.Context, contextName, url string) error {
	args := []string{"apply"}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	args = append(args, "-f", url)
	_, stderr, err := run(ctx, args...)
	if err != nil {
		return fmt.Errorf("kubectl apply -f: %s", strings.TrimSpace(stderr))
	}
	return nil
}

func CreateNamespace(ctx context.Context, contextName, name string) error {
	args := []string{"create", "namespace", name}
	if contextName != "" {
		args = append([]string{"--context", contextName}, args...)
	}
	cmd := exec.CommandContext(ctx, binaryName, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "AlreadyExists") {
			return nil
		}
		return fmt.Errorf("create namespace: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func LabelNodes(ctx context.Context, contextName string, labels map[string]string) error {
	args := []string{"label", "nodes", "--all"}
	if contextName != "" {
		args = append([]string{"--context", contextName}, args...)
	}
	for key, value := range labels {
		args = append(args, fmt.Sprintf("%s=%s", key, value))
	}
	args = append(args, "--overwrite")
	_, stderr, err := run(ctx, args...)
	if err != nil {
		return fmt.Errorf("kubectl label nodes: %s", strings.TrimSpace(stderr))
	}
	return nil
}

func RolloutStatus(ctx context.Context, contextName, namespace, kind, name string, timeout time.Duration) error {
	args := []string{"rollout", "status", fmt.Sprintf("%s/%s", kind, name), fmt.Sprintf("--timeout=%s", timeout.String())}
	if namespace != "" {
		args = append([]string{"-n", namespace}, args...)
	}
	if contextName != "" {
		args = append([]string{"--context", contextName}, args...)
	}
	_, stderr, err := run(ctx, args...)
	if err != nil {
		return fmt.Errorf("kubectl rollout status: %s", strings.TrimSpace(stderr))
	}
	return nil
}

func GetJSONPath(ctx context.Context, contextName, resource, name, namespace, jsonPath string) (string, error) {
	args := []string{"get", resource}
	if name != "" {
		args = append(args, name)
	}
	if namespace != "" {
		args = append([]string{"-n", namespace}, args...)
	}
	if contextName != "" {
		args = append([]string{"--context", contextName}, args...)
	}
	args = append(args, "-o", "jsonpath="+jsonPath)
	stdout, stderr, err := run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("kubectl get %s: %s", resource, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func GetSecretData(ctx context.Context, contextName, name, namespace, key string) ([]byte, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required for secret %q", name)
	}
	value, err := GetJSONPath(ctx, contextName, "secret", name, namespace, "{.data."+key+"}")
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, fmt.Errorf("secret %q missing key %q", name, key)
	}
	out, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode secret %q key %q: %w", name, key, err)
	}
	return out, nil
}

func run(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, binaryName, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
