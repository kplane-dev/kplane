package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigFileName = "config.yaml"
)

type Config struct {
	APIVersion     string             `yaml:"apiVersion"`
	Kind           string             `yaml:"kind"`
	CurrentProfile string             `yaml:"currentProfile"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Provider       string   `yaml:"provider"`
	ClusterName    string   `yaml:"clusterName"`
	Namespace      string   `yaml:"namespace"`
	KubeconfigPath string   `yaml:"kubeconfigPath"`
	StackVersion   string   `yaml:"stackVersion"`
	CRDSource      string   `yaml:"crdSource"`
	Images         Images   `yaml:"images"`
	Auth           Auth     `yaml:"auth"`
	Kind           KindOpts `yaml:"kind"`
	K3s            K3sOpts  `yaml:"k3s"`
	UI             UIOpts   `yaml:"ui"`
}

type Images struct {
	Apiserver string `yaml:"apiserver"`
	Operator  string `yaml:"operator"`
	Etcd      string `yaml:"etcd"`
}

type Auth struct {
	Policy         string `yaml:"policy"`
	IssuerTemplate string `yaml:"issuerTemplate"`
}

type KindOpts struct {
	NodeImage   string `yaml:"nodeImage"`
	ConfigPath  string `yaml:"configPath"`
	IngressPort int    `yaml:"ingressPort"`
}

type K3sOpts struct {
	Image       string `yaml:"image"`
	IngressPort int    `yaml:"ingressPort"`
}

type UIOpts struct {
	Enabled         bool `yaml:"enabled"`
	Color           bool `yaml:"color"`
	UpHintCount     int  `yaml:"upHintCount"`
	CreateHintCount int  `yaml:"createHintCount"`
}

func Default() Config {
	return Config{
		APIVersion:     "config.kplane.io/v1alpha1",
		Kind:           "KplaneConfig",
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"default": {
				Provider:       "kind",
				ClusterName:    "kplane-management",
				Namespace:      "kplane-system",
				KubeconfigPath: filepath.Join(userHomeDir(), ".kube", "config"),
				StackVersion:   "latest",
				CRDSource:      "https://github.com/kplane-dev/controlplane-operator//config/crd?ref=main",
				Images: Images{
					Apiserver: "docker.io/kplanedev/apiserver:v0.0.3",
					Operator:  "docker.io/kplanedev/controlplane-operator:v0.0.3",
					Etcd:      "quay.io/coreos/etcd:v3.5.13",
				},
				Auth: Auth{
					Policy:         "managed",
					IssuerTemplate: "https://{externalHost}",
				},
				Kind: KindOpts{
					NodeImage:   "kindest/node:v1.29.2",
					IngressPort: 8443,
				},
				K3s: K3sOpts{
					Image:       "rancher/k3s:v1.29.2-k3s1",
					IngressPort: 8443,
				},
				UI: UIOpts{
					Enabled: true,
					Color:   true,
				},
			},
		},
	}
}

func ResolvePath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return filepath.Join(xdg, "kplane", defaultConfigFileName), nil
	}
	home := userHomeDir()
	if home == "" {
		return "", errors.New("cannot resolve config path: HOME is not set")
	}
	return filepath.Join(home, ".config", "kplane", defaultConfigFileName), nil
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c Config) ActiveProfile() (Profile, error) {
	p, ok := c.Profiles[c.CurrentProfile]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found in config", c.CurrentProfile)
	}
	return p, nil
}

func userHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return os.Getenv("HOME")
}
