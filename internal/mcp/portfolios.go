// internal/mcp/portfolios.go
package mcp

import (
	"context"
	"fmt"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
	"github.com/mdmclean/kashmere-cli/internal/trades"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPortfolioTools(server *sdkmcp.Server, c *api.Client) {
	type noInput struct{}

	// list_portfolios
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_portfolios",
		Description: "List all investment portfolios. Each asset includes currentValue, currentPct, and driftPct (where a target is set) computed from live prices.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ noInput) (*sdkmcp.CallToolResult, any, error) {
		var portfolios []api.Portfolio
		if err := c.Get("/portfolios", &portfolios); err != nil {
			return ErrResult(err), nil, nil
		}
		enriched, err := portfolio.EnrichFull(portfolios, c)
		if err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(enriched), nil, nil
	})

	// get_portfolio
	type getPortfolioInput struct {
		ID string `json:"id" jsonschema:"The portfolio ID"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_portfolio",
		Description: "Get a specific investment portfolio by ID. Each asset includes currentValue, currentPct, and driftPct (where a target is set) computed from live prices.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getPortfolioInput) (*sdkmcp.CallToolResult, any, error) {
		var p api.Portfolio
		if err := c.Get("/portfolios/"+in.ID, &p); err != nil {
			return ErrResult(err), nil, nil
		}
		enriched, err := portfolio.EnrichFull([]api.Portfolio{p}, c)
		if err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(enriched[0]), nil, nil
	})

	// list_institutions
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_institutions",
		Description: "List all distinct financial institutions across portfolios",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ noInput) (*sdkmcp.CallToolResult, any, error) {
		var institutions []string
		if err := c.Get("/portfolios/institutions", &institutions); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(institutions), nil, nil
	})

	// create_portfolio
	type createPortfolioInput struct {
		Name                   string           `json:"name" jsonschema:"Portfolio name"`
		Institution            string           `json:"institution" jsonschema:"Financial institution name (e.g. Wealthsimple, Questrade)"`
		Owner                  string           `json:"owner" jsonschema:"Account owner: person1, person2, or joint"`
		GoalID                 string           `json:"goalId" jsonschema:"ID of the goal this portfolio is assigned to"`
		TotalValue             *float64         `json:"totalValue,omitempty" jsonschema:"Total portfolio value in dollars (optional — computed from assets when present)"`
		Allocations            []api.Allocation `json:"allocations" jsonschema:"Target asset allocations, must sum to 100"`
		Description            *string          `json:"description,omitempty" jsonschema:"Optional portfolio description"`
		ManagementType         *string          `json:"managementType,omitempty" jsonschema:"Portfolio management type: self (default) or auto"`
		Assets                 []api.Asset      `json:"assets,omitempty" jsonschema:"Optional individual asset holdings"`
		MinTransactionAmount   *float64         `json:"minTransactionAmount,omitempty" jsonschema:"Optional minimum transaction amount"`
		MinTransactionCurrency *string          `json:"minTransactionCurrency,omitempty" jsonschema:"Optional min transaction currency: CAD or USD"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_portfolio",
		Description: "Create a new investment portfolio",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in createPortfolioInput) (*sdkmcp.CallToolResult, any, error) {
		totalValue := 0.0
		if in.TotalValue != nil {
			totalValue = *in.TotalValue
		}
		body := map[string]any{
			"name":        in.Name,
			"institution": in.Institution,
			"owner":       in.Owner,
			"goalId":      in.GoalID,
			"totalValue":  totalValue,
			"allocations": in.Allocations,
		}
		if in.Description != nil {
			body["description"] = *in.Description
		}
		if in.ManagementType != nil {
			body["managementType"] = *in.ManagementType
		}
		if in.Assets != nil {
			body["assets"] = in.Assets
		}
		if in.MinTransactionAmount != nil {
			body["minTransactionAmount"] = *in.MinTransactionAmount
		}
		if in.MinTransactionCurrency != nil {
			body["minTransactionCurrency"] = *in.MinTransactionCurrency
		}
		validateP := api.Portfolio{
			Allocations: in.Allocations,
			Assets:      in.Assets,
		}
		if err := portfolio.Validate(validateP); err != nil {
			return ErrResult(err), nil, nil
		}

		var created api.Portfolio
		if err := c.Post("/portfolios", body, &created); err != nil {
			return ErrResult(err), nil, nil
		}
		enriched, err := portfolio.Enrich([]api.Portfolio{created}, c)
		if err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(enriched[0]), nil, nil
	})

	// update_portfolio
	type updatePortfolioInput struct {
		ID                     string           `json:"id" jsonschema:"The portfolio ID to update"`
		Name                   *string          `json:"name,omitempty" jsonschema:"New portfolio name"`
		Description            *string          `json:"description,omitempty" jsonschema:"New description"`
		Institution            *string          `json:"institution,omitempty" jsonschema:"New financial institution name"`
		Owner                  *string          `json:"owner,omitempty" jsonschema:"New owner: person1, person2, or joint"`
		ManagementType         *string          `json:"managementType,omitempty" jsonschema:"New management type: self or auto"`
		GoalID                 *string          `json:"goalId,omitempty" jsonschema:"New goal ID"`
		TotalValue             *float64         `json:"totalValue,omitempty" jsonschema:"New total portfolio value"`
		Allocations            []api.Allocation `json:"allocations,omitempty" jsonschema:"New target asset allocations, must sum to 100"`
		Assets                 []api.Asset      `json:"assets,omitempty" jsonschema:"New asset holdings"`
		MinTransactionAmount   *float64         `json:"minTransactionAmount,omitempty" jsonschema:"New minimum transaction amount"`
		MinTransactionCurrency *string          `json:"minTransactionCurrency,omitempty" jsonschema:"New min transaction currency: CAD or USD"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_portfolio",
		Description: "Update an existing investment portfolio. Only provided fields are updated (fetch-merge-put for E2EE).",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in updatePortfolioInput) (*sdkmcp.CallToolResult, any, error) {
		updates := map[string]any{}
		if in.Name != nil {
			updates["name"] = *in.Name
		}
		if in.Description != nil {
			updates["description"] = *in.Description
		}
		if in.Institution != nil {
			updates["institution"] = *in.Institution
		}
		if in.Owner != nil {
			updates["owner"] = *in.Owner
		}
		if in.ManagementType != nil {
			updates["managementType"] = *in.ManagementType
		}
		if in.GoalID != nil {
			updates["goalId"] = *in.GoalID
		}
		if in.TotalValue != nil {
			updates["totalValue"] = *in.TotalValue
		}
		if in.Allocations != nil {
			updates["allocations"] = in.Allocations
		}
		if in.Assets != nil {
			updates["assets"] = in.Assets
		}
		if in.MinTransactionAmount != nil {
			updates["minTransactionAmount"] = *in.MinTransactionAmount
		}
		if in.MinTransactionCurrency != nil {
			updates["minTransactionCurrency"] = *in.MinTransactionCurrency
		}
		if len(updates) == 0 {
			return ErrResult(fmt.Errorf("no fields provided to update")), nil, nil
		}
		path := "/portfolios/" + in.ID

		// Validate merged state if allocation-sensitive fields are changing.
		if in.Allocations != nil || in.Assets != nil {
			var current api.Portfolio
			if err := c.Get(path, &current); err != nil {
				return ErrResult(err), nil, nil
			}
			if in.Allocations != nil {
				current.Allocations = in.Allocations
			}
			if in.Assets != nil {
				current.Assets = in.Assets
			}
			if err := portfolio.Validate(current); err != nil {
				return ErrResult(err), nil, nil
			}
		}

		var updated api.Portfolio
		if err := c.MergeAndUpdate(path, updates, &updated); err != nil {
			return ErrResult(err), nil, nil
		}
		enriched, err := portfolio.Enrich([]api.Portfolio{updated}, c)
		if err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(enriched[0]), nil, nil
	})

	// delete_portfolio
	type deletePortfolioInput struct {
		ID string `json:"id" jsonschema:"The portfolio ID to delete"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_portfolio",
		Description: "Delete an investment portfolio",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in deletePortfolioInput) (*sdkmcp.CallToolResult, any, error) {
		if err := c.Delete("/portfolios/" + in.ID); err != nil {
			return ErrResult(err), nil, nil
		}
		return TextResult("Portfolio deleted successfully."), nil, nil
	})

	// get_top_trades
	type getTopTradesInput struct {
		PortfolioID *string `json:"portfolioId,omitempty" jsonschema:"Optional portfolio ID; omit to rank across all portfolios"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "get_top_trades",
		Description: `Get ranked BUY/SELL trade recommendations based on how far each asset has drifted from its target allocation.

Response shape:
- trades: ranked actionable BUY/SELL recommendations
- suppressedBuys: underweight positions that cannot be acted on due to insufficient cash

Each suppressedBuy includes ticker, assetName, driftPct, driftAmount, and reason:
- no_cash: portfolio holds no cash
- below_min_transaction: available cash is less than the minimum transaction amount
- cash_needs_replenishing: portfolio cash is below its own target weight

BUYs are suppressed when:
- The portfolio holds no cash (nothing to deploy)
- Available cash is below the portfolio's minimum transaction amount
- The portfolio's cash is itself below its target weight (it needs replenishing before other assets are bought)

When cash is present but insufficient for all BUYs, the highest-drift BUYs are funded first. A BUY may appear with a reduced amount (isPartialBuy=true) if cash only partially covers it; lower-priority BUYs are moved to suppressedBuys.

SELLs always appear in trades regardless of cash.`,
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getTopTradesInput) (*sdkmcp.CallToolResult, any, error) {
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
		result, err := trades.Compute(portfolios, c)
		if err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(result), nil, nil
	})
}
