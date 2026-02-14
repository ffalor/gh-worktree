package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	WorktreeBase string `mapstructure:"worktree_base"`
}

// Default values
const (
	DefaultWorktreeBase = "~/github/worktree"
	ConfigName          = "config"
	ConfigType          = "yaml"
)

// Init initializes Viper configuration
func Init() error {
	// Set default values
	viper.SetDefault("worktree_base", expandHome(DefaultWorktreeBase))

	// Set config file details
	viper.SetConfigName(ConfigName)
	viper.SetConfigType(ConfigType)

	// Set config search path
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "gh-worktree")
	viper.AddConfigPath(configDir)

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set env prefix for environment variables
	viper.SetEnvPrefix("GH_WORKTREE")
	viper.AutomaticEnv()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config: %w", err)
		}
	}

	return nil
}

// GetWorktreeBase returns the worktree base directory
func GetWorktreeBase() string {
	return expandHome(viper.GetString("worktree_base"))
}

// SetWorktreeBase sets the worktree base directory
func SetWorktreeBase(path string) {
	viper.Set("worktree_base", path)
}

// WriteConfig writes the current configuration to file
func WriteConfig() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "gh-worktree")
	configFile := filepath.Join(configDir, ConfigName+"."+ConfigType)

	return viper.WriteConfigAs(configFile)
}

// expandHome expands ~ to home directory
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
