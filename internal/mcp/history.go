// internal/mcp/history.go
package mcp

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerHistoryTools(server *sdkmcp.Server, c *api.Client) {
	// list_snapshots
	type listSnapshotsInput struct {
		PortfolioID *string `json:"portfolioId,omitempty" jsonschema:"Optional portfolio ID to filter snapshots (filtered client-side — server cannot filter encrypted docs by portfolioId)"`
		StartDate   *string `json:"startDate,omitempty" jsonschema:"Optional start date (YYYY-MM-DD); filtered server-side"`
		EndDate     *string `json:"endDate,omitempty" jsonschema:"Optional end date (YYYY-MM-DD); filtered server-side"`
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
		if err := c.GetSnapshots(path, &snapshots); err != nil {
			return ErrResult(err), nil, nil
		}
		if in.PortfolioID != nil {
			filtered := make([]api.PortfolioSnapshot, 0, len(snapshots))
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
		Description: "Create a portfolio value snapshot using live prices. If portfolioId is omitted, creates snapshots for all portfolios.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in createSnapshotInput) (*sdkmcp.CallToolResult, any, error) {
		// Fetch the relevant portfolios
		var portfolios []api.Portfolio
		if in.PortfolioID != nil && *in.PortfolioID != "" {
			var p api.Portfolio
			if err := c.Get("/portfolios/"+*in.PortfolioID, &p); err != nil {
				return ErrResult(err), nil, nil
			}
			portfolios = []api.Portfolio{p}
		} else {
			if err := c.Get("/portfolios", &portfolios); err != nil {
				return ErrResult(err), nil, nil
			}
		}

		// Enrich with live prices to compute current totalValue
		enriched, err := portfolio.Enrich(portfolios, c)
		if err != nil {
			return ErrResult(err), nil, nil
		}

		timestamp := time.Now().UTC().Format(time.RFC3339)
		type snapshotResult struct {
			ID          string  `json:"id"`
			PortfolioID string  `json:"portfolioId"`
			Timestamp   string  `json:"timestamp"`
			TotalValue  float64 `json:"totalValue"`
		}
		var created []snapshotResult
		for _, p := range enriched {
			if err := c.PostSnapshot(p.ID, timestamp, p.TotalValue); err != nil {
				return ErrResult(fmt.Errorf("snapshot for %q: %w", p.Name, err)), nil, nil
			}
			dateStr := timestamp[:10]
			created = append(created, snapshotResult{
				ID:          "snapshot-" + p.ID + "-" + dateStr,
				PortfolioID: p.ID,
				Timestamp:   timestamp,
				TotalValue:  p.TotalValue,
			})
		}

		if len(created) == 1 {
			return JSONResult(created[0]), nil, nil
		}
		return JSONResult(created), nil, nil
	})
}
