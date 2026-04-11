// internal/mcp/cashflows.go
package mcp

import (
	"context"

	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerCashFlowTools(server *sdkmcp.Server, c *api.Client) {
	// list_cashflows
	type listCashFlowsInput struct {
		PortfolioID *string `json:"portfolioId,omitempty" jsonschema:"Optional portfolio ID to filter cash flows (client-side with E2EE)"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_cashflows",
		Description: "List all cash flows (deposits and withdrawals). Optionally filter by portfolio ID.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in listCashFlowsInput) (*sdkmcp.CallToolResult, any, error) {
		var cashflows []api.CashFlow
		if err := c.Get("/cashflows", &cashflows); err != nil {
			return ErrResult(err), nil, nil
		}
		if in.PortfolioID != nil {
			filtered := cashflows[:0]
			for _, cf := range cashflows {
				if cf.PortfolioID == *in.PortfolioID {
					filtered = append(filtered, cf)
				}
			}
			cashflows = filtered
		}
		return JSONResult(cashflows), nil, nil
	})

	// get_cashflow
	type getCashFlowInput struct {
		ID string `json:"id" jsonschema:"The cash flow ID"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_cashflow",
		Description: "Get a specific cash flow by ID",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getCashFlowInput) (*sdkmcp.CallToolResult, any, error) {
		var cf api.CashFlow
		if err := c.Get("/cashflows/"+in.ID, &cf); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(cf), nil, nil
	})

	// create_cashflow
	type createCashFlowInput struct {
		PortfolioID string  `json:"portfolioId" jsonschema:"The portfolio ID"`
		Type        string  `json:"type" jsonschema:"Type of cash flow: deposit or withdrawal"`
		Amount      float64 `json:"amount" jsonschema:"Amount in dollars (positive)"`
		Date        string  `json:"date" jsonschema:"Date of the cash flow (YYYY-MM-DD)"`
		CashAssetID *string `json:"cashAssetId,omitempty" jsonschema:"Optional cash asset ID to credit/debit"`
		Description *string `json:"description,omitempty" jsonschema:"Optional description"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_cashflow",
		Description: "Create a new cash flow (deposit or withdrawal) for a portfolio",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in createCashFlowInput) (*sdkmcp.CallToolResult, any, error) {
		body := map[string]any{
			"portfolioId": in.PortfolioID,
			"type":        in.Type,
			"amount":      in.Amount,
			"date":        in.Date,
		}
		if in.CashAssetID != nil {
			body["cashAssetId"] = *in.CashAssetID
		}
		if in.Description != nil {
			body["description"] = *in.Description
		}
		var cf api.CashFlow
		if err := c.Post("/cashflows", body, &cf); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(cf), nil, nil
	})

	// update_cashflow
	type updateCashFlowInput struct {
		ID          string   `json:"id" jsonschema:"The cash flow ID to update"`
		Type        *string  `json:"type,omitempty" jsonschema:"New type: deposit or withdrawal"`
		Amount      *float64 `json:"amount,omitempty" jsonschema:"New amount in dollars"`
		Date        *string  `json:"date,omitempty" jsonschema:"New date (YYYY-MM-DD)"`
		Description *string  `json:"description,omitempty" jsonschema:"New description"`
		PortfolioID *string  `json:"portfolioId,omitempty" jsonschema:"New portfolio ID"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_cashflow",
		Description: "Update an existing cash flow. Only provided fields are updated.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in updateCashFlowInput) (*sdkmcp.CallToolResult, any, error) {
		updates := map[string]any{}
		if in.Type != nil { updates["type"] = *in.Type }
		if in.Amount != nil { updates["amount"] = *in.Amount }
		if in.Date != nil { updates["date"] = *in.Date }
		if in.Description != nil { updates["description"] = *in.Description }
		if in.PortfolioID != nil { updates["portfolioId"] = *in.PortfolioID }
		var cf api.CashFlow
		if err := c.MergeAndUpdate("/cashflows/"+in.ID, updates, &cf); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(cf), nil, nil
	})

	// delete_cashflow
	type deleteCashFlowInput struct {
		ID string `json:"id" jsonschema:"The cash flow ID to delete"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_cashflow",
		Description: "Delete a cash flow. Reverses the portfolio cash balance change. If part of a transfer, both sides are deleted.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in deleteCashFlowInput) (*sdkmcp.CallToolResult, any, error) {
		if err := c.Delete("/cashflows/" + in.ID); err != nil {
			return ErrResult(err), nil, nil
		}
		return TextResult("Cash flow deleted successfully."), nil, nil
	})
}
