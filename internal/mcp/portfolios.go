// internal/mcp/portfolios.go
package mcp

import (
	"context"
	"fmt"

	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPortfolioTools(server *sdkmcp.Server, c *api.Client) {
	type noInput struct{}

	// list_portfolios
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_portfolios",
		Description: "List all investment portfolios",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ noInput) (*sdkmcp.CallToolResult, any, error) {
		var portfolios []api.Portfolio
		if err := c.Get("/portfolios", &portfolios); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(portfolios), nil, nil
	})

	// get_portfolio
	type getPortfolioInput struct {
		ID string `json:"id" jsonschema:"The portfolio ID"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_portfolio",
		Description: "Get a specific investment portfolio by ID",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getPortfolioInput) (*sdkmcp.CallToolResult, any, error) {
		var portfolio api.Portfolio
		if err := c.Get("/portfolios/"+in.ID, &portfolio); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(portfolio), nil, nil
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
		TotalValue             float64          `json:"totalValue" jsonschema:"Total portfolio value in dollars"`
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
		body := map[string]any{
			"name":        in.Name,
			"institution": in.Institution,
			"owner":       in.Owner,
			"goalId":      in.GoalID,
			"totalValue":  in.TotalValue,
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
		var portfolio api.Portfolio
		if err := c.Post("/portfolios", body, &portfolio); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(portfolio), nil, nil
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
		if in.Name != nil { updates["name"] = *in.Name }
		if in.Description != nil { updates["description"] = *in.Description }
		if in.Institution != nil { updates["institution"] = *in.Institution }
		if in.Owner != nil { updates["owner"] = *in.Owner }
		if in.ManagementType != nil { updates["managementType"] = *in.ManagementType }
		if in.GoalID != nil { updates["goalId"] = *in.GoalID }
		if in.TotalValue != nil { updates["totalValue"] = *in.TotalValue }
		if in.Allocations != nil { updates["allocations"] = in.Allocations }
		if in.Assets != nil { updates["assets"] = in.Assets }
		if in.MinTransactionAmount != nil { updates["minTransactionAmount"] = *in.MinTransactionAmount }
		if in.MinTransactionCurrency != nil { updates["minTransactionCurrency"] = *in.MinTransactionCurrency }
		if len(updates) == 0 {
			return ErrResult(fmt.Errorf("no fields provided to update")), nil, nil
		}
		var portfolio api.Portfolio
		if err := c.MergeAndUpdate("/portfolios/"+in.ID, updates, &portfolio); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(portfolio), nil, nil
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
}
