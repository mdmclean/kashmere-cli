// internal/config/config.go
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey     string `json:"apiKey"`
	Salt       string `json:"salt"`
	APIBaseURL string `json:"apiBaseUrl"`
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kashemere", "config.json")
}

func Load() (*Config, error) {
	path := os.Getenv("KASHEMERE_CONFIG")
	if path == "" {
		path = DefaultPath()
	}
	return LoadFrom(path)
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("config not found — run 'kashemere setup' first")
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path := os.Getenv("KASHEMERE_CONFIG")
	if path == "" {
		path = DefaultPath()
	}
	return SaveTo(cfg, path)
}

func SaveTo(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
