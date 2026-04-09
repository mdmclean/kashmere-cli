// cmd/price.go
package cmd

import (
	"strings"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/spf13/cobra"
)

var priceCmd = &cobra.Command{
	Use:   "price",
	Short: "View asset prices",
}

var priceSymbols string

var priceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tracked asset prices",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)

		path := "/prices"
		if priceSymbols != "" {
			symbols := strings.Join(strings.Fields(strings.ReplaceAll(priceSymbols, ",", " ")), ",")
			path += "?tickers=" + symbols
		}

		var prices []api.TickerPrice
		if err := client.Get(path, &prices); err != nil {
			outputError(err, 0)
		}
		outputJSON(prices)
		return nil
	},
}

func init() {
	priceListCmd.Flags().StringVar(&priceSymbols, "symbols", "", "Comma-separated ticker symbols (e.g. VCN,VFV,XEQT)")
	priceCmd.AddCommand(priceListCmd)
	rootCmd.AddCommand(priceCmd)
}
