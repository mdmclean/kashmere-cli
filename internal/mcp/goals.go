// internal/mcp/goals.go
package mcp

import (
	"context"
	"fmt"

	"github.com/mdmclean/kashmere-cli/internal/api"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerGoalTools(server *sdkmcp.Server, c *api.Client) {
	type noInput struct{}

	// list_goals
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_goals",
		Description: "List all financial goals",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, _ noInput) (*sdkmcp.CallToolResult, any, error) {
		var goals []api.Goal
		if err := c.Get("/goals", &goals); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(goals), nil, nil
	})

	// get_goal
	type getGoalInput struct {
		ID string `json:"id" jsonschema:"The goal ID"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get_goal",
		Description: "Get a specific financial goal by ID",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in getGoalInput) (*sdkmcp.CallToolResult, any, error) {
		var goal api.Goal
		if err := c.Get("/goals/"+in.ID, &goal); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(goal), nil, nil
	})

	// create_goal
	type goalAllocation struct {
		Category   string  `json:"category" jsonschema:"Asset category"`
		Percentage float64 `json:"percentage" jsonschema:"Allocation percentage (0-100)"`
	}
	type createGoalInput struct {
		Name        string           `json:"name" jsonschema:"Name of the goal"`
		Description *string          `json:"description,omitempty" jsonschema:"Optional description"`
		TargetType  *string          `json:"targetType,omitempty" jsonschema:"Optional target type: fixed or percentage"`
		TargetValue *float64         `json:"targetValue,omitempty" jsonschema:"Optional target value (use with targetType)"`
		Allocations []goalAllocation `json:"allocations,omitempty" jsonschema:"Optional target asset allocations, must sum to 100"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "create_goal",
		Description: "Create a new financial goal",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in createGoalInput) (*sdkmcp.CallToolResult, any, error) {
		body := map[string]any{"name": in.Name}
		if in.Description != nil {
			body["description"] = *in.Description
		}
		if in.TargetType != nil && in.TargetValue != nil {
			body["target"] = map[string]any{"type": *in.TargetType, "value": *in.TargetValue}
		}
		if in.Allocations != nil {
			body["allocations"] = in.Allocations
		}
		var goal api.Goal
		if err := c.Post("/goals", body, &goal); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(goal), nil, nil
	})

	// update_goal
	type updateGoalInput struct {
		ID               string           `json:"id" jsonschema:"The goal ID to update"`
		Name             *string          `json:"name,omitempty" jsonschema:"New name"`
		Description      *string          `json:"description,omitempty" jsonschema:"New description"`
		TargetType       *string          `json:"targetType,omitempty" jsonschema:"New target type: fixed or percentage (use with targetValue)"`
		TargetValue      *float64         `json:"targetValue,omitempty" jsonschema:"New target value (use with targetType)"`
		ClearTarget      *bool            `json:"clearTarget,omitempty" jsonschema:"Set true to remove the target"`
		Allocations      []goalAllocation `json:"allocations,omitempty" jsonschema:"New asset allocations, must sum to 100"`
		ClearAllocations *bool            `json:"clearAllocations,omitempty" jsonschema:"Set true to remove allocations"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "update_goal",
		Description: "Update an existing financial goal. Only provided fields are updated.",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in updateGoalInput) (*sdkmcp.CallToolResult, any, error) {
		updates := map[string]any{}
		if in.Name != nil {
			updates["name"] = *in.Name
		}
		if in.Description != nil {
			updates["description"] = *in.Description
		}
		if in.ClearTarget != nil && *in.ClearTarget {
			updates["target"] = nil
		} else if in.TargetType != nil && in.TargetValue != nil {
			updates["target"] = map[string]any{"type": *in.TargetType, "value": *in.TargetValue}
		} else if in.TargetType != nil || in.TargetValue != nil {
			missing := "targetValue"
			if in.TargetValue != nil {
				missing = "targetType"
			}
			return ErrResult(fmt.Errorf("targetType and targetValue must be provided together; missing: %s", missing)), nil, nil
		}
		if in.ClearAllocations != nil && *in.ClearAllocations {
			updates["allocations"] = nil
		} else if in.Allocations != nil {
			updates["allocations"] = in.Allocations
		}
		var goal api.Goal
		if err := c.MergeAndUpdate("/goals/"+in.ID, updates, &goal); err != nil {
			return ErrResult(err), nil, nil
		}
		return JSONResult(goal), nil, nil
	})

	// delete_goal
	type deleteGoalInput struct {
		ID string `json:"id" jsonschema:"The goal ID to delete"`
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "delete_goal",
		Description: "Delete a financial goal",
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in deleteGoalInput) (*sdkmcp.CallToolResult, any, error) {
		if err := c.Delete("/goals/" + in.ID); err != nil {
			return ErrResult(err), nil, nil
		}
		return TextResult("Goal deleted successfully."), nil, nil
	})
}
