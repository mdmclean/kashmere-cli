// internal/trades/trades.go
package trades

import (
	"math"
	"sort"
	"strings"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
)

// TradeRecommendation is a ranked BUY or SELL recommendation for a single asset.
type TradeRecommendation struct {
	PortfolioID   string  `json:"portfolioId"`
	PortfolioName string  `json:"portfolioName"`
	Ticker        string  `json:"ticker"`
	AssetName     string  `json:"assetName"`
	Direction     string  `json:"direction"`    // "BUY" | "SELL"
	CurrentPct    float64 `json:"currentPct"`   // current weight in portfolio (%)
	TargetPct     float64 `json:"targetPct"`    // effective target: (asset.TargetPercentage/100) × classAllocation
	DriftPct      float64 `json:"driftPct"`     // currentPct - targetPct (signed)
	DriftAmount   float64 `json:"driftAmount"`  // transaction amount in display currency (may be < |full drift| for partial buys)
	IsPartialBuy  bool    `json:"isPartialBuy"` // true when BUY amount is capped by available portfolio cash
	Currency      string  `json:"currency"`     // display currency
}

// SuppressedBuy is an underweight asset that cannot be acted on due to
// insufficient cash. Reason is one of: "no_cash", "below_min_transaction",
// "cash_needs_replenishing".
type SuppressedBuy struct {
	PortfolioID   string  `json:"portfolioId"`
	PortfolioName string  `json:"portfolioName"`
	Ticker        string  `json:"ticker"`
	AssetName     string  `json:"assetName"`
	DriftPct      float64 `json:"driftPct"`
	DriftAmount   float64 `json:"driftAmount"`
	Reason        string  `json:"reason"`
	Currency      string  `json:"currency"`
}

// ComputeResult holds actionable trade recommendations and the buys that were
// suppressed due to insufficient cash.
type ComputeResult struct {
	Trades         []TradeRecommendation `json:"trades"`
	SuppressedBuys []SuppressedBuy       `json:"suppressedBuys"`
}

// resolveEffectiveAssetTarget mirrors resolveEffectiveAssetTarget in trade-calculations.ts.
//
// asset.TargetPercentage is a within-category percentage, not a portfolio percentage.
// This function converts it to an effective portfolio-level target:
//
//	effectiveTarget = (asset.TargetPercentage / 100) × classAllocation.Percentage
//
// Returns nil when the asset has no TargetPercentage (asset should be skipped).
// Returns a pointer to 0.0 when the asset has a TargetPercentage but no matching
// category in the portfolio's Allocations.
func resolveEffectiveAssetTarget(asset api.Asset, allocations []api.Allocation) *float64 {
	if asset.TargetPercentage == nil {
		return nil
	}
	for _, alloc := range allocations {
		if alloc.Category == asset.Category {
			result := (*asset.TargetPercentage / 100.0) * alloc.Percentage
			return &result
		}
	}
	zero := 0.0
	return &zero
}

// Compute returns a flat, drift-ranked list of BUY/SELL recommendations across
// the given portfolios.
//
// Portfolios with no targetPercentage on any asset are skipped entirely.
// CASH assets are never included (they are not tradeable instruments).
// Each asset's target is resolved via resolveEffectiveAssetTarget, which scales
// the asset-level within-category targetPercentage by the portfolio's category allocation.
//
// BUY recommendations are subject to cash availability:
//   - If the portfolio's cash is below its own target weight, all BUYs are suppressed.
//   - BUYs are funded in priority order (highest |drift| first) against available cash.
//   - A BUY is included only if there is at least minTransactionAmount of cash remaining.
//   - A BUY may be partially filled (IsPartialBuy=true) if cash covers it only in part.
func Compute(portfolios []api.Portfolio, c *api.Client) (ComputeResult, error) {
	displayCurrency := "CAD"
	var settings api.Settings
	if err := c.Get("/settings", &settings); err == nil && settings.DisplayCurrency != "" {
		displayCurrency = settings.DisplayCurrency
	}

	priceMap := portfolio.FetchPrices(portfolios, c)

	var results []TradeRecommendation
	var suppressed []SuppressedBuy

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

		// Compute minimum transaction amount in display currency (once, outside the loop).
		minAmtDisplay := 0.0
		if p.MinTransactionAmount != nil && *p.MinTransactionAmount > 0 {
			minAmtDisplay = *p.MinTransactionAmount
			if p.MinTransactionCurrency != "" && p.MinTransactionCurrency != displayCurrency {
				minAmtDisplay *= portfolio.FxRate(p.MinTransactionCurrency, displayCurrency, priceMap)
			}
		}

		// Compute available portfolio cash and its target weight.
		// This mirrors the portfolioCashDisplay / cashTargetWeight logic in trade-calculations.ts.
		portfolioCash := 0.0
		cashTargetWeight := 0.0
		for _, av := range assetVals {
			if strings.EqualFold(av.asset.Ticker, "CASH") {
				portfolioCash += av.value
			}
		}
		for _, a := range p.Assets {
			if strings.EqualFold(a.Ticker, "CASH") {
				if t := resolveEffectiveAssetTarget(a, p.Allocations); t != nil {
					cashTargetWeight += *t
				}
			}
		}
		currentCashWeight := portfolioCash / totalValue * 100
		// If cash is below its own target weight, deploying it into other assets
		// would push cash further below target — suppress all BUYs.
		cashBelowTarget := currentCashWeight < cashTargetWeight

		// Build per-portfolio recommendations (pre-cash-filter).
		var portfolioRecs []TradeRecommendation

		for _, av := range assetVals {
			// CASH is never a tradeable asset.
			if strings.EqualFold(av.asset.Ticker, "CASH") {
				continue
			}

			// Resolve effective portfolio-level target from the within-category percentage.
			effectiveTarget := resolveEffectiveAssetTarget(av.asset, p.Allocations)
			if effectiveTarget == nil {
				continue // no targetPercentage set — skip
			}

			currentPct := av.value / totalValue * 100
			targetPct := *effectiveTarget
			driftPct := currentPct - targetPct
			driftAmount := math.Abs(driftPct / 100 * totalValue)

			if driftAmount < 1 {
				continue // effectively on target
			}

			if minAmtDisplay > 0 && driftAmount < minAmtDisplay {
				continue
			}

			direction := "BUY"
			if driftPct > 0 {
				direction = "SELL"
			}

			portfolioRecs = append(portfolioRecs, TradeRecommendation{
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

		// Separate sells from buys; sells are always included.
		for _, rec := range portfolioRecs {
			if rec.Direction == "SELL" {
				results = append(results, rec)
			}
		}

		var buys []TradeRecommendation
		for _, rec := range portfolioRecs {
			if rec.Direction == "BUY" {
				buys = append(buys, rec)
			}
		}

		if len(buys) == 0 {
			continue
		}

		// Cash is below its own target allocation — don't deploy cash into other assets.
		if cashBelowTarget {
			for _, b := range buys {
				suppressed = append(suppressed, SuppressedBuy{
					PortfolioID:   b.PortfolioID,
					PortfolioName: b.PortfolioName,
					Ticker:        b.Ticker,
					AssetName:     b.AssetName,
					DriftPct:      b.DriftPct,
					DriftAmount:   b.DriftAmount,
					Reason:        "cash_needs_replenishing",
					Currency:      b.Currency,
				})
			}
			continue
		}

		// Not enough cash to meet minimum transaction threshold — skip all buys.
		if portfolioCash <= 0 {
			for _, b := range buys {
				suppressed = append(suppressed, SuppressedBuy{
					PortfolioID:   b.PortfolioID,
					PortfolioName: b.PortfolioName,
					Ticker:        b.Ticker,
					AssetName:     b.AssetName,
					DriftPct:      b.DriftPct,
					DriftAmount:   b.DriftAmount,
					Reason:        "no_cash",
					Currency:      b.Currency,
				})
			}
			continue
		}
		if minAmtDisplay > 0 && portfolioCash < minAmtDisplay {
			for _, b := range buys {
				suppressed = append(suppressed, SuppressedBuy{
					PortfolioID:   b.PortfolioID,
					PortfolioName: b.PortfolioName,
					Ticker:        b.Ticker,
					AssetName:     b.AssetName,
					DriftPct:      b.DriftPct,
					DriftAmount:   b.DriftAmount,
					Reason:        "below_min_transaction",
					Currency:      b.Currency,
				})
			}
			continue
		}

		// Fund BUYs in priority order (highest |drift| first) against available cash.
		sort.SliceStable(buys, func(i, j int) bool {
			return math.Abs(buys[i].DriftPct) > math.Abs(buys[j].DriftPct)
		})

		cashRemaining := portfolioCash
		for _, rec := range buys {
			if cashRemaining >= rec.DriftAmount {
				cashRemaining -= rec.DriftAmount
				results = append(results, rec)
			} else if cashRemaining > 0 && (minAmtDisplay == 0 || cashRemaining >= minAmtDisplay) {
				// Partial buy: cap DriftAmount to remaining cash.
				rec.DriftAmount = cashRemaining
				rec.IsPartialBuy = true
				results = append(results, rec)
				cashRemaining = 0
			} else {
				// No usable cash left for this buy.
				reason := "no_cash"
				if cashRemaining > 0 {
					reason = "below_min_transaction"
				}
				suppressed = append(suppressed, SuppressedBuy{
					PortfolioID:   rec.PortfolioID,
					PortfolioName: rec.PortfolioName,
					Ticker:        rec.Ticker,
					AssetName:     rec.AssetName,
					DriftPct:      rec.DriftPct,
					DriftAmount:   rec.DriftAmount,
					Reason:        reason,
					Currency:      rec.Currency,
				})
			}
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

	if suppressed == nil {
		suppressed = []SuppressedBuy{}
	}
	if results == nil {
		results = []TradeRecommendation{}
	}

	return ComputeResult{Trades: results, SuppressedBuys: suppressed}, nil
}
