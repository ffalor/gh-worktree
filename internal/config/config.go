package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	WorktreeBase string `mapstructure:"worktree_dir"`
}

// Default values
const (
	DefaultWorktreeBase = "~/github/worktree"
	ConfigName          = "config"
	ConfigType          = "yaml"
)

var v *viper.Viper

// Load initializes Viper and reads the configuration.
// It returns the loaded Viper instance and handles file-not-found gracefully.
func Load() (*viper.Viper, error) {
	v = viper.New()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "gh-worktree")

	v.AddConfigPath(configDir)

	v.SetConfigName(ConfigName)
	v.SetConfigType(ConfigType)

	v.AutomaticEnv()
	v.SetEnvPrefix("GH_WT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Sensible defaults
	v.SetDefault("worktree_dir", filepath.Join(home, "github", "worktree"))

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return v, nil
}

// Save persists the current Viper state to the config file.
// Creates directories and file if needed.
func Save() error {
	if v == nil {
		return errors.New("config not initialized; call Load first")
	}

	configFile := v.ConfigFileUsed()
	if configFile == "" {
		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".config", "gh-worktree")
		configFile = filepath.Join(configDir, "config.yaml")

		if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
			return fmt.Errorf("cannot create config directory: %w", err)
		}
	}

	// SafeWriteConfigAs creates if missing, refuses to overwrite unexpectedly
	if err := v.SafeWriteConfigAs(configFile); err != nil {
		if os.IsNotExist(err) {
			return v.WriteConfigAs(configFile)
		}
		return fmt.Errorf("failed to write config to %s: %w", configFile, err)
	}

	return nil
}

// Get returns the unmarshaled configuration struct.
// Prefer this over direct viper.Get* calls for type safety.
func Get() (Config, error) {
	if v == nil {
		return Config{}, errors.New("config not initialized")
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("cannot unmarshal config: %w", err)
	}

	// Expand tilde in WorktreeBase if present
	if strings.HasPrefix(cfg.WorktreeBase, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("cannot determine home directory: %w", err)
		}
		cfg.WorktreeBase = filepath.Join(home, cfg.WorktreeBase[2:])
	}

	return cfg, nil
}

// Set updates a value in the Viper store (in memory only).
// Call Save() afterward to persist changes.
func Set(key string, value any) {
	if v != nil {
		v.Set(key, value)
	}
}

// ConfigFileUsed returns the path of the loaded config file (or "" if none).
func ConfigFileUsed() string {
	if v != nil {
		return v.ConfigFileUsed()
	}
	return ""
}
