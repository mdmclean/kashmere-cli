// internal/portfolio/enrich.go
package portfolio

import (
	"net/url"
	"strings"

	"github.com/mdmclean/kashmere-cli/internal/api"
)

// staticTickers matches isStaticAsset() in @fp/shared/types.ts.
var staticTickers = map[string]bool{
	"CASH":       true,
	"GIC":        true,
	"PROPERTY":   true,
	"MUTUALFUND": true,
}

const fxTickerUSDCAD = "USDCAD=X"

func isStatic(ticker string) bool {
	return staticTickers[strings.ToUpper(ticker)]
}

// priceKey returns the map key used to look up a price.
// Matches getPriceKey() in the web client: "TICKER:EXCHANGE" or "TICKER".
func priceKey(ticker, exchange string) string {
	if exchange != "" {
		return strings.ToUpper(ticker) + ":" + strings.ToUpper(exchange)
	}
	return strings.ToUpper(ticker)
}

// fxRate returns the conversion multiplier from → to.
// Mirrors getExchangeRate() in currency.ts.
func fxRate(from, to string, priceMap map[string]api.TickerPrice) float64 {
	if from == to || from == "" {
		return 1
	}
	usdcad, ok := priceMap[fxTickerUSDCAD]
	if !ok || usdcad.LatestPrice == nil || *usdcad.LatestPrice <= 0 {
		return 1
	}
	rate := *usdcad.LatestPrice
	if from == "USD" && to == "CAD" {
		return rate
	}
	if from == "CAD" && to == "USD" {
		return 1 / rate
	}
	return 1
}

// Enrich computes TotalValue for each portfolio from its assets and live prices.
// It mirrors enrichPortfoliosWithPrices() in usePortfolios.ts.
//
// Rules:
//   - Portfolios with no assets are returned unchanged.
//   - Static assets (CASH, GIC, PROPERTY, MUTUALFUND): value = quantity.
//   - Traded assets: value = quantity × latestPrice (skipped if price unavailable).
//   - All values are converted to displayCurrency (from /settings, default CAD).
//   - If no asset in a portfolio contributes a value, the portfolio is returned unchanged.
func Enrich(portfolios []api.Portfolio, c *api.Client) ([]api.Portfolio, error) {
	// Collect unique non-static tickers + always include USDCAD=X.
	tickerSet := map[string]struct{}{fxTickerUSDCAD: {}}
	for _, p := range portfolios {
		for _, a := range p.Assets {
			if !isStatic(a.Ticker) {
				tickerSet[priceKey(a.Ticker, a.Exchange)] = struct{}{}
			}
		}
	}

	// Fetch prices (bulk call).
	priceMap := map[string]api.TickerPrice{}
	if len(tickerSet) > 0 {
		tickers := make([]string, 0, len(tickerSet))
		for k := range tickerSet {
			tickers = append(tickers, k)
		}
		params := url.Values{}
		params.Set("tickers", strings.Join(tickers, ","))
		var prices []api.TickerPrice
		if err := c.Get("/prices?"+params.Encode(), &prices); err == nil {
			for _, p := range prices {
				priceMap[priceKey(p.Ticker, p.Exchange)] = p
				// Also index by ticker alone for fallback matching.
				if p.Exchange != "" {
					bare := strings.ToUpper(p.Ticker)
					if _, exists := priceMap[bare]; !exists {
						priceMap[bare] = p
					}
				}
			}
		}
		// Intentionally ignore price fetch errors — best-effort enrichment.
	}

	// Fetch displayCurrency from settings; default to CAD on failure.
	displayCurrency := "CAD"
	var settings api.Settings
	if err := c.Get("/settings", &settings); err == nil && settings.DisplayCurrency != "" {
		displayCurrency = settings.DisplayCurrency
	}

	result := make([]api.Portfolio, len(portfolios))
	for i, p := range portfolios {
		if len(p.Assets) == 0 {
			result[i] = p
			continue
		}

		total := 0.0
		hasAnyPrice := false

		for _, a := range p.Assets {
			var assetValue float64
			var assetCurrency string

			if isStatic(a.Ticker) {
				assetValue = a.Quantity
				assetCurrency = a.Currency
				if assetCurrency == "" {
					assetCurrency = displayCurrency
				}
				hasAnyPrice = true
			} else {
				key := priceKey(a.Ticker, a.Exchange)
				priceData, ok := priceMap[key]
				if !ok {
					// Try bare ticker fallback.
					priceData, ok = priceMap[strings.ToUpper(a.Ticker)]
				}
				if !ok || priceData.LatestPrice == nil {
					continue
				}
				assetValue = a.Quantity * *priceData.LatestPrice
				assetCurrency = a.Currency
				if assetCurrency == "" {
					assetCurrency = priceData.Currency
				}
				if assetCurrency == "" {
					assetCurrency = "USD" // traded assets default to USD
				}
				hasAnyPrice = true
			}

			rate := fxRate(assetCurrency, displayCurrency, priceMap)
			total += assetValue * rate
		}

		if !hasAnyPrice {
			result[i] = p
			continue
		}
		p.TotalValue = total
		result[i] = p
	}

	return result, nil
}
