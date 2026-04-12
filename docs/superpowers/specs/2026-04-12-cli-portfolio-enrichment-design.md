# CLI Portfolio Enrichment Design

**Date:** 2026-04-12  
**Status:** Approved  
**Scope:** kashmere-cli only — no server or shared-package changes required

## Background

The web UI computes `totalValue` for each portfolio at read time by combining asset quantities with live prices. It always stores `totalValue: 0` in the encrypted portfolio blob (`PortfolioForm.tsx:144`). The CLI/MCP did not mirror this behaviour — it read and returned the stored `0`, causing the dashboard and goal summaries to silently show $0 for all portfolios.

This was identified during a manual portfolio audit on 2026-04-12 where ~$1.6M in portfolio value was missing from the goal summary.

## Root Cause

The CLI's `MergeAndUpdate` / `Get` calls decrypt the portfolio blob and return `totalValue` exactly as stored. The UI's `enrichPortfoliosWithPrices` (`usePortfolios.ts:24–58`) overrides this on every read. The CLI had no equivalent enrichment step.

## Design

### Core principle

The CLI becomes a first-class client that mirrors the UI's runtime behaviour. `totalValue` on a `Portfolio` struct has two distinct roles:

- **Stored value (in encrypted blob):** Always `0` when written by the UI. Used as a fallback only for portfolios with no assets.
- **Runtime value:** Computed from assets + live prices at read time. This is what all display, dashboard, and snapshot logic consumes.

The CLI must replicate the runtime computation.

### New package: `internal/portfolio/enrich.go`

Mirrors `enrichPortfoliosWithPrices` from `apps/client/src/hooks/usePortfolios.ts`.

```
func Enrich(portfolios []api.Portfolio, c *api.Client) ([]api.Portfolio, error)
```

Algorithm:
1. Collect unique non-static tickers across all portfolio assets (skip CASH, GIC, PROPERTY, MUTUALFUND).
2. Always include `USDCAD=X` in the price fetch for FX conversion.
3. If any non-static tickers exist, fetch all in a single bulk call: `GET /prices?tickers=VCN,VFV,USDCAD=X,...`
4. Fetch settings (`GET /settings`) to get `displayCurrency` (fallback: CAD).
5. For each portfolio:
   - If `assets` is empty → return as-is (stored `totalValue` is the fallback).
   - Otherwise, iterate assets:
     - **Static asset** (CASH, GIC, PROPERTY, MUTUALFUND): `assetValue = quantity`, `assetCurrency = asset.Currency ?? displayCurrency`
     - **Traded asset**: look up `latestPrice` from price map. If nil → skip (contributes 0).
     - Apply FX conversion: `convertedValue = assetValue × fxRate(assetCurrency → displayCurrency)`
   - If at least one asset contributed a value (`hasAnyPrice = true`) → set `portfolio.TotalValue = sum`.
   - If no asset contributed → return as-is (stored value, typically 0).

FX conversion: USD→CAD uses `USDCAD=X` latestPrice. If unavailable, rate = 1 (no conversion). Matches `currency.ts:getExchangeRate`.

Static asset detection: use `isStaticAsset()` equivalent — check ticker against `["CASH", "GIC", "PROPERTY", "MUTUALFUND"]`.

### Call sites — where enrichment is applied

All five portfolio read paths enrich before returning:

| Location | Change |
|---|---|
| `internal/mcp/portfolios.go` — `list_portfolios` | call `portfolio.Enrich()` after GET |
| `internal/mcp/portfolios.go` — `get_portfolio` | call `portfolio.Enrich()` after GET (wraps single portfolio in slice, unwraps after) |
| `internal/mcp/portfolios.go` — `create_portfolio` | call `portfolio.Enrich()` on returned portfolio before returning result |
| `internal/mcp/portfolios.go` — `update_portfolio` | call `portfolio.Enrich()` on returned portfolio before returning result |
| `internal/mcp/dashboard.go` — `get_dashboard` | enrich portfolios before passing to `dashboard.Compute()` |
| `cmd/portfolio.go` — `portfolio list` | call `portfolio.Enrich()` after GET |
| `cmd/portfolio.go` — `portfolio get` | call `portfolio.Enrich()` after GET |

Write paths (`create_portfolio`, `update_portfolio`) store whatever the caller provides (unchanged). The returned portfolio is enriched before being returned to the caller — each write handler calls `portfolio.Enrich()` on the single returned portfolio so the agent immediately sees the computed `totalValue`.

### `totalValue` input on create/update

`totalValue` becomes optional in `create_portfolio` MCP tool (default `0`). The CLI `portfolio create` flag `--total-value` is kept for manual overrides on asset-less portfolios but is no longer required when `--assets` is provided.

`update_portfolio` is unchanged — callers can still explicitly set `totalValue` for asset-less portfolios.

### What is NOT changing

- No changes to the server (`apps/server`)
- No changes to `@fp/shared`
- No changes to the price server
- `USDCAD=X` FX ticker is already bootstrapped at price-server startup — no new tracking needed
- The `recompute`, `audit`, and dashboard `warnings[]` proposals from the MCP agent's README are not implemented — they solve problems that no longer exist once enrichment is live

### `totalValue` field semantics (not deprecated)

`totalValue` on `Portfolio` is retained. Its semantics are:

- **Stored:** manual fallback for asset-less portfolios. The UI always writes `0` here (a latent UI bug for asset-less portfolios — out of scope).
- **Runtime:** computed by enrichment, overrides stored value whenever assets are present and resolvable.

Do not remove `totalValue` from the type — it is used by the snapshot scheduler (`SnapshotScheduler.tsx:75`: only snapshots portfolios where `totalValue > 0`), display components, dashboard aggregation, and goal delta calculations.

## File changes summary

| File | Change |
|---|---|
| `internal/portfolio/enrich.go` | New file — enrichment logic |
| `internal/mcp/portfolios.go` | Enrich after list/get/create/update |
| `internal/mcp/dashboard.go` | Enrich before `dashboard.Compute()` |
| `cmd/portfolio.go` | Enrich after list/get; `--total-value` optional when assets present |

## Out of scope

- UI bug: `PortfolioForm` always passes `totalValue: 0`, silently zeroing manually-set values for asset-less portfolios on edit.
- CLI `portfolio create` / `portfolio update` commands — these are less commonly used than MCP tools and can be enriched in a follow-up if needed.
- Performance: bulk price fetch means one HTTP call per enrich pass regardless of portfolio count. Acceptable for CLI/MCP usage patterns.
