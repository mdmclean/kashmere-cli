// cmd/dashboard.go
package cmd

import (
	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/dashboard"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Show aggregated portfolio and goal summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		var portfolios []api.Portfolio
		if err := client.Get("/portfolios", &portfolios); err != nil {
			outputError(err, 0)
		}
		var goals []api.Goal
		if err := client.Get("/goals", &goals); err != nil {
			outputError(err, 0)
		}
		var mortgages []api.Mortgage
		client.Get("/mortgages", &mortgages) // optional, ignore error

		enriched, err := portfolio.Enrich(portfolios, client)
		if err != nil {
			outputError(err, 0)
		}

		outputJSON(dashboard.Compute(enriched, goals, mortgages))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
