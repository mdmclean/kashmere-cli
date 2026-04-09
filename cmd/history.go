// cmd/history.go
package cmd

import (
	"github.com/kashemere/kashemere-cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	historyFrom        string
	historyTo          string
	historyPortfolioID string
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View portfolio history snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		path := "/history/snapshots"
		sep := "?"
		if historyFrom != "" {
			path += sep + "startDate=" + historyFrom
			sep = "&"
		}
		if historyTo != "" {
			path += sep + "endDate=" + historyTo
			sep = "&"
		}

		var snapshots []api.PortfolioSnapshot
		if err := client.Get(path, &snapshots); err != nil {
			outputError(err, 0)
		}

		// Client-side portfolio filter (server cannot filter by portfolioId on encrypted docs)
		if historyPortfolioID != "" {
			filtered := snapshots[:0]
			for _, s := range snapshots {
				if s.PortfolioID == historyPortfolioID {
					filtered = append(filtered, s)
				}
			}
			snapshots = filtered
		}

		outputJSON(snapshots)
		return nil
	},
}

func init() {
	historyCmd.Flags().StringVar(&historyFrom, "from", "", "Start date YYYY-MM-DD")
	historyCmd.Flags().StringVar(&historyTo, "to", "", "End date YYYY-MM-DD")
	historyCmd.Flags().StringVar(&historyPortfolioID, "portfolio-id", "", "Filter by portfolio ID (client-side)")
	rootCmd.AddCommand(historyCmd)
}
