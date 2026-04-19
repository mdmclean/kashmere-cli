// internal/performance/performance.go
package performance

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mdmclean/kashmere-cli/internal/api"
)

// Result holds the computed performance metrics for a portfolio over a date range.
type Result struct {
	PortfolioID      string  `json:"portfolioId"`
	PortfolioName    string  `json:"portfolioName"`
	StartDate        string  `json:"startDate"`
	EndDate          string  `json:"endDate"`
	StartValue       float64 `json:"startValue"`
	EndValue         float64 `json:"endValue"`
	DayCount         int     `json:"dayCount"`
	SimpleReturn     float64 `json:"simpleReturn"`
	TWR              float64 `json:"twr"`
	CAGR             float64 `json:"cagr"`
	CashFlowsFound   int     `json:"cashFlowsFound"`
	CashFlowAdjusted bool    `json:"cashFlowAdjusted"`
}

// Compute calculates portfolio performance over the given date range.
//
// startDate and endDate are YYYY-MM-DD strings; empty string means use the
// earliest/latest non-zero snapshot, respectively.
//
// Algorithm: Modified Dietz TWR across consecutive snapshot pairs, chained.
// Annualised as CAGR = (1+TWR)^(365.25/dayCount) - 1.
func Compute(
	portfolioID string,
	portfolioName string,
	snapshots []api.PortfolioSnapshot,
	cashflows []api.CashFlow,
	startDate string,
	endDate string,
) (Result, error) {
	// Filter to non-zero snapshots (pre-fix snapshots have TotalValue == 0)
	nonZero := make([]api.PortfolioSnapshot, 0, len(snapshots))
	for _, s := range snapshots {
		if s.TotalValue > 0 {
			nonZero = append(nonZero, s)
		}
	}
	if len(nonZero) == 0 {
		return Result{}, fmt.Errorf("no non-zero snapshots found for portfolio %s", portfolioID)
	}

	// Sort snapshots by date ascending
	sort.Slice(nonZero, func(i, j int) bool {
		return snapshotDate(nonZero[i]) < snapshotDate(nonZero[j])
	})

	// Boundary matching:
	// start = earliest snapshot on or after startDate
	// end   = latest snapshot on or before endDate
	start, end, err := findBoundaries(nonZero, startDate, endDate)
	if err != nil {
		return Result{}, err
	}

	// Collect snapshots in [start, end] range
	inRange := snapshotsInRange(nonZero, snapshotDate(start), snapshotDate(end))
	if len(inRange) < 2 {
		return Result{}, fmt.Errorf("need at least 2 snapshots in range; found %d", len(inRange))
	}

	// Filter cashflows to this portfolio and within (start, end]
	// Per Modified Dietz: cashflows on the start date don't count (already reflected in start value);
	// cashflows on or before the end date and after the start date do.
	pfCashflows := filterCashflows(cashflows, portfolioID, snapshotDate(start), snapshotDate(end))

	// Compute TWR by chaining sub-period returns
	twr, err := computeTWR(inRange, pfCashflows)
	if err != nil {
		return Result{}, err
	}

	// Day count
	startTime, _ := time.Parse("2006-01-02", snapshotDate(start))
	endTime, _ := time.Parse("2006-01-02", snapshotDate(end))
	dayCount := int(endTime.Sub(startTime).Hours() / 24)

	// Simple return (no cashflow adjustment)
	simpleReturn := 0.0
	if start.TotalValue != 0 {
		simpleReturn = (end.TotalValue - start.TotalValue) / start.TotalValue
	}

	cashFlowAdjusted := len(pfCashflows) > 0

	// If no cashflows, TWR == simple return
	if !cashFlowAdjusted {
		twr = simpleReturn
	}

	// CAGR
	cagr := 0.0
	if dayCount > 0 {
		cagr = math.Pow(1+twr, 365.25/float64(dayCount)) - 1
	}

	return Result{
		PortfolioID:      portfolioID,
		PortfolioName:    portfolioName,
		StartDate:        snapshotDate(start),
		EndDate:          snapshotDate(end),
		StartValue:       start.TotalValue,
		EndValue:         end.TotalValue,
		DayCount:         dayCount,
		SimpleReturn:     simpleReturn,
		TWR:              twr,
		CAGR:             cagr,
		CashFlowsFound:   len(pfCashflows),
		CashFlowAdjusted: cashFlowAdjusted,
	}, nil
}

// snapshotDate extracts YYYY-MM-DD from a snapshot's Timestamp field.
func snapshotDate(s api.PortfolioSnapshot) string {
	if len(s.Timestamp) >= 10 {
		return s.Timestamp[:10]
	}
	return s.Timestamp
}

// findBoundaries finds the start and end snapshots based on date strings.
func findBoundaries(sorted []api.PortfolioSnapshot, startDate, endDate string) (api.PortfolioSnapshot, api.PortfolioSnapshot, error) {
	first := sorted[0]
	last := sorted[len(sorted)-1]

	if startDate == "" {
		startDate = snapshotDate(first)
	}
	if endDate == "" {
		endDate = snapshotDate(last)
	}

	// start = earliest on or after startDate
	var start *api.PortfolioSnapshot
	for i := range sorted {
		d := snapshotDate(sorted[i])
		if d >= startDate {
			start = &sorted[i]
			break
		}
	}
	if start == nil {
		return api.PortfolioSnapshot{}, api.PortfolioSnapshot{},
			fmt.Errorf("no snapshot found on or after %s", startDate)
	}

	// end = latest on or before endDate
	var end *api.PortfolioSnapshot
	for i := len(sorted) - 1; i >= 0; i-- {
		d := snapshotDate(sorted[i])
		if d <= endDate {
			end = &sorted[i]
			break
		}
	}
	if end == nil {
		return api.PortfolioSnapshot{}, api.PortfolioSnapshot{},
			fmt.Errorf("no snapshot found on or before %s", endDate)
	}

	if snapshotDate(*start) >= snapshotDate(*end) && snapshotDate(*start) != snapshotDate(*end) {
		return api.PortfolioSnapshot{}, api.PortfolioSnapshot{},
			fmt.Errorf("start date %s is after end date %s", snapshotDate(*start), snapshotDate(*end))
	}

	return *start, *end, nil
}

// snapshotsInRange returns all snapshots with dates in [startDate, endDate].
func snapshotsInRange(sorted []api.PortfolioSnapshot, startDate, endDate string) []api.PortfolioSnapshot {
	var result []api.PortfolioSnapshot
	for _, s := range sorted {
		d := snapshotDate(s)
		if d >= startDate && d <= endDate {
			result = append(result, s)
		}
	}
	return result
}

// filterCashflows returns cashflows for the given portfolio with dates strictly
// after startDate and on or before endDate.
func filterCashflows(cashflows []api.CashFlow, portfolioID, startDate, endDate string) []api.CashFlow {
	var result []api.CashFlow
	for _, cf := range cashflows {
		if cf.PortfolioID != portfolioID {
			continue
		}
		d := cf.Date
		if len(d) > 10 {
			d = d[:10]
		}
		if d > startDate && d <= endDate {
			result = append(result, cf)
		}
	}
	return result
}

// computeTWR chains Modified Dietz sub-period returns across consecutive snapshot pairs.
func computeTWR(snapshots []api.PortfolioSnapshot, cashflows []api.CashFlow) (float64, error) {
	product := 1.0
	for i := 0; i < len(snapshots)-1; i++ {
		s0 := snapshots[i]
		s1 := snapshots[i+1]
		d0 := snapshotDate(s0)
		d1 := snapshotDate(s1)

		// Net cashflow in (d0, d1]: deposits positive, withdrawals negative
		cf := netCashflow(cashflows, d0, d1)

		// Modified Dietz: r = (V1 - V0 - CF) / (V0 + CF/2)
		denominator := s0.TotalValue + cf/2
		if denominator == 0 {
			return 0, fmt.Errorf("zero denominator in sub-period %s → %s (start value: %.2f, CF: %.2f)",
				d0, d1, s0.TotalValue, cf)
		}
		r := (s1.TotalValue - s0.TotalValue - cf) / denominator
		product *= (1 + r)
	}
	return product - 1, nil
}

// netCashflow sums cashflows with dates in (afterDate, onOrBeforeDate].
func netCashflow(cashflows []api.CashFlow, afterDate, onOrBeforeDate string) float64 {
	net := 0.0
	for _, cf := range cashflows {
		d := cf.Date
		if len(d) > 10 {
			d = d[:10]
		}
		if d > afterDate && d <= onOrBeforeDate {
			if cf.Type == "deposit" {
				net += cf.Amount
			} else {
				net -= cf.Amount
			}
		}
	}
	return net
}
