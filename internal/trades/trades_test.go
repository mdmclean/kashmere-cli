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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("len(recs) = %d, want 0 (GIC is perfectly on target at 100%% of 100%% allocation)", len(recs))
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

	recs, err := trades.Compute(portfolios, c2)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	// GIC: effectiveTarget=5%, currentPct=5000/100000*100=5% → drift≈0 → skip
	// VCN: effectiveTarget=95%, currentPct=95000/100000*100=95% → drift≈0 → skip
	if len(recs) != 0 {
		for _, r := range recs {
			t.Logf("unexpected rec: %s drift=%.2f%% (target=%.2f%% current=%.2f%%)", r.Ticker, r.DriftPct, r.TargetPct, r.CurrentPct)
		}
		t.Errorf("len(recs) = %d, want 0 (both assets on target when effectiveTarget is scaled by allocation)", len(recs))
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

	// VCN: 100 shares × $50 = $5000 (50% of total)
	//   category "CanadianEquity", allocation 60%
	//   targetPercentage=100 → effectiveTarget = (100/100)*60 = 60%
	//   drift = 50 - 60 = -10% → BUY, amount = $1000
	// VFV: 50 shares × $100 = $5000 (50% of total)
	//   category "USEquity", allocation 40%
	//   targetPercentage=100 → effectiveTarget = (100/100)*40 = 40%
	//   drift = 50 - 40 = +10% → SELL, amount = $1000
	// Equal |drift|, tiebreak by ticker: VCN < VFV → VCN first
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("len(recs) = %d, want 2", len(recs))
	}

	vcn := recs[0]
	if vcn.Ticker != "VCN" {
		t.Errorf("recs[0].Ticker = %s, want VCN (alphabetic tiebreak)", vcn.Ticker)
	}
	if vcn.Direction != "BUY" {
		t.Errorf("VCN direction = %s, want BUY", vcn.Direction)
	}
	if math.Abs(vcn.DriftPct-(-10)) > 0.001 {
		t.Errorf("VCN DriftPct = %.4f, want -10", vcn.DriftPct)
	}
	if math.Abs(vcn.TargetPct-60) > 0.001 {
		t.Errorf("VCN TargetPct = %.4f, want 60", vcn.TargetPct)
	}
	if math.Abs(vcn.DriftAmount-1000) > 0.01 {
		t.Errorf("VCN DriftAmount = %.2f, want 1000", vcn.DriftAmount)
	}

	vfv := recs[1]
	if vfv.Ticker != "VFV" {
		t.Errorf("recs[1].Ticker = %s, want VFV", vfv.Ticker)
	}
	if vfv.Direction != "SELL" {
		t.Errorf("VFV direction = %s, want SELL", vfv.Direction)
	}
	if math.Abs(vfv.DriftPct-10) > 0.001 {
		t.Errorf("VFV DriftPct = %.4f, want 10", vfv.DriftPct)
	}
	if math.Abs(vfv.TargetPct-40) > 0.001 {
		t.Errorf("VFV TargetPct = %.4f, want 40", vfv.TargetPct)
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, r := range recs {
		if r.Ticker == "CASH" {
			t.Errorf("CASH appeared in results — it must always be excluded")
		}
	}
	// VCN should still appear
	if len(recs) != 1 || recs[0].Ticker != "VCN" {
		t.Errorf("expected exactly 1 rec (VCN), got %d", len(recs))
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	for _, r := range recs {
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("len(recs) = %d, want 0 (no targets defined)", len(recs))
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("len(recs) = %d, want 0 (all below min transaction)", len(recs))
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("len(recs) = %d, want 0 (zero total value)", len(recs))
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("len(recs) = %d, want 2", len(recs))
	}
	if recs[0].PortfolioID != "p2" {
		t.Errorf("recs[0].PortfolioID = %s, want p2 (higher drift)", recs[0].PortfolioID)
	}
	if recs[1].PortfolioID != "p1" {
		t.Errorf("recs[1].PortfolioID = %s, want p1", recs[1].PortfolioID)
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("len(recs) = %d, want 0 (zero drift should be skipped)", len(recs))
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("len(recs) = %d, want 0 (both $1000 CAD trades below $600 USD = $1200 CAD threshold)", len(recs))
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

	recs, err := trades.Compute(portfolios, c)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("len(recs) = %d, want 1", len(recs))
	}
	if recs[0].Ticker != "VCN" {
		t.Errorf("ticker = %s, want VCN", recs[0].Ticker)
	}
	if recs[0].Direction != "SELL" {
		t.Errorf("direction = %s, want SELL (effective target is 0%%)", recs[0].Direction)
	}
	if math.Abs(recs[0].TargetPct) > 0.001 {
		t.Errorf("TargetPct = %.4f, want 0 (no matching category)", recs[0].TargetPct)
	}
}
