// internal/mcp/dashboard.go
package mcp

import (
	"context"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/dashboard"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerDashboardTools(server *sdkmcp.Server, c *api.Client) {
	type noInput struct{}

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_dashboard",
		Description: "Get aggregated dashboard: total portfolio value, weighted asset allocations, goal summaries, and net worth. Computed client-side from portfolios, goals, and mortgages.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ noInput) (*sdkmcp.CallToolResult, any, error) {
		var portfolios []api.Portfolio
		if err := c.Get("/portfolios", &portfolios); err != nil {
			return ErrResult(err), nil, nil
		}
		enriched, err := portfolio.Enrich(portfolios, c)
		if err != nil {
			return ErrResult(err), nil, nil
		}
		var goals []api.Goal
		if err := c.Get("/goals", &goals); err != nil {
			return ErrResult(err), nil, nil
		}
		var mortgages []api.Mortgage
		c.Get("/mortgages", &mortgages) // optional — ignore error

		return JSONResult(dashboard.Compute(enriched, goals, mortgages)), nil, nil
	})
}
