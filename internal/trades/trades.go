// internal/trades/trades.go
package trades

import (
	"math"
	"sort"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
)

// TradeRecommendation is a ranked BUY or SELL recommendation for a single asset.
type TradeRecommendation struct {
	PortfolioID   string  `json:"portfolioId"`
	PortfolioName string  `json:"portfolioName"`
	Ticker        string  `json:"ticker"`
	AssetName     string  `json:"assetName"`
	Direction     string  `json:"direction"`   // "BUY" | "SELL"
	CurrentPct    float64 `json:"currentPct"`  // current weight in portfolio (%)
	TargetPct     float64 `json:"targetPct"`   // target weight (0 if null in a portfolio with targets)
	DriftPct      float64 `json:"driftPct"`    // currentPct - targetPct (signed)
	DriftAmount   float64 `json:"driftAmount"` // |driftPct / 100 × totalValue|, in display currency
	Currency      string  `json:"currency"`    // display currency
}

// Compute returns a flat, drift-ranked list of BUY/SELL recommendations across
// the given portfolios. Portfolios with no targetPercentage on any asset are skipped.
// Assets with a null targetPercentage in a portfolio that has targets are treated as 0%.
func Compute(portfolios []api.Portfolio, c *api.Client) ([]TradeRecommendation, error) {
	displayCurrency := "CAD"
	var settings api.Settings
	if err := c.Get("/settings", &settings); err == nil && settings.DisplayCurrency != "" {
		displayCurrency = settings.DisplayCurrency
	}

	priceMap := portfolio.FetchPrices(portfolios, c)

	var results []TradeRecommendation

	for _, p := range portfolios {
		// Skip portfolios with no target percentages defined at all.
		hasTargets := false
		for _, a := range p.Assets {
			if a.TargetPercentage != nil {
				hasTargets = true
				break
			}
		}
		if !hasTargets {
			continue
		}

		// Compute per-asset values in display currency.
		type valued struct {
			asset api.Asset
			value float64
		}
		var assetVals []valued
		totalValue := 0.0
		for _, a := range p.Assets {
			val, ok := portfolio.ComputeAssetValue(a, priceMap, displayCurrency)
			if !ok {
				continue // price unavailable — skip silently
			}
			assetVals = append(assetVals, valued{asset: a, value: val})
			totalValue += val
		}

		if totalValue == 0 {
			continue // division guard
		}

		for _, av := range assetVals {
			currentPct := av.value / totalValue * 100

			targetPct := 0.0
			if av.asset.TargetPercentage != nil {
				targetPct = *av.asset.TargetPercentage
			}

			driftPct := currentPct - targetPct
			if driftPct == 0 {
				continue
			}
			driftAmount := math.Abs(driftPct / 100 * totalValue)

			// Filter by minTransactionAmount if set.
			if p.MinTransactionAmount != nil && *p.MinTransactionAmount > 0 {
				minAmt := *p.MinTransactionAmount
				if p.MinTransactionCurrency != "" && p.MinTransactionCurrency != displayCurrency {
					minAmt *= portfolio.FxRate(p.MinTransactionCurrency, displayCurrency, priceMap)
				}
				if driftAmount < minAmt {
					continue
				}
			}

			direction := "BUY"
			if driftPct > 0 {
				direction = "SELL"
			}

			results = append(results, TradeRecommendation{
				PortfolioID:   p.ID,
				PortfolioName: p.Name,
				Ticker:        av.asset.Ticker,
				AssetName:     av.asset.Name,
				Direction:     direction,
				CurrentPct:    currentPct,
				TargetPct:     targetPct,
				DriftPct:      driftPct,
				DriftAmount:   driftAmount,
				Currency:      displayCurrency,
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		ai, aj := math.Abs(results[i].DriftPct), math.Abs(results[j].DriftPct)
		if ai != aj {
			return ai > aj
		}
		if results[i].PortfolioID != results[j].PortfolioID {
			return results[i].PortfolioID < results[j].PortfolioID
		}
		return results[i].Ticker < results[j].Ticker
	})

	return results, nil
}
