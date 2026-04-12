# Portfolio Enrichment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the CLI/MCP compute `totalValue` from assets + live prices at read time, mirroring the UI's `enrichPortfoliosWithPrices` logic so agents never see stale $0 values.

**Architecture:** A new `internal/portfolio` package exposes an `Enrich()` function that fetches prices in a single bulk call, then overwrites each portfolio's `TotalValue` using the same static/traded/FX logic as `usePortfolios.ts`. All five portfolio read paths and the two write-response paths call `Enrich()` before returning data.

**Tech Stack:** Go 1.26, `github.com/mdmclean/kashmere-cli/internal/api` (existing client + types), standard library only.

**Spec:** `docs/superpowers/specs/2026-04-12-cli-portfolio-enrichment-design.md`

---

## File Map

| File | Status | Responsibility |
|---|---|---|
| `internal/portfolio/enrich.go` | **Create** | `Enrich()` function — price fetching, static/traded logic, FX conversion |
| `internal/portfolio/enrich_test.go` | **Create** | Unit tests for all enrichment cases |
| `internal/mcp/portfolios.go` | **Modify** | Call `Enrich()` in list, get, create, update handlers; make `totalValue` optional in create |
| `internal/mcp/dashboard.go` | **Modify** | Call `Enrich()` on portfolios before `dashboard.Compute()` |
| `cmd/portfolio.go` | **Modify** | Call `Enrich()` in `portfolio list` and `portfolio get`; make `--total-value` optional |

---

## Task 1: Core enrichment package (TDD)

**Files:**
- Create: `internal/portfolio/enrich.go`
- Create: `internal/portfolio/enrich_test.go`

The enrichment function takes decrypted portfolios and an API client. It fetches prices once (bulk), then iterates portfolios to compute `TotalValue`. It does NOT modify the caller's slice — it returns a new slice.

Static tickers: `CASH`, `GIC`, `PROPERTY`, `MUTUALFUND` — value = quantity directly.  
Traded tickers: value = quantity × latestPrice (skip if price missing).  
FX: USD→CAD uses `USDCAD=X` latestPrice; missing rate → rate=1.  
`displayCurrency` comes from `GET /settings`; failure → default CAD.

- [ ] **Step 1: Write the failing tests**

Create `internal/portfolio/enrich_test.go`:

```go
package portfolio_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
)

func ptr[T any](v T) *T { return &v }

// newTestServer starts a fake API that handles /prices and /settings.
// pricesResp is the JSON array to return from GET /prices.
// displayCurrency is returned in GET /settings.
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
	// CASH with currency USD — should be converted to CAD using USDCAD rate
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
	// No price returned for VCN — portfolio should stay at 0
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
	// No asset contributed a price — stored value preserved
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
	// USDCAD=X has nil price — FX rate falls back to 1
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
	// Rate falls back to 1 — no conversion
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go test ./internal/portfolio/... 2>&1
```

Expected: `cannot find package "github.com/mdmclean/kashmere-cli/internal/portfolio"` (package doesn't exist yet).

- [ ] **Step 3: Implement `internal/portfolio/enrich.go`**

Create `internal/portfolio/enrich.go`:

```go
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
```

- [ ] **Step 4: Run tests and verify they pass**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go test ./internal/portfolio/... -v 2>&1
```

Expected: all 8 tests PASS.

- [ ] **Step 5: Verify all existing tests still pass**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go test ./... 2>&1
```

Expected: no failures.

- [ ] **Step 6: Commit**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
git add internal/portfolio/enrich.go internal/portfolio/enrich_test.go
git commit -m "feat: add portfolio enrichment package

Mirrors enrichPortfoliosWithPrices from usePortfolios.ts.
Static assets (CASH/GIC/PROPERTY/MUTUALFUND) use quantity directly.
Traded assets use quantity × latestPrice. FX via USDCAD=X.
Falls back to stored totalValue when no asset has a resolvable price."
```

---

## Task 2: Wire enrichment into MCP portfolio tools

**Files:**
- Modify: `internal/mcp/portfolios.go`

Add `portfolio.Enrich()` calls to all four portfolio MCP handlers. The `totalValue` field in `createPortfolioInput` becomes optional (pointer, defaults to 0).

- [ ] **Step 1: Update `list_portfolios` handler**

In `internal/mcp/portfolios.go`, change the `list_portfolios` handler from:

```go
var portfolios []api.Portfolio
if err := c.Get("/portfolios", &portfolios); err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(portfolios), nil, nil
```

To:

```go
var portfolios []api.Portfolio
if err := c.Get("/portfolios", &portfolios); err != nil {
    return ErrResult(err), nil, nil
}
enriched, err := portfolio.Enrich(portfolios, c)
if err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(enriched), nil, nil
```

- [ ] **Step 2: Update `get_portfolio` handler**

Change the `get_portfolio` handler from:

```go
var portfolio api.Portfolio
if err := c.Get("/portfolios/"+in.ID, &portfolio); err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(portfolio), nil, nil
```

To:

```go
var p api.Portfolio
if err := c.Get("/portfolios/"+in.ID, &p); err != nil {
    return ErrResult(err), nil, nil
}
enriched, err := portfolio.Enrich([]api.Portfolio{p}, c)
if err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(enriched[0]), nil, nil
```

- [ ] **Step 3: Update `create_portfolio` — make `totalValue` optional and enrich response**

Change the `createPortfolioInput` struct — make `TotalValue` a pointer:

```go
type createPortfolioInput struct {
    Name                   string           `json:"name" jsonschema:"Portfolio name"`
    Institution            string           `json:"institution" jsonschema:"Financial institution name (e.g. Wealthsimple, Questrade)"`
    Owner                  string           `json:"owner" jsonschema:"Account owner: person1, person2, or joint"`
    GoalID                 string           `json:"goalId" jsonschema:"ID of the goal this portfolio is assigned to"`
    TotalValue             *float64         `json:"totalValue,omitempty" jsonschema:"Total portfolio value in dollars (optional — computed from assets when present)"`
    Allocations            []api.Allocation `json:"allocations" jsonschema:"Target asset allocations, must sum to 100"`
    Description            *string          `json:"description,omitempty" jsonschema:"Optional portfolio description"`
    ManagementType         *string          `json:"managementType,omitempty" jsonschema:"Portfolio management type: self (default) or auto"`
    Assets                 []api.Asset      `json:"assets,omitempty" jsonschema:"Optional individual asset holdings"`
    MinTransactionAmount   *float64         `json:"minTransactionAmount,omitempty" jsonschema:"Optional minimum transaction amount"`
    MinTransactionCurrency *string          `json:"minTransactionCurrency,omitempty" jsonschema:"Optional min transaction currency: CAD or USD"`
}
```

Change the body construction in the create handler:

```go
totalValue := 0.0
if in.TotalValue != nil {
    totalValue = *in.TotalValue
}
body := map[string]any{
    "name":        in.Name,
    "institution": in.Institution,
    "owner":       in.Owner,
    "goalId":      in.GoalID,
    "totalValue":  totalValue,
    "allocations": in.Allocations,
}
```

Enrich the response before returning. Change the final section from:

```go
var portfolio api.Portfolio
if err := c.Post("/portfolios", body, &portfolio); err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(portfolio), nil, nil
```

To:

```go
var created api.Portfolio
if err := c.Post("/portfolios", body, &created); err != nil {
    return ErrResult(err), nil, nil
}
enriched, err := portfolio.Enrich([]api.Portfolio{created}, c)
if err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(enriched[0]), nil, nil
```

- [ ] **Step 4: Update `update_portfolio` — enrich response**

Change the final section of the update handler from:

```go
var portfolio api.Portfolio
if err := c.MergeAndUpdate("/portfolios/"+in.ID, updates, &portfolio); err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(portfolio), nil, nil
```

To:

```go
var updated api.Portfolio
if err := c.MergeAndUpdate("/portfolios/"+in.ID, updates, &updated); err != nil {
    return ErrResult(err), nil, nil
}
enriched, err := portfolio.Enrich([]api.Portfolio{updated}, c)
if err != nil {
    return ErrResult(err), nil, nil
}
return JSONResult(enriched[0]), nil, nil
```

- [ ] **Step 5: Add import for portfolio package**

At the top of `internal/mcp/portfolios.go`, add to the imports:

```go
import (
    "context"
    "fmt"

    "github.com/mdmclean/kashmere-cli/internal/api"
    "github.com/mdmclean/kashmere-cli/internal/portfolio"
    sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)
```

- [ ] **Step 6: Build to verify no compile errors**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go build ./... 2>&1
```

Expected: no output (clean build).

- [ ] **Step 7: Run all tests**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go test ./... 2>&1
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
git add internal/mcp/portfolios.go
git commit -m "feat: enrich portfolio values in MCP handlers

list_portfolios, get_portfolio, create_portfolio, and update_portfolio
all compute totalValue from assets + live prices before returning.
totalValue is now optional in create_portfolio (defaults to 0)."
```

---

## Task 3: Wire enrichment into MCP dashboard tool

**Files:**
- Modify: `internal/mcp/dashboard.go`

The dashboard aggregates `p.TotalValue` across portfolios. Enriching before `dashboard.Compute()` automatically fixes goal summaries, total, and weighted allocations.

- [ ] **Step 1: Update `get_dashboard` handler**

In `internal/mcp/dashboard.go`, change the handler from:

```go
var portfolios []api.Portfolio
if err := c.Get("/portfolios", &portfolios); err != nil {
    return ErrResult(err), nil, nil
}
var goals []api.Goal
if err := c.Get("/goals", &goals); err != nil {
    return ErrResult(err), nil, nil
}
var mortgages []api.Mortgage
c.Get("/mortgages", &mortgages) // optional — ignore error

return JSONResult(dashboard.Compute(portfolios, goals, mortgages)), nil, nil
```

To:

```go
var portfolios []api.Portfolio
if err := c.Get("/portfolios", &portfolios); err != nil {
    return ErrResult(err), nil, nil
}
enriched, err := portfolio.Enrich(portfolios, c)
if err != nil {
    return ErrResult(err), nil, nil
}
var goals []api.Goal
if err := c.Get("/goals", &goals); err != nil {
    return ErrResult(err), nil, nil
}
var mortgages []api.Mortgage
c.Get("/mortgages", &mortgages) // optional — ignore error

return JSONResult(dashboard.Compute(enriched, goals, mortgages)), nil, nil
```

- [ ] **Step 2: Add import for portfolio package**

In `internal/mcp/dashboard.go`, update imports to:

```go
import (
    "context"

    "github.com/mdmclean/kashmere-cli/internal/api"
    "github.com/mdmclean/kashmere-cli/internal/dashboard"
    "github.com/mdmclean/kashmere-cli/internal/portfolio"
    sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)
```

- [ ] **Step 3: Build and test**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go build ./... && go test ./... 2>&1
```

Expected: clean build, all tests pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
git add internal/mcp/dashboard.go
git commit -m "feat: enrich portfolios before dashboard compute in MCP

Goal summaries and total value now reflect live asset prices."
```

---

## Task 4: Wire enrichment into CLI portfolio commands

**Files:**
- Modify: `cmd/portfolio.go`

The `portfolio list` and `portfolio get` CLI commands pass results through enrichment. The `--total-value` flag on `portfolio create` becomes optional.

- [ ] **Step 1: Update `portfolio list` command**

In `cmd/portfolio.go`, change `portfolioListCmd.RunE` from:

```go
client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
var portfolios []api.Portfolio
if err := client.Get("/portfolios", &portfolios); err != nil {
    outputError(err, 0)
}
outputJSON(portfolios)
return nil
```

To:

```go
client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
var portfolios []api.Portfolio
if err := client.Get("/portfolios", &portfolios); err != nil {
    outputError(err, 0)
}
enriched, err := portfolio.Enrich(portfolios, client)
if err != nil {
    outputError(err, 0)
}
outputJSON(enriched)
return nil
```

- [ ] **Step 2: Update `portfolio get` command**

Change `portfolioGetCmd.RunE` from:

```go
client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
var portfolio api.Portfolio
if err := client.Get("/portfolios/"+args[0], &portfolio); err != nil {
    outputError(err, 0)
}
outputJSON(portfolio)
return nil
```

To:

```go
client := api.New(cfg.APIBaseURL, cfg.APIKey, encKey)
var p api.Portfolio
if err := client.Get("/portfolios/"+args[0], &p); err != nil {
    outputError(err, 0)
}
enriched, err := portfolio.Enrich([]api.Portfolio{p}, client)
if err != nil {
    outputError(err, 0)
}
outputJSON(enriched[0])
return nil
```

- [ ] **Step 3: Make `--total-value` optional on `portfolio create`**

In `portfolioCreateCmd.RunE`, change the totalValue parsing from a hard requirement:

```go
// Replace this block:
totalValue, err := strconv.ParseFloat(pfTotalValue, 64)
if err != nil {
    return fmt.Errorf("--total-value must be a number: %w", err)
}
```

To:

```go
totalValue := 0.0
if pfTotalValue != "" {
    v, err := strconv.ParseFloat(pfTotalValue, 64)
    if err != nil {
        return fmt.Errorf("--total-value must be a number: %w", err)
    }
    totalValue = v
}
```

In the `init()` function, remove the `MarkFlagRequired` call for `--total-value`:

```go
// Remove this line:
portfolioCreateCmd.MarkFlagRequired("total-value")
```

Update the flag description in `init()` to reflect it is now optional:

```go
portfolioCreateCmd.Flags().StringVar(&pfTotalValue, "total-value", "", "Total portfolio value (optional — computed from assets when present)")
```

- [ ] **Step 4: Add import for portfolio package**

In `cmd/portfolio.go`, add the portfolio package import. The import block should include:

```go
import (
    "encoding/json"
    "fmt"
    "strconv"

    "github.com/mdmclean/kashmere-cli/internal/api"
    "github.com/mdmclean/kashmere-cli/internal/portfolio"
    "github.com/spf13/cobra"
)
```

- [ ] **Step 5: Build and test**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go build ./... && go test ./... 2>&1
```

Expected: clean build, all tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
git add cmd/portfolio.go
git commit -m "feat: enrich portfolio values in CLI commands

portfolio list and portfolio get now compute totalValue from assets.
--total-value is now optional on portfolio create."
```

---

## Self-Review Notes

**Spec coverage check:**
- ✅ `internal/portfolio/enrich.go` — static/traded/FX logic
- ✅ `list_portfolios`, `get_portfolio` — enrich after GET
- ✅ `create_portfolio`, `update_portfolio` — enrich response
- ✅ `get_dashboard` — enrich before Compute
- ✅ `portfolio list`, `portfolio get` CLI commands
- ✅ `totalValue` optional in create_portfolio MCP tool
- ✅ `--total-value` optional in CLI create command
- ✅ Fallback to stored value when no assets / no prices

**Note on Settings fetch:** The `/settings` endpoint returns an encrypted blob on E2EE accounts. The `api.Client.Get` call in `Enrich` will decrypt it automatically (settings is in `encryptedPaths`). The `api.Settings` struct has `DisplayCurrency string` — this will be populated correctly after decryption.

**Note on price map key matching:** The price server returns `ticker` + `exchange` as separate fields. The plan builds two map entries for each price with an exchange: `"VCN:TSX"` and `"VCN"` (bare fallback). This mirrors the multi-step fallback in `findPriceForTicker` on the price server.
