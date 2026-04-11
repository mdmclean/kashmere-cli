// internal/mcp/settings.go
package mcp

import (
	"context"

	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerSettingsTools(server *sdkmcp.Server, c *api.Client) {
	type noInput struct{}

	// get_settings
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_settings",
		Description: "Get application settings including person names and display currency",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ noInput) (*sdkmcp.CallToolResult, any, error) {
		var settings api.Settings
		if err := c.Get("/settings", &settings); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(settings), nil, nil
	})

	// update_settings
	type updateSettingsInput struct {
		Person1Name     *string `json:"person1Name,omitempty" jsonschema:"Name for person 1"`
		Person2Name     *string `json:"person2Name,omitempty" jsonschema:"Name for person 2"`
		DisplayCurrency *string `json:"displayCurrency,omitempty" jsonschema:"Display currency: CAD or USD"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_settings",
		Description: "Update application settings. Only provided fields are updated.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in updateSettingsInput) (*sdkmcp.CallToolResult, any, error) {
		updates := map[string]any{}
		if in.Person1Name != nil {
			updates["person1Name"] = *in.Person1Name
		}
		if in.Person2Name != nil {
			updates["person2Name"] = *in.Person2Name
		}
		if in.DisplayCurrency != nil {
			updates["displayCurrency"] = *in.DisplayCurrency
		}
		var settings api.Settings
		if err := c.MergeAndUpdate("/settings", updates, &settings); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(settings), nil, nil
	})
}
