// internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mdmclean/kashmere-cli/internal/config"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &config.Config{
		APIKey:     "fp_abc123",
		Salt:       "dGVzdHNhbHQ=",
		APIBaseURL: "https://kashmere.app/api/v1",
	}

	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	loaded, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if loaded.APIKey != cfg.APIKey {
		t.Errorf("APIKey: got %q, want %q", loaded.APIKey, cfg.APIKey)
	}
	if loaded.Salt != cfg.Salt {
		t.Errorf("Salt: got %q, want %q", loaded.Salt, cfg.Salt)
	}
	if loaded.APIBaseURL != cfg.APIBaseURL {
		t.Errorf("APIBaseURL: got %q, want %q", loaded.APIBaseURL, cfg.APIBaseURL)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".kashmere", "config.json")
	if got := config.DefaultPath(); got != want {
		t.Errorf("DefaultPath: got %q, want %q", got, want)
	}
}
