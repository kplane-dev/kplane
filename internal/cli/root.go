package cli

import (
	"fmt"
	"os"

	"github.com/kplane-dev/kplane/internal/config"
	"github.com/spf13/cobra"
)

var cfgPath string

func Execute() error {
	root := &cobra.Command{
		Use:   "kplane",
		Short: "kplane CLI",
	}

	root.PersistentFlags().StringVar(&cfgPath, "config", "", "Path to config file")

	root.AddCommand(
		newUpCommand(),
		newDownCommand(),
		newCreateCommand(),
		newCreateClusterAliasCommand(),
		newConfigCommand(),
		newGetCommand(),
		newGetCredentialsCommand(),
		newDoctorCommand(),
	)

	return root.Execute()
}

func loadConfig() (config.Config, error) {
	path, err := config.ResolvePath(cfgPath)
	if err != nil {
		return config.Config{}, err
	}

	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, err
	}

	return cfg, nil
}

func mustConfig() config.Config {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	return cfg
}
