package portfolio_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
)

func ptr[T any](v T) *T { return &v }

// newTestServer starts a fake API that handles /prices and /settings.
func newTestServer(t *testing.T, pricesResp []api.TickerPrice, displayCurrency string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/prices":
			json.NewEncoder(w).Encode(pricesResp)
		case r.URL.Path == "/settings":
			json.NewEncoder(w).Encode(api.Settings{DisplayCurrency: displayCurrency})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestEnrich_StaticCashAsset(t *testing.T) {
	srv := newTestServer(t, nil, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID: "p1",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "CASH", Quantity: 50000},
			},
			TotalValue: 0,
		},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if got[0].TotalValue != 50000 {
		t.Errorf("TotalValue = %.2f, want 50000", got[0].TotalValue)
	}
}

func TestEnrich_StaticCashAssetUSD(t *testing.T) {
	usdcadRate := 1.38
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(usdcadRate)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID: "p1",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "CASH", Quantity: 10000, Currency: "USD"},
			},
			TotalValue: 0,
		},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	want := 10000 * usdcadRate
	if got[0].TotalValue != want {
		t.Errorf("TotalValue = %.2f, want %.2f", got[0].TotalValue, want)
	}
}

func TestEnrich_TradedAsset(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(45.50), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID: "p1",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100, Currency: "CAD"},
			},
			TotalValue: 0,
		},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	want := 100.0 * 45.50
	if got[0].TotalValue != want {
		t.Errorf("TotalValue = %.2f, want %.2f", got[0].TotalValue, want)
	}
}

func TestEnrich_TradedAssetUSDToCad(t *testing.T) {
	usdcadRate := 1.38
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(150.0), Currency: "USD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(usdcadRate)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID: "p1",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VFV", Exchange: "TSX", Quantity: 10, Currency: "USD"},
			},
			TotalValue: 0,
		},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	want := 10 * 150.0 * usdcadRate
	if got[0].TotalValue != want {
		t.Errorf("TotalValue = %.2f, want %.2f", got[0].TotalValue, want)
	}
}

func TestEnrich_MissingPrice_SkipsAsset(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID: "p1",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "VCN", Exchange: "TSX", Quantity: 100},
			},
			TotalValue: 999,
		},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if got[0].TotalValue != 999 {
		t.Errorf("TotalValue = %.2f, want 999 (stored fallback)", got[0].TotalValue)
	}
}

func TestEnrich_NoAssets_Unchanged(t *testing.T) {
	srv := newTestServer(t, nil, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{ID: "p1", Assets: []api.Asset{}, TotalValue: 75000},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if got[0].TotalValue != 75000 {
		t.Errorf("TotalValue = %.2f, want 75000", got[0].TotalValue)
	}
}

func TestEnrich_MixedStaticAndTraded(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(45.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID: "p1",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "CASH", Quantity: 10000},
				{ID: "a2", Ticker: "VCN", Exchange: "TSX", Quantity: 200, Currency: "CAD"},
			},
			TotalValue: 0,
		},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	want := 10000.0 + 200*45.0
	if got[0].TotalValue != want {
		t.Errorf("TotalValue = %.2f, want %.2f", got[0].TotalValue, want)
	}
}

func TestEnrich_MissingFXRate_NoConversion(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: nil},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{
			ID: "p1",
			Assets: []api.Asset{
				{ID: "a1", Ticker: "CASH", Quantity: 5000, Currency: "USD"},
			},
			TotalValue: 0,
		},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if got[0].TotalValue != 5000 {
		t.Errorf("TotalValue = %.2f, want 5000", got[0].TotalValue)
	}
}

func TestEnrich_MultiplePortfolios(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{
		{ID: "p1", Assets: []api.Asset{{ID: "a1", Ticker: "CASH", Quantity: 1000}}, TotalValue: 0},
		{ID: "p2", Assets: []api.Asset{{ID: "a2", Ticker: "GIC", Quantity: 25000}}, TotalValue: 0},
	}

	got, err := portfolio.Enrich(portfolios, c)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if got[0].TotalValue != 1000 {
		t.Errorf("p1 TotalValue = %.2f, want 1000", got[0].TotalValue)
	}
	if got[1].TotalValue != 25000 {
		t.Errorf("p2 TotalValue = %.2f, want 25000", got[1].TotalValue)
	}
}

// ---------------------------------------------------------------------------
// EnrichFull tests
// ---------------------------------------------------------------------------

func TestEnrichFull_ComputesCurrentValueAndPct(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: 100 × $50 = $5000, CASH: $5000 → total $10000
	// VCN currentPct = 50%, CASH currentPct = 50%
	portfolios := []api.Portfolio{{
		ID: "p1",
		Assets: []api.Asset{
			{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD"},
			{ID: "a2", Ticker: "CASH", Category: "Cash", Quantity: 5000, Currency: "CAD"},
		},
	}}

	got, err := portfolio.EnrichFull(portfolios, c)
	if err != nil {
		t.Fatalf("EnrichFull: %v", err)
	}
	if got[0].TotalValue != 10000 {
		t.Errorf("TotalValue = %.2f, want 10000", got[0].TotalValue)
	}

	vcn := got[0].Assets[0]
	if vcn.CurrentValue == nil || math.Abs(*vcn.CurrentValue-5000) > 0.01 {
		t.Errorf("VCN CurrentValue = %v, want 5000", vcn.CurrentValue)
	}
	if vcn.CurrentPct == nil || math.Abs(*vcn.CurrentPct-50) > 0.001 {
		t.Errorf("VCN CurrentPct = %v, want 50", vcn.CurrentPct)
	}

	cash := got[0].Assets[1]
	if cash.CurrentValue == nil || math.Abs(*cash.CurrentValue-5000) > 0.01 {
		t.Errorf("CASH CurrentValue = %v, want 5000", cash.CurrentValue)
	}
	if cash.CurrentPct == nil || math.Abs(*cash.CurrentPct-50) > 0.001 {
		t.Errorf("CASH CurrentPct = %v, want 50", cash.CurrentPct)
	}
}

func TestEnrichFull_DriftPct(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "VCN", Exchange: "TSX", LatestPrice: ptr(50.0), Currency: "CAD"},
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	// VCN: $6000 (60%), effectiveTarget = (100/100)*50 = 50% → driftPct = +10%
	// CASH: $4000 (40%), no targetPercentage → driftPct nil
	portfolios := []api.Portfolio{{
		ID: "p1",
		Allocations: []api.Allocation{
			{Category: "CanadianEquity", Percentage: 50},
		},
		Assets: []api.Asset{
			{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 120, Currency: "CAD", TargetPercentage: ptr(100.0)},
			{ID: "a2", Ticker: "CASH", Category: "Cash", Quantity: 4000, Currency: "CAD"},
		},
	}}

	got, err := portfolio.EnrichFull(portfolios, c)
	if err != nil {
		t.Fatalf("EnrichFull: %v", err)
	}

	vcn := got[0].Assets[0]
	if vcn.DriftPct == nil {
		t.Fatal("VCN DriftPct = nil, want +10")
	}
	if math.Abs(*vcn.DriftPct-10) > 0.001 {
		t.Errorf("VCN DriftPct = %.4f, want 10", *vcn.DriftPct)
	}

	cash := got[0].Assets[1]
	if cash.DriftPct != nil {
		t.Errorf("CASH DriftPct = %v, want nil (no targetPercentage)", cash.DriftPct)
	}
}

func TestEnrichFull_MissingPrice_NilFields(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
		// no VCN price
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{{
		ID: "p1",
		Assets: []api.Asset{
			{ID: "a1", Ticker: "VCN", Exchange: "TSX", Category: "CanadianEquity", Quantity: 100, Currency: "CAD", TargetPercentage: ptr(100.0)},
		},
	}}

	got, err := portfolio.EnrichFull(portfolios, c)
	if err != nil {
		t.Fatalf("EnrichFull: %v", err)
	}
	a := got[0].Assets[0]
	if a.CurrentValue != nil {
		t.Errorf("CurrentValue = %v, want nil (no price)", a.CurrentValue)
	}
	if a.CurrentPct != nil {
		t.Errorf("CurrentPct = %v, want nil (no price)", a.CurrentPct)
	}
	if a.DriftPct != nil {
		t.Errorf("DriftPct = %v, want nil (no price)", a.DriftPct)
	}
}

func TestEnrichFull_PreservesBaseFields(t *testing.T) {
	srv := newTestServer(t, []api.TickerPrice{
		{Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}, "CAD")
	defer srv.Close()
	c := api.New(srv.URL, "", nil)

	portfolios := []api.Portfolio{{
		ID:   "p1",
		Name: "TFSA",
		Assets: []api.Asset{
			{ID: "a1", Ticker: "CASH", Name: "Cash", Category: "Cash", Quantity: 10000, Currency: "CAD"},
		},
	}}

	got, err := portfolio.EnrichFull(portfolios, c)
	if err != nil {
		t.Fatalf("EnrichFull: %v", err)
	}
	if got[0].ID != "p1" || got[0].Name != "TFSA" {
		t.Errorf("portfolio base fields not preserved: id=%s name=%s", got[0].ID, got[0].Name)
	}
	a := got[0].Assets[0]
	if a.ID != "a1" || a.Ticker != "CASH" || a.Quantity != 10000 {
		t.Errorf("asset base fields not preserved: id=%s ticker=%s qty=%.0f", a.ID, a.Ticker, a.Quantity)
	}
}

func TestFxRate_SameCurrency(t *testing.T) {
	priceMap := map[string]api.TickerPrice{
		"USDCAD=X": {Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}
	if got := portfolio.FxRate("CAD", "CAD", priceMap); got != 1.0 {
		t.Errorf("FxRate CAD→CAD = %.4f, want 1.0", got)
	}
}

func TestFxRate_USDToCAD(t *testing.T) {
	priceMap := map[string]api.TickerPrice{
		"USDCAD=X": {Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}
	if got := portfolio.FxRate("USD", "CAD", priceMap); got != 1.38 {
		t.Errorf("FxRate USD→CAD = %.4f, want 1.38", got)
	}
}

func TestFxRate_CADToUSD(t *testing.T) {
	priceMap := map[string]api.TickerPrice{
		"USDCAD=X": {Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}
	got := portfolio.FxRate("CAD", "USD", priceMap)
	want := 1.0 / 1.38
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("FxRate CAD→USD = %.6f, want %.6f", got, want)
	}
}

func TestFxRate_MissingRate_ReturnsOne(t *testing.T) {
	if got := portfolio.FxRate("USD", "CAD", map[string]api.TickerPrice{}); got != 1.0 {
		t.Errorf("FxRate missing rate = %.4f, want 1.0", got)
	}
}

func TestComputeAssetValue_StaticCAD(t *testing.T) {
	priceMap := map[string]api.TickerPrice{}
	a := api.Asset{Ticker: "CASH", Quantity: 10000, Currency: "CAD"}
	val, ok := portfolio.ComputeAssetValue(a, priceMap, "CAD")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if val != 10000 {
		t.Errorf("val = %.2f, want 10000", val)
	}
}

func TestComputeAssetValue_StaticUSDtoCAD(t *testing.T) {
	priceMap := map[string]api.TickerPrice{
		"USDCAD=X": {Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}
	a := api.Asset{Ticker: "CASH", Quantity: 1000, Currency: "USD"}
	val, ok := portfolio.ComputeAssetValue(a, priceMap, "CAD")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if val != 1380 {
		t.Errorf("val = %.2f, want 1380", val)
	}
}

func TestComputeAssetValue_TradedNoPriceReturnsFalse(t *testing.T) {
	priceMap := map[string]api.TickerPrice{}
	a := api.Asset{Ticker: "VCN", Exchange: "TSX", Quantity: 10}
	_, ok := portfolio.ComputeAssetValue(a, priceMap, "CAD")
	if ok {
		t.Error("ok = true, want false for missing price")
	}
}

func TestComputeAssetValue_TradedUSDtoCAD(t *testing.T) {
	priceMap := map[string]api.TickerPrice{
		"VFV:TSX":  {Ticker: "VFV", Exchange: "TSX", LatestPrice: ptr(150.0), Currency: "USD"},
		"USDCAD=X": {Ticker: "USDCAD=X", LatestPrice: ptr(1.38)},
	}
	a := api.Asset{Ticker: "VFV", Exchange: "TSX", Quantity: 10, Currency: "USD"}
	val, ok := portfolio.ComputeAssetValue(a, priceMap, "CAD")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	want := 10 * 150.0 * 1.38
	if math.Abs(val-want) > 0.001 {
		t.Errorf("val = %.4f, want %.4f", val, want)
	}
}
