// cmd/portfolio.go
package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kashemere/kashemere-cli/internal/api"
	"github.com/spf13/cobra"
)

var portfolioCmd = &cobra.Command{
	Use:   "portfolio",
	Short: "Manage portfolios",
}

var portfolioListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all portfolios",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var portfolios []api.Portfolio
		if err := client.Get("/portfolios", &portfolios); err != nil {
			outputError(err, 0)
		}
		outputJSON(portfolios)
		return nil
	},
}

var portfolioGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a portfolio by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var portfolio api.Portfolio
		if err := client.Get("/portfolios/"+args[0], &portfolio); err != nil {
			outputError(err, 0)
		}
		outputJSON(portfolio)
		return nil
	},
}

var (
	pfName                   string
	pfDescription            string
	pfInstitution            string
	pfOwner                  string
	pfManagementType         string
	pfGoalID                 string
	pfTotalValue             string
	pfMinTransactionAmount   string
	pfMinTransactionCurrency string
	pfAllocationsJSON        string
	pfAssetsJSON             string
)

var portfolioCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new portfolio",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		totalValue, err := strconv.ParseFloat(pfTotalValue, 64)
		if err != nil {
			return fmt.Errorf("--total-value must be a number: %w", err)
		}

		var allocations []api.Allocation
		if pfAllocationsJSON != "" {
			if err := json.Unmarshal([]byte(pfAllocationsJSON), &allocations); err != nil {
				return fmt.Errorf("--allocations must be valid JSON: %w", err)
			}
		}

		var assets []api.Asset
		if pfAssetsJSON != "" {
			if err := json.Unmarshal([]byte(pfAssetsJSON), &assets); err != nil {
				return fmt.Errorf("--assets must be valid JSON: %w", err)
			}
		}

		body := map[string]any{
			"name":           pfName,
			"institution":    pfInstitution,
			"owner":          pfOwner,
			"managementType": pfManagementType,
			"goalId":         pfGoalID,
			"totalValue":     totalValue,
			"allocations":    allocations,
			"assets":         assets,
		}
		if pfDescription != "" {
			body["description"] = pfDescription
		}
		if pfMinTransactionAmount != "" {
			v, err := strconv.ParseFloat(pfMinTransactionAmount, 64)
			if err != nil {
				return fmt.Errorf("--min-transaction-amount must be a number: %w", err)
			}
			body["minTransactionAmount"] = v
		}
		if pfMinTransactionCurrency != "" {
			body["minTransactionCurrency"] = pfMinTransactionCurrency
		}

		var portfolio api.Portfolio
		if err := client.Post("/portfolios", body, &portfolio); err != nil {
			outputError(err, 0)
		}
		outputJSON(portfolio)
		return nil
	},
}

var portfolioUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a portfolio (fetches current, merges flags, puts full object)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		updates := map[string]any{}
		if cmd.Flags().Changed("name") {
			updates["name"] = pfName
		}
		if cmd.Flags().Changed("description") {
			updates["description"] = pfDescription
		}
		if cmd.Flags().Changed("institution") {
			updates["institution"] = pfInstitution
		}
		if cmd.Flags().Changed("owner") {
			updates["owner"] = pfOwner
		}
		if cmd.Flags().Changed("management-type") {
			updates["managementType"] = pfManagementType
		}
		if cmd.Flags().Changed("goal-id") {
			updates["goalId"] = pfGoalID
		}
		if cmd.Flags().Changed("total-value") {
			v, err := strconv.ParseFloat(pfTotalValue, 64)
			if err != nil {
				return fmt.Errorf("--total-value must be a number: %w", err)
			}
			updates["totalValue"] = v
		}
		if cmd.Flags().Changed("allocations") {
			var allocations []api.Allocation
			if err := json.Unmarshal([]byte(pfAllocationsJSON), &allocations); err != nil {
				return fmt.Errorf("--allocations must be valid JSON: %w", err)
			}
			updates["allocations"] = allocations
		}
		if cmd.Flags().Changed("assets") {
			var assets []api.Asset
			if err := json.Unmarshal([]byte(pfAssetsJSON), &assets); err != nil {
				return fmt.Errorf("--assets must be valid JSON: %w", err)
			}
			updates["assets"] = assets
		}
		if cmd.Flags().Changed("min-transaction-amount") {
			v, err := strconv.ParseFloat(pfMinTransactionAmount, 64)
			if err != nil {
				return fmt.Errorf("--min-transaction-amount must be a number: %w", err)
			}
			updates["minTransactionAmount"] = v
		}
		if cmd.Flags().Changed("min-transaction-currency") {
			updates["minTransactionCurrency"] = pfMinTransactionCurrency
		}

		if len(updates) == 0 {
			return fmt.Errorf("no flags provided — nothing to update")
		}

		var result api.Portfolio
		path := "/portfolios/" + args[0]
		if err := client.MergeAndUpdate(path, updates, &result); err != nil {
			outputError(err, 0)
		}
		outputJSON(result)
		return nil
	},
}

func init() {
	// create flags
	portfolioCreateCmd.Flags().StringVar(&pfName, "name", "", "Portfolio name (required)")
	portfolioCreateCmd.Flags().StringVar(&pfDescription, "description", "", "Description")
	portfolioCreateCmd.Flags().StringVar(&pfInstitution, "institution", "", "Institution name (required)")
	portfolioCreateCmd.Flags().StringVar(&pfOwner, "owner", "", "Owner: person1|person2|joint (required)")
	portfolioCreateCmd.Flags().StringVar(&pfManagementType, "management-type", "self", "Management type: self|auto")
	portfolioCreateCmd.Flags().StringVar(&pfGoalID, "goal-id", "", "Goal ID to assign this portfolio to (required)")
	portfolioCreateCmd.Flags().StringVar(&pfTotalValue, "total-value", "", "Total portfolio value in display currency (required)")
	portfolioCreateCmd.Flags().StringVar(&pfMinTransactionAmount, "min-transaction-amount", "", "Minimum transaction amount")
	portfolioCreateCmd.Flags().StringVar(&pfMinTransactionCurrency, "min-transaction-currency", "", "Min transaction currency: CAD|USD")
	portfolioCreateCmd.Flags().StringVar(&pfAllocationsJSON, "allocations", "[]", `Allocations JSON: '[{"category":"US Equity","percentage":100}]'`)
	portfolioCreateCmd.Flags().StringVar(&pfAssetsJSON, "assets", "[]", `Assets JSON: '[{"id":"...","ticker":"VCN","name":"...","category":"...","quantity":100}]'`)
	portfolioCreateCmd.MarkFlagRequired("name")
	portfolioCreateCmd.MarkFlagRequired("institution")
	portfolioCreateCmd.MarkFlagRequired("owner")
	portfolioCreateCmd.MarkFlagRequired("goal-id")
	portfolioCreateCmd.MarkFlagRequired("total-value")

	// update flags (same names, none required)
	portfolioUpdateCmd.Flags().StringVar(&pfName, "name", "", "New name")
	portfolioUpdateCmd.Flags().StringVar(&pfDescription, "description", "", "New description")
	portfolioUpdateCmd.Flags().StringVar(&pfInstitution, "institution", "", "New institution")
	portfolioUpdateCmd.Flags().StringVar(&pfOwner, "owner", "", "New owner: person1|person2|joint")
	portfolioUpdateCmd.Flags().StringVar(&pfManagementType, "management-type", "", "New management type: self|auto")
	portfolioUpdateCmd.Flags().StringVar(&pfGoalID, "goal-id", "", "New goal ID")
	portfolioUpdateCmd.Flags().StringVar(&pfTotalValue, "total-value", "", "New total value")
	portfolioUpdateCmd.Flags().StringVar(&pfMinTransactionAmount, "min-transaction-amount", "", "New min transaction amount")
	portfolioUpdateCmd.Flags().StringVar(&pfMinTransactionCurrency, "min-transaction-currency", "", "New min transaction currency: CAD|USD")
	portfolioUpdateCmd.Flags().StringVar(&pfAllocationsJSON, "allocations", "", "New allocations JSON")
	portfolioUpdateCmd.Flags().StringVar(&pfAssetsJSON, "assets", "", "New assets JSON")

	portfolioCmd.AddCommand(portfolioListCmd, portfolioGetCmd, portfolioCreateCmd, portfolioUpdateCmd)
	rootCmd.AddCommand(portfolioCmd)
}
