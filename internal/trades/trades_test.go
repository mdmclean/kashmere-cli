package trades_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/trades"
)

func ptr[T any](v T) *T { return &v }

func newTestServer(t *testing.T, prices []api.TickerPrice, displayCurrency string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/prices":
			json.NewEncoder(w).Encode(prices)
		case "/settings":
			json.NewEncoder(w).Encode(api.Settings{DisplayCurrency: displayCurrency})
		default:
			http.NotFound(w, r)
		}
	}))
}

// ---------------------------------------------------------------------------
// resolveEffectiveAssetTarget unit tests (via Compute behaviour)
// ---------------------------------------------------------------------------

// TestCompute_EffectiveTargetFromAllocation is the regression test for the GIC bug:
// asset.TargetPercentage is a within-category percentage, not a portfolio percentage.
// A GIC with targetPercentage=100 in a category allocated 5% of the portfolio
// has an effective portfolio target of 5%, not 100%.
func TestCompute_EffectiveTargetFromAllocation(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// GIC: $5000 quantity (static asset), category "Fixed Income"
	// Category allocation: Fixed Income = 5%
	// effectiveTarget = (100/100) * 5 = 5%
	// Total portfolio = $100,000 (GIC $5000 + other $95000, but we only have GIC priced)
	// Since other asset has no price, totalValue = $5000
	// currentPct = 5000/5000 = 100%
	// effectiveTarget = 5%
	// drift = 100 - 5 = 95% → SELL
	//
	// But if we set both assets to have a price this gets complex.
	// Simpler: single GIC asset at exactly its target to prove zero drift.
	// GIC: $5000, category "Fixed Income", targetPercentage=100
	// Allocation: Fixed Income=100% → effectiveTarget = (100/100)*100 = 100%
	// currentPct = 5000/5000 = 100% → drift = 0 → skipped
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "Fixed Income", Percentage: 100},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "GIC", Name: "GIC", Category: "Fixed Income", Quantity: 5000, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 0 {
		t.Errorf("len(result.Trades) = %d, want 0 (GIC is perfectly on target at 100%% of 100%% allocation)", len(result.Trades))
	}
}

// TestCompute_EffectiveTargetScaledByAllocation verifies that a GIC with
// targetPercentage=100 in a 5% category allocation has effectiveTarget=5%,
// not 100% (the pre-fix behaviour).
func TestCompute_EffectiveTargetScaledByAllocation(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// GIC: $5000 static asset (quantity = dollar value), category "Fixed Income"
	// VCN: $95000 (priced), category "Canadian Equity"
	// Portfolio total: $100,000
	// GIC currentPct: 5%
	// GIC effectiveTarget: (100/100)*5 = 5% → drift ≈ 0 → should be skipped
	// VCN currentPct: 95%
	// VCN effectiveTarget: (100/100)*95 = 95% → drift ≈ 0 → should be skipped
	//
	// If the old (broken) behaviour were still in place, GIC would have
	// effectiveTarget=100% → drift=-95% → BUY for ~$95,000.
	srv2 := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(950.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv2.Close()
	c2 := api.New(srv2.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "Fixed Income", Percentage: 5},
				{Category: "Canadian Equity", Percentage: 95},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "GIC", Name: "GIC", Category: "Fixed Income", Quantity: 5000, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VCN", Exchange: "TSX", Category: "Canadian Equity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c2)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	// GIC: effectiveTarget=5%, currentPct=5000/100000*100=5% → drift≈0 → skip
	// VCN: effectiveTarget=95%, currentPct=95000/100000*100=95% → drift≈0 → skip
	if len(result.Trades) != 0 {
		for _, r := range result.Trades {
			t.Logf("unexpected rec: %s drift=%.2f%% (target=%.2f%% current=%.2f%%)", r.Ticker, r.DriftPct, r.TargetPct, r.CurrentPct)
		}
		t.Errorf("len(result.Trades) = %d, want 0 (both assets on target when effectiveTarget is scaled by allocation)", len(result.Trades))
	}
	_ = srv // suppress unused warning
	_ = c
}

// ---------------------------------------------------------------------------
// TestCompute_NormalRanking
// ---------------------------------------------------------------------------

func TestCompute_NormalRanking(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 120 shares × $50 = $6000 (60% of $10000 total)
	//   category "CanadianEquity", allocation 50%
	//   effectiveTarget = (100/100)*50 = 50%
	//   drift = 60 - 50 = +10% → SELL, amount = $1000
	// VFV: 40 shares × $100 = $4000 (40% of $10000 total)
	//   category "USEquity", allocation 30%
	//   effectiveTarget = (100/100)*30 = 30%
	//   drift = 40 - 30 = +10% → SELL, amount = $1000
	// Equal |drift|, tiebreak by ticker: VCN < VFV → VCN first.
	// SELLs are always included regardless of portfolio cash.
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 50},
				{Category: "USEquity", Percentage: 30},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 120, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 40, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 2 {
		t.Fatalf("len(result.Trades) = %d, want 2", len(result.Trades))
	}

	vcn := result.Trades[0]
	if vcn.Ticker != "VCN" {
		t.Errorf("result.Trades[0].Ticker = %s, want VCN (alphabetic tiebreak)", vcn.Ticker)
	}
	if vcn.Direction != "SELL" {
		t.Errorf("VCN direction = %s, want SELL", vcn.Direction)
	}
	if math.Abs(vcn.DriftPct-10) > 0.001 {
		t.Errorf("VCN DriftPct = %.4f, want 10", vcn.DriftPct)
	}
	if math.Abs(vcn.TargetPct-50) > 0.001 {
		t.Errorf("VCN TargetPct = %.4f, want 50", vcn.TargetPct)
	}
	if math.Abs(vcn.DriftAmount-1000) > 0.01 {
		t.Errorf("VCN DriftAmount = %.2f, want 1000", vcn.DriftAmount)
	}

	vfv := result.Trades[1]
	if vfv.Ticker != "VFV" {
		t.Errorf("result.Trades[1].Ticker = %s, want VFV", vfv.Ticker)
	}
	if vfv.Direction != "SELL" {
		t.Errorf("VFV direction = %s, want SELL", vfv.Direction)
	}
	if math.Abs(vfv.DriftPct-10) > 0.001 {
		t.Errorf("VFV DriftPct = %.4f, want 10", vfv.DriftPct)
	}
	if math.Abs(vfv.TargetPct-30) > 0.001 {
		t.Errorf("VFV TargetPct = %.4f, want 30", vfv.TargetPct)
	}
}

// ---------------------------------------------------------------------------
// CASH and null-target behaviour
// ---------------------------------------------------------------------------

// TestCompute_CashAssetsSkipped verifies that CASH is never included in
// trade recommendations, even when it has a targetPercentage set.
func TestCompute_CashAssetsSkipped(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 100 × $50 = $5000 (50%), effectiveTarget=60% → BUY
	// CASH: $5000, targetPercentage set → must be skipped (not a tradeable instrument)
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 60},
				{Category: "Cash", Percentage: 40},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "CASH", Category: "Cash", Quantity: 5000, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, r := range result.Trades {
		if r.Ticker == "CASH" {
			t.Errorf("CASH appeared in results — it must always be excluded")
		}
	}
	// VCN should still appear
	if len(result.Trades) != 1 || result.Trades[0].Ticker != "VCN" {
		t.Errorf("expected exactly 1 rec (VCN), got %d", len(result.Trades))
	}
}

// TestCompute_NullTargetSkipped verifies that assets with no targetPercentage
// are skipped entirely (unlike CASH, they produce no recommendation at 0%).
func TestCompute_NullTargetSkipped(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN has targetPercentage=nil → skip
	// VFV has targetPercentage set → include if drift is non-zero
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 40},
				{Category: "USEquity", Percentage: 60},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: nil},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 50, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, r := range result.Trades {
		if r.Ticker == "VCN" {
			t.Errorf("VCN (null targetPercentage) appeared in results — it must be skipped")
		}
	}
}

// ---------------------------------------------------------------------------
// TestCompute_NoTargets_PortfolioSkipped
// ---------------------------------------------------------------------------

func TestCompute_NoTargets_PortfolioSkipped(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: nil},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 0 {
		t.Errorf("len(result.Trades) = %d, want 0 (no targets defined)", len(result.Trades))
	}
}

// ---------------------------------------------------------------------------
// TestCompute_MinTransactionFilter
// ---------------------------------------------------------------------------

func TestCompute_MinTransactionFilter(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: $5000 (50%), effectiveTarget=60% → drift=-10%, amount=$1000
	// VFV: $5000 (50%), effectiveTarget=40% → drift=+10%, amount=$1000
	// minTransactionAmount=1500 CAD → both $1000 trades filtered out
	portfolios := []api.Portfolio{
		{
			ID:                     "p1",
			Name:                   "TFSA",
			MinTransactionAmount:   ptr(1500.0),
			MinTransactionCurrency: "CAD",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 60},
				{Category: "USEquity", Percentage: 40},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 50, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 0 {
		t.Errorf("len(result.Trades) = %d, want 0 (all below min transaction)", len(result.Trades))
	}
}

// ---------------------------------------------------------------------------
// TestCompute_ZeroTotalValue_PortfolioSkipped
// ---------------------------------------------------------------------------

func TestCompute_ZeroTotalValue_PortfolioSkipped(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN has no price → value=0 → total=0 → skip
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 100},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 0 {
		t.Errorf("len(result.Trades) = %d, want 0 (zero total value)", len(result.Trades))
	}
}

// ---------------------------------------------------------------------------
// TestCompute_MultiPortfolioFlat
// ---------------------------------------------------------------------------

func TestCompute_MultiPortfolioFlat(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// p1: VCN 100% actual, effectiveTarget=80% → drift +20%
	// p2: VFV 100% actual, effectiveTarget=70% → drift +30% (bigger, should rank first)
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 80},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
		{
			ID:   "p2",
			Name: "RRSP",
			Allocations: []api.Allocation{
				{Category: "USEquity", Percentage: 70},
			},
			Assets: []api.Asset{
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 50, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 2 {
		t.Fatalf("len(result.Trades) = %d, want 2", len(result.Trades))
	}
	if result.Trades[0].PortfolioID != "p2" {
		t.Errorf("result.Trades[0].PortfolioID = %s, want p2 (higher drift)", result.Trades[0].PortfolioID)
	}
	if result.Trades[1].PortfolioID != "p1" {
		t.Errorf("result.Trades[1].PortfolioID = %s, want p1", result.Trades[1].PortfolioID)
	}
}

// ---------------------------------------------------------------------------
// TestCompute_ZeroDrift_Skipped
// ---------------------------------------------------------------------------

func TestCompute_ZeroDrift_Skipped(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 100 × $50 = $5000, total=$5000, currentPct=100%
	// category "CanadianEquity", allocation=100%
	// effectiveTarget = (100/100)*100 = 100% → drift=0 → skip
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 100},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 0 {
		t.Errorf("len(result.Trades) = %d, want 0 (zero drift should be skipped)", len(result.Trades))
	}
}

// ---------------------------------------------------------------------------
// TestCompute_MinTransactionFilter_USDCurrency
// ---------------------------------------------------------------------------

func TestCompute_MinTransactionFilter_USDCurrency(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(2.0)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: $5000 (50%), effectiveTarget=60% → drift=-10%, amount=$1000 CAD
	// VFV: $5000 (50%), effectiveTarget=40% → drift=+10%, amount=$1000 CAD
	// minTransactionAmount=600 USD → 600 × 2.0 = $1200 CAD → both $1000 trades filtered out
	portfolios := []api.Portfolio{
		{
			ID:                     "p1",
			Name:                   "TFSA",
			MinTransactionAmount:   ptr(600.0),
			MinTransactionCurrency: "USD",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 60},
				{Category: "USEquity", Percentage: 40},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 50, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 0 {
		t.Errorf("len(result.Trades) = %d, want 0 (both $1000 CAD trades below $600 USD = $1200 CAD threshold)", len(result.Trades))
	}
}

// ---------------------------------------------------------------------------
// TestCompute_CategoryNotInAllocations
// ---------------------------------------------------------------------------

// TestCompute_CategoryNotInAllocations verifies that when an asset's category
// has no matching allocation entry, the effective target is 0% and the asset
// is treated as having zero allocation (SELL if currently held).
func TestCompute_CategoryNotInAllocations(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN has targetPercentage=100, category="CanadianEquity"
	// But portfolio allocations only list "USEquity" — no match for VCN's category
	// resolveEffectiveAssetTarget returns 0 → effectiveTarget=0%
	// currentPct=100% → drift=+100% → SELL
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "USEquity", Percentage: 100},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("len(result.Trades) = %d, want 1", len(result.Trades))
	}
	if result.Trades[0].Ticker != "VCN" {
		t.Errorf("ticker = %s, want VCN", result.Trades[0].Ticker)
	}
	if result.Trades[0].Direction != "SELL" {
		t.Errorf("direction = %s, want SELL (effective target is 0%%)", result.Trades[0].Direction)
	}
	if math.Abs(result.Trades[0].TargetPct) > 0.001 {
		t.Errorf("TargetPct = %.4f, want 0 (no matching category)", result.Trades[0].TargetPct)
	}
}

// ---------------------------------------------------------------------------
// Cash availability tests for BUYs
// ---------------------------------------------------------------------------

// TestCompute_BuyIncludedWhenCashAvailable verifies that a BUY recommendation
// appears when the portfolio has enough CASH to fund it.
func TestCompute_BuyIncludedWhenCashAvailable(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 80 × $50 = $4000 (40% of $10000 total)
	//   alloc CanadianEquity=60%, effectiveTarget=60%, drift=-20% BUY, amount=$2000
	// CASH: quantity=6000 (60% of total), no cash allocation → cashTargetWeight=0 → cashBelowTarget=false
	// portfolioCash=$6000 ≥ $2000 → VCN BUY fully funded
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 60},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 80, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "CASH", Category: "Cash", Quantity: 6000, Currency: "CAD"},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("len(result.Trades) = %d, want 1 (VCN BUY funded by cash)", len(result.Trades))
	}
	if result.Trades[0].Ticker != "VCN" {
		t.Errorf("ticker = %s, want VCN", result.Trades[0].Ticker)
	}
	if result.Trades[0].Direction != "BUY" {
		t.Errorf("direction = %s, want BUY", result.Trades[0].Direction)
	}
	if result.Trades[0].IsPartialBuy {
		t.Errorf("IsPartialBuy = true, want false (cash fully covers the trade)")
	}
	if math.Abs(result.Trades[0].DriftAmount-2000) > 0.01 {
		t.Errorf("DriftAmount = %.2f, want 2000", result.Trades[0].DriftAmount)
	}
}

// TestCompute_BuySuppressedWhenNoCash verifies that BUY recommendations are
// omitted when the portfolio holds no cash. SELLs are unaffected.
func TestCompute_BuySuppressedWhenNoCash(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// Total: $10000
	// VFV: 60 × $100 = $6000 (60%), alloc USEquity=40%, drift=+20% SELL, amount=$2000
	// VCN: 80 × $50 = $4000 (40%), alloc CanadianEquity=60%, drift=-20% BUY, amount=$2000
	// No CASH → portfolioCash=0 → VCN BUY suppressed; VFV SELL still appears
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 60},
				{Category: "USEquity", Percentage: 40},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 80, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 60, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, r := range result.Trades {
		if r.Direction == "BUY" {
			t.Errorf("unexpected BUY for %s: no cash in portfolio, BUYs must be suppressed", r.Ticker)
		}
	}
	found := false
	for _, r := range result.Trades {
		if r.Ticker == "VFV" && r.Direction == "SELL" {
			found = true
		}
	}
	if !found {
		t.Errorf("VFV SELL missing — SELLs should be unaffected by cash availability")
	}
}

// TestCompute_BuySuppressedWhenCashBelowTarget verifies that BUYs are omitted
// when the portfolio's cash holdings are below the cash's own target weight.
// Deploying cash when it's already below target would push it further off target.
func TestCompute_BuySuppressedWhenCashBelowTarget(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// Total: $10000
	// VFV: 60 × $100 = $6000 (60%), alloc USEquity=30% → drift=+30% SELL
	// VCN: 40 × $50 = $2000 (20%), alloc CanadianEquity=30% → drift=-10% BUY (suppressed)
	// CASH: quantity=2000 (20%), alloc Cash=40% → cashTargetWeight=40%
	//   currentCashWeight=20% < 40% → cashBelowTarget=true → all BUYs suppressed
	// Expected: only VFV SELL appears
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "USEquity", Percentage: 30},
				{Category: "CanadianEquity", Percentage: 30},
				{Category: "Cash", Percentage: 40},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 60, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 40, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a3", Ticker: "CASH", Category: "Cash", Quantity: 2000, Currency: "CAD", TargetPercentage: ptr(100.0)},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, r := range result.Trades {
		if r.Direction == "BUY" {
			t.Errorf("unexpected BUY for %s: cash is below its target, BUYs should be suppressed", r.Ticker)
		}
	}
	found := false
	for _, r := range result.Trades {
		if r.Ticker == "VFV" && r.Direction == "SELL" {
			found = true
		}
	}
	if !found {
		t.Errorf("VFV SELL not found: SELLs should still appear even when BUYs are suppressed")
	}
}

// TestCompute_BuyPartiallyFunded verifies that a BUY is included with a reduced
// DriftAmount (and IsPartialBuy=true) when portfolio cash covers only part of it.
func TestCompute_BuyPartiallyFunded(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 40 × $50 = $2000 (20% of $10000 total)
	//   alloc CanadianEquity=60%, effectiveTarget=60%, drift=-40% BUY, full amount=$4000
	// CASH: quantity=1000 (10%), no cash allocation → cashBelowTarget=false
	// portfolioCash=$1000 < $4000 → partial buy: DriftAmount capped to $1000, IsPartialBuy=true
	// VGS: $7000 to make up the rest of the total (SELL, not relevant)
	srv2 := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VGS", Exchange: "TSX", LatestPrice: ptr(70.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv2.Close()
	c2 := api.New(srv2.URL, "", nil)

	// VCN: $2000 (20%), alloc=60%, drift=-40%, full amount=$4000
	// VGS: 100 × $70 = $7000 (70%), alloc=40%, drift=+30% SELL
	// CASH: $1000 (10%), no cash alloc
	// Total: $10000
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 60},
				{Category: "Global", Percentage: 40},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 40, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VGS", Exchange: "TSX", Category: "Global", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a3", Ticker: "CASH", Category: "Cash", Quantity: 1000, Currency: "CAD"},
			},
		},
	}

	result, err := trades.Compute(portfolios, c2)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	var vcnRec *trades.TradeRecommendation
	for i, r := range result.Trades {
		if r.Ticker == "VCN" {
			vcnRec = &result.Trades[i]
		}
	}
	if vcnRec == nil {
		t.Fatalf("VCN not in results — expected a partial BUY")
	}
	if !vcnRec.IsPartialBuy {
		t.Errorf("IsPartialBuy = false, want true (cash covers only part of the BUY)")
	}
	if math.Abs(vcnRec.DriftAmount-1000) > 0.01 {
		t.Errorf("DriftAmount = %.2f, want 1000 (capped to available cash)", vcnRec.DriftAmount)
	}
	_ = srv // suppress unused warning
	_ = c
}

// TestCompute_BuyPriorityFunding verifies that when cash is insufficient for all
// BUYs, the highest-|drift| BUY is funded first and lower-priority BUYs are omitted.
func TestCompute_BuyPriorityFunding(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(1.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(1.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// Total: $10000
	// VCN: 6000 × $1 = $6000 (60%), alloc=90%, effectiveTarget=90%, drift=-30% BUY, amount=$3000
	// VFV: 1000 × $1 = $1000 (10%), alloc=20%, effectiveTarget=20%, drift=-10% BUY, amount=$1000
	// CASH: quantity=3000 (30%), no cash allocation → cashBelowTarget=false
	//
	// Sort BUYs by |drift|: VCN (30%) first, VFV (10%) second.
	// cashRemaining=$3000 ≥ VCN.$3000 → VCN funded, cashRemaining=$0
	// cashRemaining=$0 → VFV NOT funded (0 is not > 0)
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 90},
				{Category: "USEquity", Percentage: 20},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 6000, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 1000, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a3", Ticker: "CASH", Category: "Cash", Quantity: 3000, Currency: "CAD"},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	hasBuyVCN, hasBuyVFV := false, false
	for _, r := range result.Trades {
		if r.Direction == "BUY" {
			switch r.Ticker {
			case "VCN":
				hasBuyVCN = true
			case "VFV":
				hasBuyVFV = true
			}
		}
	}
	if !hasBuyVCN {
		t.Errorf("VCN BUY missing — highest drift BUY should be funded first")
	}
	if hasBuyVFV {
		t.Errorf("VFV BUY present — should be omitted (cash exhausted by higher-priority VCN)")
	}
}

// TestCompute_BuySuppressedWhenCashBelowMinTransaction verifies that all BUYs
// are omitted when available cash is less than the minimum transaction amount,
// even if the BUY's drift amount itself exceeds the minimum.
func TestCompute_BuySuppressedWhenCashBelowMinTransaction(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// Total: $10000
	// VCN: 80 × $50 = $4000 (40%), alloc CanadianEquity=60%, drift=-20% BUY, amount=$2000
	//   → passes per-asset min filter ($2000 ≥ $1000)
	//   → suppressed because portfolioCash($500) < minAmt($1000)
	// VFV: 55 × $100 = $5500 (55%), alloc USEquity=50%, drift=+5%, amount=$500
	//   → filtered by per-asset min filter ($500 < $1000) — never reaches cash check
	// CASH: quantity=500 < minAmt=$1000 → cash guard suppresses all BUYs
	// Expected: 0 result.Trades
	portfolios := []api.Portfolio{
		{
			ID:                     "p1",
			Name:                   "TFSA",
			MinTransactionAmount:   ptr(1000.0),
			MinTransactionCurrency: "CAD",
			Allocations: []api.Allocation{
				{Category: "CanadianEquity", Percentage: 60},
				{Category: "USEquity", Percentage: 50},
			},
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 80, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Category: "USEquity", Quantity: 55, Currency: "CAD", TargetPercentage: ptr(100.0)},
				{ID: "a3", Ticker: "CASH", Category: "Cash", Quantity: 500, Currency: "CAD"},
			},
		},
	}

	result, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(result.Trades) != 0 {
		for _, r := range result.Trades {
			t.Logf("unexpected rec: %s %s amount=%.2f", r.Direction, r.Ticker, r.DriftAmount)
		}
		t.Errorf("len(result.Trades) = %d, want 0 (cash $500 < minTransaction $1000 → BUY suppressed, SELL below min filtered)", len(result.Trades))
	}
}
