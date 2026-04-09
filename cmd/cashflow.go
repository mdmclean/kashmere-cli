// cmd/cashflow.go
package cmd

import (
	"fmt"
	"strconv"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/spf13/cobra"
)

var cashflowCmd = &cobra.Command{
	Use:   "cashflow",
	Short: "Manage cash flows (deposits and withdrawals)",
}

var cashflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cash flows",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		path := "/cashflows"
		if cfListPortfolioID != "" {
			path += "?portfolioId=" + cfListPortfolioID
		}
		var cashflows []api.CashFlow
		if err := client.Get(path, &cashflows); err != nil {
			outputError(err, 0)
		}
		outputJSON(cashflows)
		return nil
	},
}

var cashflowGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a cash flow by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
		var cf api.CashFlow
		if err := client.Get("/cashflows/"+args[0], &cf); err != nil {
			outputError(err, 0)
		}
		outputJSON(cf)
		return nil
	},
}

var (
	cfListPortfolioID string
	cfPortfolioID     string
	cfType            string
	cfAmount          string
	cfDate            string
	cfCashAssetID     string
	cfDescription     string
)

var cashflowCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a cash flow",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		amount, err := strconv.ParseFloat(cfAmount, 64)
		if err != nil {
			return fmt.Errorf("--amount must be a number: %w", err)
		}

		body := map[string]any{
			"portfolioId": cfPortfolioID,
			"type":        cfType,
			"amount":      amount,
			"date":        cfDate,
		}
		if cfCashAssetID != "" {
			body["cashAssetId"] = cfCashAssetID
		}
		if cfDescription != "" {
			body["description"] = cfDescription
		}

		var cf api.CashFlow
		if err := client.Post("/cashflows", body, &cf); err != nil {
			outputError(err, 0)
		}
		outputJSON(cf)
		return nil
	},
}

var cashflowUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a cash flow (fetches current, merges flags, puts full object)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		updates := map[string]any{}
		if cmd.Flags().Changed("portfolio-id") {
			updates["portfolioId"] = cfPortfolioID
		}
		if cmd.Flags().Changed("type") {
			updates["type"] = cfType
		}
		if cmd.Flags().Changed("amount") {
			v, err := strconv.ParseFloat(cfAmount, 64)
			if err != nil {
				return fmt.Errorf("--amount must be a number: %w", err)
			}
			updates["amount"] = v
		}
		if cmd.Flags().Changed("date") {
			updates["date"] = cfDate
		}
		if cmd.Flags().Changed("cash-asset-id") {
			updates["cashAssetId"] = cfCashAssetID
		}
		if cmd.Flags().Changed("description") {
			updates["description"] = cfDescription
		}
		if len(updates) == 0 {
			return fmt.Errorf("no flags provided — nothing to update")
		}

		var result api.CashFlow
		if err := client.MergeAndUpdate("/cashflows/"+args[0], updates, &result); err != nil {
			outputError(err, 0)
		}
		outputJSON(result)
		return nil
	},
}

func init() {
	cashflowListCmd.Flags().StringVar(&cfListPortfolioID, "portfolio-id", "", "Filter by portfolio ID")

	cashflowCreateCmd.Flags().StringVar(&cfPortfolioID, "portfolio-id", "", "Portfolio ID (required)")
	cashflowCreateCmd.Flags().StringVar(&cfType, "type", "", "Type: deposit|withdrawal (required)")
	cashflowCreateCmd.Flags().StringVar(&cfAmount, "amount", "", "Amount (required)")
	cashflowCreateCmd.Flags().StringVar(&cfDate, "date", "", "Date YYYY-MM-DD (required)")
	cashflowCreateCmd.Flags().StringVar(&cfCashAssetID, "cash-asset-id", "", "Cash asset ID")
	cashflowCreateCmd.Flags().StringVar(&cfDescription, "description", "", "Description")
	cashflowCreateCmd.MarkFlagRequired("portfolio-id")
	cashflowCreateCmd.MarkFlagRequired("type")
	cashflowCreateCmd.MarkFlagRequired("amount")
	cashflowCreateCmd.MarkFlagRequired("date")

	cashflowUpdateCmd.Flags().StringVar(&cfPortfolioID, "portfolio-id", "", "New portfolio ID")
	cashflowUpdateCmd.Flags().StringVar(&cfType, "type", "", "New type: deposit|withdrawal")
	cashflowUpdateCmd.Flags().StringVar(&cfAmount, "amount", "", "New amount")
	cashflowUpdateCmd.Flags().StringVar(&cfDate, "date", "", "New date YYYY-MM-DD")
	cashflowUpdateCmd.Flags().StringVar(&cfCashAssetID, "cash-asset-id", "", "New cash asset ID")
	cashflowUpdateCmd.Flags().StringVar(&cfDescription, "description", "", "New description")

	cashflowCmd.AddCommand(cashflowListCmd, cashflowGetCmd, cashflowCreateCmd, cashflowUpdateCmd)
	rootCmd.AddCommand(cashflowCmd)
}
