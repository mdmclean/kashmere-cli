// internal/mcp/server.go
package mcp

import (
	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates and returns a configured MCP server with all Kashmere tools registered.
func NewServer(c *api.Client) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "kashmere",
		Version: "1.0.0",
	}, nil)

	registerPortfolioTools(server, c)
	registerGoalTools(server, c)
	registerCashFlowTools(server, c)
	registerMortgageTools(server, c)
	registerPriceTools(server, c)
	registerHistoryTools(server, c)
	registerSettingsTools(server, c)
	registerDashboardTools(server, c)

	return server
}
