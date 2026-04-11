// internal/mcp/history.go
package mcp

import (
	"context"
	"net/url"

	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerHistoryTools(server *sdkmcp.Server, c *api.Client) {
	// list_snapshots
	type listSnapshotsInput struct {
		PortfolioID *string `json:"portfolioId,omitempty" jsonschema:"Optional portfolio ID to filter snapshots (client-side with E2EE)"`
		StartDate   *string `json:"startDate,omitempty" jsonschema:"Optional start date (YYYY-MM-DD)"`
		EndDate     *string `json:"endDate,omitempty" jsonschema:"Optional end date (YYYY-MM-DD)"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_snapshots",
		Description: "List portfolio snapshots. Optionally filter by portfolio ID and/or date range.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in listSnapshotsInput) (*sdkmcp.CallToolResult, any, error) {
		params := url.Values{}
		if in.StartDate != nil {
			params.Set("startDate", *in.StartDate)
		}
		if in.EndDate != nil {
			params.Set("endDate", *in.EndDate)
		}
		path := "/history/snapshots"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
		var snapshots []api.PortfolioSnapshot
		if err := c.Get(path, &snapshots); err != nil {
			return ErrResult(err), nil, nil
		}
		if in.PortfolioID != nil {
			filtered := snapshots[:0]
			for _, s := range snapshots {
				if s.PortfolioID == *in.PortfolioID {
					filtered = append(filtered, s)
				}
			}
			snapshots = filtered
		}
		return JSONResult(snapshots), nil, nil
	})

	// create_snapshot
	type createSnapshotInput struct {
		PortfolioID *string `json:"portfolioId,omitempty" jsonschema:"Optional portfolio ID. If omitted, creates snapshots for all portfolios."`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_snapshot",
		Description: "Create a portfolio snapshot. If portfolioId is omitted, creates snapshots for all portfolios.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in createSnapshotInput) (*sdkmcp.CallToolResult, any, error) {
		body := map[string]any{}
		if in.PortfolioID != nil {
			body["portfolioId"] = *in.PortfolioID
		}
		var result any
		if err := c.Post("/history/snapshots", body, &result); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(result), nil, nil
	})
}
