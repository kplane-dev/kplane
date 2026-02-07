package cli

import (
	"fmt"

	"github.com/kplane-dev/kplane/internal/config"
)

func saveConfig(cfg config.Config) error {
	path, err := config.ResolvePath(cfgPath)
	if err != nil {
		return err
	}
	return config.Save(path, cfg)
}

func markUICompletion(cfg config.Config, markUp, markCreate bool) error {
	profile, ok := cfg.Profiles[cfg.CurrentProfile]
	if !ok {
		return fmt.Errorf("profile %q not found in config", cfg.CurrentProfile)
	}
	if markUp {
		profile.UI.UpHintCount++
	}
	if markCreate {
		profile.UI.CreateHintCount++
	}
	cfg.Profiles[cfg.CurrentProfile] = profile
	return saveConfig(cfg)
}

func setProfileProvider(cfg config.Config, provider string) error {
	profile, ok := cfg.Profiles[cfg.CurrentProfile]
	if !ok {
		return fmt.Errorf("profile %q not found in config", cfg.CurrentProfile)
	}
	if provider == "" || profile.Provider == provider {
		return nil
	}
	profile.Provider = provider
	cfg.Profiles[cfg.CurrentProfile] = profile
	return saveConfig(cfg)
}
