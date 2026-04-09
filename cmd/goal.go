// cmd/goal.go
package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/spf13/cobra"
)

var goalCmd = &cobra.Command{
	Use:   "goal",
	Short: "Manage financial goals",
}

var goalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all goals",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var goals []api.Goal
		if err := client.Get("/goals", &goals); err != nil {
			outputError(err, 0)
		}
		outputJSON(goals)
		return nil
	},
}

var goalGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a goal by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var goal api.Goal
		if err := client.Get("/goals/"+args[0], &goal); err != nil {
			outputError(err, 0)
		}
		outputJSON(goal)
		return nil
	},
}

var (
	goalName        string
	goalDescription string
	goalTargetType  string
	goalTargetValue string
	goalAllocations string
)

var goalCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new goal",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		body := map[string]any{
			"name": goalName,
		}
		if goalDescription != "" {
			body["description"] = goalDescription
		}
		if goalTargetType != "" && goalTargetValue != "" {
			v, err := strconv.ParseFloat(goalTargetValue, 64)
			if err != nil {
				return fmt.Errorf("--target-value must be a number: %w", err)
			}
			body["target"] = map[string]any{"type": goalTargetType, "value": v}
		}
		if goalAllocations != "" {
			var allocs []api.Allocation
			if err := json.Unmarshal([]byte(goalAllocations), &allocs); err != nil {
				return fmt.Errorf("--allocations must be valid JSON: %w", err)
			}
			body["allocations"] = allocs
		}

		var goal api.Goal
		if err := client.Post("/goals", body, &goal); err != nil {
			outputError(err, 0)
		}
		outputJSON(goal)
		return nil
	},
}

var goalUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a goal (fetches current, merges flags, puts full object)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		updates := map[string]any{}
		if cmd.Flags().Changed("name") {
			updates["name"] = goalName
		}
		if cmd.Flags().Changed("description") {
			updates["description"] = goalDescription
		}
		if cmd.Flags().Changed("target-type") || cmd.Flags().Changed("target-value") {
			v, err := strconv.ParseFloat(goalTargetValue, 64)
			if err != nil {
				return fmt.Errorf("--target-value must be a number: %w", err)
			}
			updates["target"] = map[string]any{"type": goalTargetType, "value": v}
		}
		if cmd.Flags().Changed("allocations") {
			var allocs []api.Allocation
			if err := json.Unmarshal([]byte(goalAllocations), &allocs); err != nil {
				return fmt.Errorf("--allocations must be valid JSON: %w", err)
			}
			updates["allocations"] = allocs
		}
		if len(updates) == 0 {
			return fmt.Errorf("no flags provided — nothing to update")
		}

		var result api.Goal
		if err := client.MergeAndUpdate("/goals/"+args[0], updates, &result); err != nil {
			outputError(err, 0)
		}
		outputJSON(result)
		return nil
	},
}

func init() {
	goalCreateCmd.Flags().StringVar(&goalName, "name", "", "Goal name (required)")
	goalCreateCmd.Flags().StringVar(&goalDescription, "description", "", "Description")
	goalCreateCmd.Flags().StringVar(&goalTargetType, "target-type", "", "Target type: fixed|percentage")
	goalCreateCmd.Flags().StringVar(&goalTargetValue, "target-value", "", "Target value (dollars when fixed, 0-100 when percentage)")
	goalCreateCmd.Flags().StringVar(&goalAllocations, "allocations", "", `Target allocations JSON: '[{"category":"US Equity","percentage":60}]'`)
	goalCreateCmd.MarkFlagRequired("name")

	goalUpdateCmd.Flags().StringVar(&goalName, "name", "", "New name")
	goalUpdateCmd.Flags().StringVar(&goalDescription, "description", "", "New description")
	goalUpdateCmd.Flags().StringVar(&goalTargetType, "target-type", "", "New target type: fixed|percentage")
	goalUpdateCmd.Flags().StringVar(&goalTargetValue, "target-value", "", "New target value")
	goalUpdateCmd.Flags().StringVar(&goalAllocations, "allocations", "", "New allocations JSON")

	goalCmd.AddCommand(goalListCmd, goalGetCmd, goalCreateCmd, goalUpdateCmd)
	rootCmd.AddCommand(goalCmd)
}
