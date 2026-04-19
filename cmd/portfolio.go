// cmd/portfolio.go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/performance"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
	"github.com/mdmclean/kashmere-cli/internal/trades"
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
		enriched, err := portfolio.EnrichFull(portfolios, client)
		if err != nil {
			outputError(err, 0)
		}
		outputJSON(enriched)
		return nil
	},
}

var portfolioGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a portfolio by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var p api.Portfolio
		if err := client.Get("/portfolios/"+args[0], &p); err != nil {
			outputError(err, 0)
		}
		enriched, err := portfolio.EnrichFull([]api.Portfolio{p}, client)
		if err != nil {
			outputError(err, 0)
		}
		outputJSON(enriched[0])
		return nil
	},
}

var (
	perfFrom string
	perfTo   string
)

var portfolioPerformanceCmd = &cobra.Command{
	Use:   "performance <id>",
	Short: "Compute time-weighted return (TWR) for a portfolio",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		portfolioID := args[0]

		// Fetch the portfolio for its name
		var p api.Portfolio
		if err := client.Get("/portfolios/"+portfolioID, &p); err != nil {
			outputError(err, 0)
		}

		// Fetch snapshots (filtered by date range server-side)
		snapshotPath := "/history/snapshots"
		params := url.Values{}
		if perfFrom != "" {
			params.Set("startDate", perfFrom)
		}
		if perfTo != "" {
			params.Set("endDate", perfTo)
		}
		if len(params) > 0 {
			snapshotPath += "?" + params.Encode()
		}
		var snapshots []api.PortfolioSnapshot
		if err := client.GetSnapshots(snapshotPath, &snapshots); err != nil {
			outputError(err, 0)
		}
		// Filter client-side to this portfolio
		filtered := snapshots[:0]
		for _, s := range snapshots {
			if s.PortfolioID == portfolioID {
				filtered = append(filtered, s)
			}
		}
		snapshots = filtered

		// Fetch cashflows and filter client-side
		var cashflows []api.CashFlow
		if err := client.Get("/cashflows", &cashflows); err != nil {
			outputError(err, 0)
		}

		result, err := performance.Compute(portfolioID, p.Name, snapshots, cashflows, perfFrom, perfTo)
		if err != nil {
			return err
		}
		outputJSON(result)
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

		totalValue := 0.0
		if pfTotalValue != "" {
			v, err := strconv.ParseFloat(pfTotalValue, 64)
			if err != nil {
				return fmt.Errorf("--total-value must be a number: %w", err)
			}
			totalValue = v
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

		if err := portfolio.Validate(api.Portfolio{Allocations: allocations, Assets: assets}); err != nil {
			return err
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

		var created api.Portfolio
		if err := client.Post("/portfolios", body, &created); err != nil {
			outputError(err, 0)
		}
		outputJSON(created)
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

		path := "/portfolios/" + args[0]

		// Validate merged state if allocation-sensitive fields are changing.
		if updates["allocations"] != nil || updates["assets"] != nil {
			var current api.Portfolio
			if err := client.Get(path, &current); err != nil {
				outputError(err, 0)
			}
			if v, ok := updates["allocations"]; ok {
				current.Allocations = v.([]api.Allocation)
			}
			if v, ok := updates["assets"]; ok {
				current.Assets = v.([]api.Asset)
			}
			if err := portfolio.Validate(current); err != nil {
				return err
			}
		}

		var result api.Portfolio
		if err := client.MergeAndUpdate(path, updates, &result); err != nil {
			outputError(err, 0)
		}
		outputJSON(result)
		return nil
	},
}

var portfolioTopTradesCmd = &cobra.Command{
	Use:   "top-trades [portfolioId]",
	Short: "Get ranked BUY/SELL trade recommendations based on drift from target allocations",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var portfolios []api.Portfolio
		if len(args) == 1 {
			var p api.Portfolio
			if err := client.Get("/portfolios/"+args[0], &p); err != nil {
				outputError(err, 0)
			}
			portfolios = []api.Portfolio{p}
		} else {
			if err := client.Get("/portfolios", &portfolios); err != nil {
				outputError(err, 0)
			}
		}
		recs, err := trades.Compute(portfolios, client)
		if err != nil {
			outputError(err, 0)
		}
		outputJSON(recs)
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
	portfolioCreateCmd.Flags().StringVar(&pfTotalValue, "total-value", "", "Total portfolio value (optional — computed from assets when present)")
	portfolioCreateCmd.Flags().StringVar(&pfMinTransactionAmount, "min-transaction-amount", "", "Minimum transaction amount")
	portfolioCreateCmd.Flags().StringVar(&pfMinTransactionCurrency, "min-transaction-currency", "", "Min transaction currency: CAD|USD")
	portfolioCreateCmd.Flags().StringVar(&pfAllocationsJSON, "allocations", "[]", `Allocations JSON: '[{"category":"US Equity","percentage":100}]'`)
	portfolioCreateCmd.Flags().StringVar(&pfAssetsJSON, "assets", "[]", `Assets JSON: '[{"id":"...","ticker":"VCN","name":"...","category":"...","quantity":100}]'`)
	portfolioCreateCmd.MarkFlagRequired("name")
	portfolioCreateCmd.MarkFlagRequired("institution")
	portfolioCreateCmd.MarkFlagRequired("owner")
	portfolioCreateCmd.MarkFlagRequired("goal-id")

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

	// performance flags
	portfolioPerformanceCmd.Flags().StringVar(&perfFrom, "from", "", "Start date YYYY-MM-DD (default: earliest snapshot)")
	portfolioPerformanceCmd.Flags().StringVar(&perfTo, "to", "", "End date YYYY-MM-DD (default: latest snapshot)")

	portfolioCmd.AddCommand(portfolioListCmd, portfolioGetCmd, portfolioCreateCmd, portfolioUpdateCmd, portfolioTopTradesCmd, portfolioPerformanceCmd)
	rootCmd.AddCommand(portfolioCmd)
}
