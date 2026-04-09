package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kashemere/kashemere-cli/internal/config"
	"github.com/kashemere/kashemere-cli/internal/crypto"
	"github.com/spf13/cobra"
)

var prettyFlag bool
var cfg *config.Config
var encKey []byte

var rootCmd = &cobra.Command{
	Use:   "kashemere",
	Short: "Kashemere CLI — manage your finances from the terminal or as an agent",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip auth for setup command
		if cmd.Name() == "setup" {
			return nil
		}

		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("run 'kashemere setup' first: %w", err)
		}

		passphrase := os.Getenv("KASHEMERE_PASSPHRASE")
		if passphrase == "" {
			fmt.Fprint(os.Stderr, "Encryption passphrase: ")
			pass, err := readPassword()
			if err != nil {
				return fmt.Errorf("reading passphrase: %w", err)
			}
			passphrase = pass
		}

		salt, err := crypto.SaltFromBase64(cfg.Salt)
		if err != nil {
			return fmt.Errorf("invalid salt in config: %w", err)
		}
		encKey, err = crypto.DeriveKey(passphrase, salt)
		if err != nil {
			return fmt.Errorf("deriving encryption key: %w", err)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func SetVersion(v string) {
	rootCmd.Version = v
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&prettyFlag, "pretty", false, "Pretty-print JSON output")
}

func outputJSON(v any) {
	var data []byte
	var err error
	if prettyFlag {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		outputError(fmt.Errorf("marshaling output: %w", err), 0)
		return
	}
	fmt.Println(string(data))
}

func outputError(err error, status int) {
	data, _ := json.Marshal(map[string]any{
		"error":  err.Error(),
		"status": status,
	})
	fmt.Fprintln(os.Stderr, string(data))
	os.Exit(1)
}

// readPassword reads a password from stdin without echoing (Unix).
// Falls back to plain readline on Windows.
func readPassword() (string, error) {
	return readPasswordFromTerminal()
}
