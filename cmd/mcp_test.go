package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdmclean/kashmere-cli/internal/config"
)

func TestMCPRequiresPassphrase(t *testing.T) {
	// Write a valid config to a temp dir and point KASHMERE_CONFIG at it.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	cfg := &config.Config{
		APIKey:     "fp_testkey",
		Salt:       "dGVzdHNhbHQ=", // base64("testsalt") — valid for SaltFromBase64
		APIBaseURL: "https://kashmere.app/api/v1",
	}
	if err := config.SaveTo(cfg, cfgPath); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	t.Setenv("KASHMERE_CONFIG", cfgPath)

	// Ensure KASHMERE_PASSPHRASE is unset for this test.
	old, hadOld := os.LookupEnv("KASHMERE_PASSPHRASE")
	os.Unsetenv("KASHMERE_PASSPHRASE")
	if hadOld {
		t.Cleanup(func() { os.Setenv("KASHMERE_PASSPHRASE", old) })
	}

	_, _, err := loadMCPConfig()
	if err == nil {
		t.Fatal("expected error when KASHMERE_PASSPHRASE is unset, got nil")
	}

	want := "KASHMERE_PASSPHRASE environment variable is not set"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
}
