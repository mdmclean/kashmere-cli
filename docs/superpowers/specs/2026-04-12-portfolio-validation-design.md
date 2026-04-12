# Portfolio Allocation Validation & Skill Doc Fixes

**Date:** 2026-04-12
**Status:** Approved

## Problem

The Kashmere API silently accepts invalid allocation data:

- `allocations[].percentage` values can sum to something other than 100% (e.g. 105%) with no complaint.
- `asset.targetPercentage` values can sum to more than 100% within a class.
- No cross-check between the two levels is performed.

Additionally, the `kashmere.md` skill doc does not explain the two-level allocation model, causing `targetPercentage` to be misunderstood as portfolio share rather than within-class share.

## Goals

1. Add client-side validation that errors before calling the API when allocations are invalid.
2. Fix the skill doc to clearly explain the two-level model with an example.

## Non-Goals

- Server-side validation (out of scope).
- Interactive/incremental allocation entry UX.

---

## Design

### 1. Validator — `internal/portfolio/validate.go`

A new exported function:

```go
func Validate(p api.Portfolio) error
```

Runs three checks in order. All use a ±0.01 float tolerance for rounding.

**Check 1 — Allocation sum**

Sum all `p.Allocations[i].Percentage`. If not 100:

```
allocation percentages sum to 94.00%, must equal 100%
```

Skip this check if `p.Allocations` is empty (portfolios with no allocations set are valid).

**Check 2 — Asset target sum per class**

Group assets by `Category`. For each group where **all** assets have `TargetPercentage != nil`, sum those values. If the sum is not 100:

```
asset target percentages for "US Equity" sum to 105.00%, must equal 100%
```

Groups where any asset has a nil `TargetPercentage` are skipped — partial targets mean the user hasn't finished setting them for that class.

**Call sites**

`Validate` is called in both layers immediately before the API write, on the portfolio body being sent:

- `cmd/portfolio.go`: `portfolioCreateCmd` and `portfolioUpdateCmd` (after parsing flags, before `client.Post`/`client.MergeAndUpdate`)
- `internal/mcp/portfolios.go`: `create_portfolio` and `update_portfolio` handlers (after building `body`, before `c.Post`/`c.MergeAndUpdate`)

For the update path, validate the **merged** portfolio (fetch current, apply updates, then validate) so the check reflects what will actually be saved.

### 2. Skill Doc — `skills/kashmere.md`

Add an **Asset targets** subsection under the portfolio section:

---

#### Asset targets (`targetPercentage`)

Assets have an optional `targetPercentage` field. This is the asset's **share within its asset class**, not its share of the total portfolio.

Example — a portfolio with two allocation classes:

```json
"allocations": [
  { "category": "US Equity",        "percentage": 50 },
  { "category": "Canadian Equity",  "percentage": 50 }
]
```

With assets:

```json
"assets": [
  { "ticker": "VTI", "category": "US Equity",       "targetPercentage": 60 },
  { "ticker": "VGK", "category": "US Equity",       "targetPercentage": 40 },
  { "ticker": "VCN", "category": "Canadian Equity", "targetPercentage": 100 }
]
```

VTI is 60% of the US Equity class, which is 50% of the portfolio → VTI = **30% of total portfolio**.

**Rules enforced by the CLI/MCP (hard errors):**

- `allocations[].percentage` must sum to 100%.
- `asset.targetPercentage` values must sum to 100% within each class (for classes where any asset has a target set).
- The rolled-up asset targets must be consistent with the portfolio-level allocations (within 1%).

---

## File Changes

| File | Change |
|------|--------|
| `internal/portfolio/validate.go` | New file — `Validate(p api.Portfolio) error` |
| `internal/portfolio/validate_test.go` | New file — unit tests for all checks |
| `cmd/portfolio.go` | Call `portfolio.Validate(...)` in create and update before API write |
| `internal/mcp/portfolios.go` | Call `portfolio.Validate(...)` in create and update before API write |
| `skills/kashmere.md` | Add Asset targets subsection with example and rules |

## Testing

Unit tests in `validate_test.go` cover:

- Valid portfolio (all checks pass)
- Allocations sum ≠ 100%
- Allocations empty (skipped — no error)
- Asset targets sum ≠ 100% within a class (all assets in class have targets)
- Asset targets partially set in a class (skipped — no error)
- Asset targets absent for a class (skipped — no error)
- Float rounding edge cases (e.g. 33.33 + 33.33 + 33.34)
