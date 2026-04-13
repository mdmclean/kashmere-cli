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

func TestCompute_NormalRanking(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 100 shares × $50 = $5000 (50% of total)
	// VFV: 50 shares × $100 = $5000 (50% of total)
	// Total: $10000
	// VCN target: 60% → drift = 50 - 60 = -10% → BUY, amount = 1000
	// VFV target: 40% → drift = 50 - 40 = +10% → SELL, amount = 1000
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(60.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Quantity: 50, Currency: "CAD", TargetPercentage: ptr(40.0)},
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

	// Both have equal |drift| so order between them is stable but either can be first.
	// Verify both are present with correct values.
	byTicker := map[string]trades.TradeRecommendation{}
	for _, r := range recs {
		byTicker[r.Ticker] = r
	}

	vcn := byTicker["VCN"]
	if vcn.Direction != "BUY" {
		t.Errorf("VCN direction = %s, want BUY", vcn.Direction)
	}
	if math.Abs(vcn.DriftPct-(-10)) > 0.001 {
		t.Errorf("VCN DriftPct = %.4f, want -10", vcn.DriftPct)
	}
	if math.Abs(vcn.DriftAmount-1000) > 0.01 {
		t.Errorf("VCN DriftAmount = %.2f, want 1000", vcn.DriftAmount)
	}

	vfv := byTicker["VFV"]
	if vfv.Direction != "SELL" {
		t.Errorf("VFV direction = %s, want SELL", vfv.Direction)
	}
	if math.Abs(vfv.DriftPct-10) > 0.001 {
		t.Errorf("VFV DriftPct = %.4f, want 10", vfv.DriftPct)
	}
}

func TestCompute_NullTargetTreatedAsZero(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "CASH", LatestPrice: nil},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 100 × $50 = $5000 (50%)  target=60%  drift=-10%
	// CASH: $5000 (50%)  target=nil → treated as 0%  drift=+50%
	// CASH has bigger |drift| so it should rank first
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(60.0)},
				{ID: "a2", Ticker: "CASH", Quantity: 5000, Currency: "CAD", TargetPercentage: nil},
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
	if recs[0].Ticker != "CASH" {
		t.Errorf("first rec ticker = %s, want CASH (highest |drift|)", recs[0].Ticker)
	}
	if recs[0].Direction != "SELL" {
		t.Errorf("CASH direction = %s, want SELL", recs[0].Direction)
	}
	if math.Abs(recs[0].TargetPct) > 0.001 {
		t.Errorf("CASH TargetPct = %.4f, want 0", recs[0].TargetPct)
	}
}

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
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100, Currency: "CAD", TargetPercentage: nil},
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

func TestCompute_MinTransactionFilter(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: $5000 (50%), target=60% → drift=-10%, amount=$1000
	// VFV: $5000 (50%), target=40% → drift=+10%, amount=$1000
	// minTransactionAmount=1500 CAD → both $1000 trades filtered out
	portfolios := []api.Portfolio{
		{
			ID:                     "p1",
			Name:                   "TFSA",
			MinTransactionAmount:   ptr(1500.0),
			MinTransactionCurrency: "CAD",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(60.0)},
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Quantity: 50, Currency: "CAD", TargetPercentage: ptr(40.0)},
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
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100, TargetPercentage: ptr(100.0)},
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

func TestCompute_MultiPortfolioFlat(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(100.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// p1: VCN 100% actual, 80% target → drift +20%
	// p2: VFV 100% actual, 70% target → drift +30% (bigger, should rank first)
	portfolios := []api.Portfolio{
		{
			ID:   "p1",
			Name: "TFSA",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(80.0)},
			},
		},
		{
			ID:   "p2",
			Name: "RRSP",
			Assets: []api.Asset{
				{ID: "a2", Ticker: "VFV", Exchange: "TSX", Quantity: 50, Currency: "CAD", TargetPercentage: ptr(70.0)},
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
