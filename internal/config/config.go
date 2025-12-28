package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/varoOP/shinkrodb/internal/domain"
)

// Load loads configuration from multiple sources:
// 1. Config file (config.yaml or .shinkrodb.yaml)
// 2. Environment variables (SHINKRODB_*)
// 3. secrets.json (for backward compatibility)
func Load() (*domain.Config, error) {
	cfg := &domain.Config{}

	// Load from Viper (config file + env vars)
	cfg.MalClientID = viper.GetString("mal_client_id")
	cfg.TmdbApiKey = viper.GetString("tmdb_api_key")

	// Fallback to secrets.json for backward compatibility
	// This allows existing setups to continue working
	if cfg.MalClientID == "" || cfg.TmdbApiKey == "" {
		secretsCfg, err := loadSecretsJSON()
		if err != nil {
			// If secrets.json doesn't exist and config is empty, return error
			if cfg.MalClientID == "" || cfg.TmdbApiKey == "" {
				return nil, fmt.Errorf("missing required configuration: mal_client_id and tmdb_api_key must be set via config file, environment variables, or secrets.json")
			}
		} else {
			// Use secrets.json values if not set in config/env
			if cfg.MalClientID == "" {
				cfg.MalClientID = secretsCfg.MalClientID
			}
			if cfg.TmdbApiKey == "" {
				cfg.TmdbApiKey = secretsCfg.TmdbApiKey
			}
		}
	}

	// Validate required fields
	if cfg.MalClientID == "" {
		return nil, fmt.Errorf("mal_client_id is required")
	}
	if cfg.TmdbApiKey == "" {
		return nil, fmt.Errorf("tmdb_api_key is required")
	}

	return cfg, nil
}

// loadSecretsJSON loads configuration from secrets.json for backward compatibility
func loadSecretsJSON() (*domain.Config, error) {
	// Try common locations
	paths := []string{
		"./secrets.json",
		"$HOME/.shinkrodb/secrets.json",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return loadSecretsFromFile(path)
		}
	}

	return nil, fmt.Errorf("secrets.json not found")
}

// loadSecretsFromFile loads secrets from a specific file
func loadSecretsFromFile(path string) (*domain.Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("json")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read secrets.json: %w", err)
	}

	cfg := &domain.Config{
		MalClientID: viper.GetString("mal-client-id"),
		TmdbApiKey:   viper.GetString("tmdb-api-key"),
	}

	return cfg, nil
}

// NewConfig is kept for backward compatibility but is deprecated
// Use Load() instead
func NewConfig() *domain.Config {
	cfg, err := Load()
	if err != nil {
		// For backward compatibility, we'll try to continue
		// but this should be migrated to proper error handling
		return &domain.Config{}
	}
	return cfg
}
