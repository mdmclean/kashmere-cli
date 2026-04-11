// internal/mcp/mortgages.go
package mcp

import (
	"context"

	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerMortgageTools(server *sdkmcp.Server, c *api.Client) {
	// list_mortgages
	type listMortgagesInput struct {
		Owner *string `json:"owner,omitempty" jsonschema:"Optional owner filter: person1, person2, or joint (client-side with E2EE)"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_mortgages",
		Description: "List all mortgages. Optionally filter by owner.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in listMortgagesInput) (*sdkmcp.CallToolResult, any, error) {
		var mortgages []api.Mortgage
		if err := c.Get("/mortgages", &mortgages); err != nil {
			return ErrResult(err), nil, nil
		}
		if in.Owner != nil {
			filtered := mortgages[:0]
			for _, m := range mortgages {
				if m.Owner == *in.Owner {
					filtered = append(filtered, m)
				}
			}
			mortgages = filtered
		}
		return JSONResult(mortgages), nil, nil
	})

	// get_mortgage
	type getMortgageInput struct {
		ID string `json:"id" jsonschema:"The mortgage ID"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_mortgage",
		Description: "Get a specific mortgage by ID",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getMortgageInput) (*sdkmcp.CallToolResult, any, error) {
		var mortgage api.Mortgage
		if err := c.Get("/mortgages/"+in.ID, &mortgage); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(mortgage), nil, nil
	})

	// create_mortgage
	type createMortgageInput struct {
		Name              string   `json:"name" jsonschema:"Mortgage name"`
		Owner             string   `json:"owner" jsonschema:"Owner: person1, person2, or joint"`
		Institution       string   `json:"institution" jsonschema:"Financial institution"`
		OriginalPrincipal float64  `json:"originalPrincipal" jsonschema:"Original loan amount"`
		CurrentBalance    float64  `json:"currentBalance" jsonschema:"Current outstanding balance"`
		InterestRate      float64  `json:"interestRate" jsonschema:"Annual interest rate as percentage (e.g. 5.25)"`
		PaymentAmount     float64  `json:"paymentAmount" jsonschema:"Regular payment amount"`
		PaymentFrequency  string   `json:"paymentFrequency" jsonschema:"Payment frequency: monthly, bi-weekly, accelerated-bi-weekly, or weekly"`
		StartDate         string   `json:"startDate" jsonschema:"Mortgage start date (YYYY-MM-DD)"`
		TermEndDate       string   `json:"termEndDate" jsonschema:"Current term end date (YYYY-MM-DD)"`
		AmortizationYears int      `json:"amortizationYears" jsonschema:"Total amortization period in years"`
		Description       *string  `json:"description,omitempty" jsonschema:"Optional description"`
		ExtraPayment      *float64 `json:"extraPayment,omitempty" jsonschema:"Optional recurring extra payment"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_mortgage",
		Description: "Create a new mortgage",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in createMortgageInput) (*sdkmcp.CallToolResult, any, error) {
		body := map[string]any{
			"name":              in.Name,
			"owner":             in.Owner,
			"institution":       in.Institution,
			"originalPrincipal": in.OriginalPrincipal,
			"currentBalance":    in.CurrentBalance,
			"interestRate":      in.InterestRate,
			"paymentAmount":     in.PaymentAmount,
			"paymentFrequency":  in.PaymentFrequency,
			"startDate":         in.StartDate,
			"termEndDate":       in.TermEndDate,
			"amortizationYears": in.AmortizationYears,
		}
		if in.Description != nil {
			body["description"] = *in.Description
		}
		if in.ExtraPayment != nil {
			body["extraPayment"] = *in.ExtraPayment
		}
		var mortgage api.Mortgage
		if err := c.Post("/mortgages", body, &mortgage); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(mortgage), nil, nil
	})

	// update_mortgage
	type updateMortgageInput struct {
		ID                string   `json:"id" jsonschema:"The mortgage ID to update"`
		Name              *string  `json:"name,omitempty" jsonschema:"New name"`
		Description       *string  `json:"description,omitempty" jsonschema:"New description"`
		Owner             *string  `json:"owner,omitempty" jsonschema:"New owner: person1, person2, or joint"`
		Institution       *string  `json:"institution,omitempty" jsonschema:"New institution"`
		OriginalPrincipal *float64 `json:"originalPrincipal,omitempty" jsonschema:"New original principal"`
		CurrentBalance    *float64 `json:"currentBalance,omitempty" jsonschema:"New current balance"`
		InterestRate      *float64 `json:"interestRate,omitempty" jsonschema:"New annual interest rate as percentage"`
		PaymentAmount     *float64 `json:"paymentAmount,omitempty" jsonschema:"New payment amount"`
		PaymentFrequency  *string  `json:"paymentFrequency,omitempty" jsonschema:"New payment frequency"`
		StartDate         *string  `json:"startDate,omitempty" jsonschema:"New start date (YYYY-MM-DD)"`
		TermEndDate       *string  `json:"termEndDate,omitempty" jsonschema:"New term end date (YYYY-MM-DD)"`
		AmortizationYears *int     `json:"amortizationYears,omitempty" jsonschema:"New amortization years"`
		ExtraPayment      *float64 `json:"extraPayment,omitempty" jsonschema:"New recurring extra payment"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_mortgage",
		Description: "Update an existing mortgage. Only provided fields are updated.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in updateMortgageInput) (*sdkmcp.CallToolResult, any, error) {
		updates := map[string]any{}
		if in.Name != nil { updates["name"] = *in.Name }
		if in.Description != nil { updates["description"] = *in.Description }
		if in.Owner != nil { updates["owner"] = *in.Owner }
		if in.Institution != nil { updates["institution"] = *in.Institution }
		if in.OriginalPrincipal != nil { updates["originalPrincipal"] = *in.OriginalPrincipal }
		if in.CurrentBalance != nil { updates["currentBalance"] = *in.CurrentBalance }
		if in.InterestRate != nil { updates["interestRate"] = *in.InterestRate }
		if in.PaymentAmount != nil { updates["paymentAmount"] = *in.PaymentAmount }
		if in.PaymentFrequency != nil { updates["paymentFrequency"] = *in.PaymentFrequency }
		if in.StartDate != nil { updates["startDate"] = *in.StartDate }
		if in.TermEndDate != nil { updates["termEndDate"] = *in.TermEndDate }
		if in.AmortizationYears != nil { updates["amortizationYears"] = *in.AmortizationYears }
		if in.ExtraPayment != nil { updates["extraPayment"] = *in.ExtraPayment }
		var mortgage api.Mortgage
		if err := c.MergeAndUpdate("/mortgages/"+in.ID, updates, &mortgage); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(mortgage), nil, nil
	})

	// delete_mortgage
	type deleteMortgageInput struct {
		ID string `json:"id" jsonschema:"The mortgage ID to delete"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_mortgage",
		Description: "Delete a mortgage",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in deleteMortgageInput) (*sdkmcp.CallToolResult, any, error) {
		if err := c.Delete("/mortgages/" + in.ID); err != nil {
			return ErrResult(err), nil, nil
		}
		return TextResult("Mortgage deleted successfully."), nil, nil
	})

	// get_mortgage_projection
	type getMortgageProjectionInput struct {
		ID string `json:"id" jsonschema:"The mortgage ID"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_mortgage_projection",
		Description: "Get amortization schedule and payoff projection for a mortgage",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getMortgageProjectionInput) (*sdkmcp.CallToolResult, any, error) {
		var projection any
		if err := c.Get("/mortgages/"+in.ID+"/projection", &projection); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(projection), nil, nil
	})
}
