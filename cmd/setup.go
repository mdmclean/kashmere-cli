// cmd/setup.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/auth"
	"github.com/mdmclean/kashmere-cli/internal/config"
	"github.com/mdmclean/kashmere-cli/internal/crypto"
	"github.com/spf13/cobra"
)

var (
	setupEmail   string
	setupForce   bool
	setupAPIBase string
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Authenticate and store credentials in ~/.kashemere/config.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		if setupAPIBase == "" {
			setupAPIBase = os.Getenv("KASHEMERE_API_BASE_URL")
		}
		if setupAPIBase == "" {
			setupAPIBase = "https://api.kashemere.app/api/v1"
		}

		// Check for existing config
		if !setupForce {
			if _, err := config.Load(); err == nil {
				fmt.Fprint(os.Stderr, "Config already exists. Overwrite? [y/N]: ")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}
		}

		var jwt string
		var err error

		if setupEmail != "" {
			jwt, err = loginWithEmail(setupAPIBase, setupEmail)
		} else {
			jwt, err = loginWithBrowser(setupAPIBase)
		}
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		if jwt == "" {
			return fmt.Errorf("no token received")
		}

		// Create API key using the JWT
		unauthClient := api.New(setupAPIBase, "", nil)
		unauthClient.SetBearerToken(jwt)

		hostname, _ := os.Hostname()
		keyName := "agent-" + hostname

		var keyResp struct {
			Key string `json:"key"`
		}
		if err := unauthClient.Post("/auth/api-keys", map[string]string{"name": keyName}, &keyResp); err != nil {
			return fmt.Errorf("creating API key: %w", err)
		}

		// Fetch encryption salt
		apiClient := api.New(setupAPIBase, keyResp.Key, nil)
		var saltResp struct {
			Salt string `json:"salt"`
		}
		if err := apiClient.Get("/auth/encryption-salt", &saltResp); err != nil {
			return fmt.Errorf("fetching encryption salt: %w", err)
		}

		// Verify passphrase
		passphrase := os.Getenv("KASHEMERE_PASSPHRASE")
		if passphrase == "" {
			fmt.Fprint(os.Stderr, "Encryption passphrase (same as web app): ")
			p, err := readPassword()
			if err != nil {
				return err
			}
			passphrase = p
		}

		// Verify the passphrase works by attempting key derivation (length check)
		if len(passphrase) < 12 {
			return fmt.Errorf("passphrase must be at least 12 characters")
		}
		salt, err := crypto.SaltFromBase64(saltResp.Salt)
		if err != nil {
			return fmt.Errorf("invalid salt from server: %w", err)
		}
		if _, err := crypto.DeriveKey(passphrase, salt); err != nil {
			return fmt.Errorf("deriving key: %w", err)
		}

		cfg := &config.Config{
			APIKey:     keyResp.Key,
			Salt:       saltResp.Salt,
			APIBaseURL: setupAPIBase,
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		outputJSON(map[string]string{
			"status":     "ok",
			"configPath": config.DefaultPath(),
			"apiKeyName": keyName,
		})
		return nil
	},
}

func init() {
	setupCmd.Flags().StringVar(&setupEmail, "email", "", "Email address for email/password login (skips browser)")
	setupCmd.Flags().BoolVar(&setupForce, "force", false, "Overwrite existing config without prompting")
	setupCmd.Flags().StringVar(&setupAPIBase, "api-base", "", "API base URL (default: https://api.kashemere.app/api/v1)")
	rootCmd.AddCommand(setupCmd)
}

func loginWithEmail(apiBase, email string) (string, error) {
	fmt.Fprint(os.Stderr, "Password: ")
	password, err := readPassword()
	if err != nil {
		return "", err
	}

	unauthClient := api.New(apiBase, "", nil)
	var authResp struct {
		Token string `json:"token"`
	}
	if err := unauthClient.Post("/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, &authResp); err != nil {
		return "", err
	}
	return authResp.Token, nil
}

func loginWithBrowser(apiBase string) (string, error) {
	port, tokenCh, err := auth.WaitForCallback(5 * time.Minute)
	if err != nil {
		return "", err
	}

	callbackURL := fmt.Sprintf("http://localhost:%d/callback", port)
	appURL := strings.Replace(apiBase, "/api/v1", "", 1)
	loginURL := fmt.Sprintf("%s/auth/cli?callback=%s", appURL, callbackURL)

	fmt.Fprintf(os.Stderr, "\nOpening browser to: %s\n", loginURL)
	fmt.Fprintln(os.Stderr, "If the browser does not open, visit the URL above manually.")

	openBrowser(loginURL)
	fmt.Fprintln(os.Stderr, "Waiting for authentication (5 minute timeout)...")

	token := <-tokenCh
	if token == "" {
		return "", fmt.Errorf("authentication timed out")
	}
	return token, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
