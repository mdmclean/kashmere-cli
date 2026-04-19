// cmd/mcp.go
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/config"
	"github.com/mdmclean/kashmere-cli/internal/crypto"
	internalmcp "github.com/mdmclean/kashmere-cli/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start an MCP server on stdio for use with Claude Desktop and other MCP clients",
	// Use RunE directly — bypass root PersistentPreRunE which prompts for passphrase interactively.
	RunE: func(cmd *cobra.Command, args []string) error {
		_, apiClient, err := loadMCPConfig()
		if err != nil {
			return err
		}
		server := internalmcp.NewServer(apiClient)
		if err := server.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
			return fmt.Errorf("mcp server error: %w", err)
		}
		return nil
	},
}

func loadMCPConfig() ([]byte, *api.Client, error) {
	apiKey := os.Getenv("KASHMERE_API_KEY")
	salt := os.Getenv("KASHMERE_SALT")
	apiBaseURL := os.Getenv("KASHMERE_API_BASE_URL")

	if apiKey == "" || salt == "" {
		cfg, err := config.Load()
		if err != nil {
			return nil, nil, fmt.Errorf("run 'kashmere setup' first: %w", err)
		}
		if apiKey == "" {
			apiKey = cfg.APIKey
		}
		if salt == "" {
			salt = cfg.Salt
		}
		if apiBaseURL == "" {
			apiBaseURL = cfg.APIBaseURL
		}
	}

	if apiBaseURL == "" {
		apiBaseURL = "https://kashmere.app/api/v1"
	}

	passphrase := os.Getenv("KASHMERE_PASSPHRASE")
	if passphrase == "" {
		return nil, nil, fmt.Errorf(
			"KASHMERE_PASSPHRASE environment variable is not set\n" +
				"Set it in your Claude Desktop config under env: { \"KASHMERE_PASSPHRASE\": \"...\" }",
		)
	}

	saltBytes, err := crypto.SaltFromBase64(salt)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid salt: %w", err)
	}
	encKey, err := crypto.DeriveKey(passphrase, saltBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("deriving encryption key: %w", err)
	}

	client := api.New(apiBaseURL, apiKey, encKey)
	return encKey, client, nil
}

func init() {
	// Do NOT add mcpCmd to rootCmd's PersistentPreRunE chain.
	// The root command prompts for a passphrase interactively; mcp must not.
	rootCmd.AddCommand(mcpCmd)
}
