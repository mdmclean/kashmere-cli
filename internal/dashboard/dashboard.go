// internal/dashboard/dashboard.go
package dashboard

import (
	"math"

	"github.com/mdmclean/kashmere-cli/internal/api"
)

// Data is the aggregated dashboard view.
type Data struct {
	TotalValue     float64          `json:"totalValue"`
	PortfolioCount int              `json:"portfolioCount"`
	WeightedAllocs []WeightedAlloc  `json:"weightedAllocations"`
	GoalSummaries  []GoalSummary    `json:"goalSummaries"`
	NetWorth       *NetWorthSummary `json:"netWorth,omitempty"`
}

// WeightedAlloc is a weighted asset class allocation across all portfolios.
type WeightedAlloc struct {
	Category       string  `json:"category"`
	Percentage     float64 `json:"percentage"`
	TotalDollars   float64 `json:"totalDollars"`
	PortfolioCount int     `json:"portfolioCount"`
}

// GoalSummary is a per-goal aggregation of portfolio values.
type GoalSummary struct {
	GoalID          string          `json:"goalId"`
	GoalName        string          `json:"goalName"`
	TotalValue      float64         `json:"totalValue"`
	PortfolioCount  int             `json:"portfolioCount"`
	Target          *api.GoalTarget `json:"target,omitempty"`
	TargetValue     *float64        `json:"targetValue,omitempty"`
	DeltaValue      *float64        `json:"deltaValue,omitempty"`
	DeltaPercentage *float64        `json:"deltaPercentage,omitempty"`
}

// NetWorthSummary is a high-level assets vs. debt summary.
type NetWorthSummary struct {
	TotalAssets    float64 `json:"totalAssets"`
	TotalDebt      float64 `json:"totalDebt"`
	NetWorth       float64 `json:"netWorth"`
	AssetBreakdown struct {
		PortfolioValue float64 `json:"portfolioValue"`
	} `json:"assetBreakdown"`
	DebtBreakdown struct {
		MortgageDebt float64 `json:"mortgageDebt"`
	} `json:"debtBreakdown"`
}

// Compute builds a dashboard from decrypted portfolio, goal, and mortgage data.
func Compute(portfolios []api.Portfolio, goals []api.Goal, mortgages []api.Mortgage) Data {
	totalValue := 0.0
	for _, p := range portfolios {
		totalValue += p.TotalValue
	}

	type allocAgg struct {
		weightedPctSum float64
		totalDollars   float64
		count          int
	}
	allocMap := map[string]*allocAgg{}

	for _, p := range portfolios {
		if len(p.Allocations) == 0 {
			continue
		}
		sum := 0.0
		for _, a := range p.Allocations {
			sum += a.Percentage
		}
		if math.Abs(sum-100) > 0.01 {
			continue
		}
		for _, a := range p.Allocations {
			if _, ok := allocMap[a.Category]; !ok {
				allocMap[a.Category] = &allocAgg{}
			}
			allocMap[a.Category].weightedPctSum += p.TotalValue * a.Percentage
			allocMap[a.Category].totalDollars += p.TotalValue * a.Percentage / 100
			allocMap[a.Category].count++
		}
	}

	weightedAllocs := make([]WeightedAlloc, 0, len(allocMap))
	for cat, agg := range allocMap {
		pct := 0.0
		if totalValue > 0 {
			pct = agg.weightedPctSum / totalValue
		}
		weightedAllocs = append(weightedAllocs, WeightedAlloc{
			Category:       cat,
			Percentage:     pct,
			TotalDollars:   agg.totalDollars,
			PortfolioCount: agg.count,
		})
	}

	goalMap := map[string]api.Goal{}
	for _, g := range goals {
		goalMap[g.ID] = g
	}
	goalAgg := map[string]*struct {
		totalValue     float64
		portfolioCount int
	}{}
	for _, p := range portfolios {
		gid := p.GoalID
		if gid == "" {
			gid = "__unassigned__"
		}
		if _, ok := goalAgg[gid]; !ok {
			goalAgg[gid] = &struct {
				totalValue     float64
				portfolioCount int
			}{}
		}
		goalAgg[gid].totalValue += p.TotalValue
		goalAgg[gid].portfolioCount++
	}

	goalSummaries := make([]GoalSummary, 0, len(goalAgg))
	for gid, agg := range goalAgg {
		gs := GoalSummary{
			GoalID:         gid,
			TotalValue:     agg.totalValue,
			PortfolioCount: agg.portfolioCount,
		}
		if gid == "__unassigned__" {
			gs.GoalName = "Unassigned"
		} else if goal, ok := goalMap[gid]; ok {
			gs.GoalName = goal.Name
			gs.Target = goal.Target
			if goal.Target != nil {
				var targetValue float64
				if goal.Target.Type == "fixed" {
					targetValue = goal.Target.Value
				} else {
					targetValue = goal.Target.Value / 100 * totalValue
				}
				delta := agg.totalValue - targetValue
				gs.TargetValue = &targetValue
				gs.DeltaValue = &delta
				if targetValue > 0 {
					deltaPct := delta / targetValue * 100
					gs.DeltaPercentage = &deltaPct
				}
			}
		} else {
			gs.GoalName = "Unknown Goal"
		}
		goalSummaries = append(goalSummaries, gs)
	}

	d := Data{
		TotalValue:     totalValue,
		PortfolioCount: len(portfolios),
		WeightedAllocs: weightedAllocs,
		GoalSummaries:  goalSummaries,
	}

	if len(mortgages) > 0 {
		totalDebt := 0.0
		for _, m := range mortgages {
			totalDebt += m.CurrentBalance
		}
		nw := &NetWorthSummary{
			TotalAssets: totalValue,
			TotalDebt:   totalDebt,
			NetWorth:    totalValue - totalDebt,
		}
		nw.AssetBreakdown.PortfolioValue = totalValue
		nw.DebtBreakdown.MortgageDebt = totalDebt
		d.NetWorth = nw
	}

	return d
}
