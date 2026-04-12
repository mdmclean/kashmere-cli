# Portfolio Allocation Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add client-side validation that rejects invalid portfolio allocation percentages before hitting the API, and update the skill doc to clarify the two-level allocation model.

**Architecture:** A new `Validate(p api.Portfolio) error` function in `internal/portfolio/validate.go` is the single source of truth. Both `cmd/portfolio.go` and `internal/mcp/portfolios.go` call it before writing. For create, validate the constructed portfolio. For update, only validate if allocations or assets are being changed (skip an extra fetch otherwise).

**Tech Stack:** Go 1.26, `github.com/mdmclean/kashmere-cli/internal/api` types, standard `math` and `fmt` packages.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/portfolio/validate.go` | Create | `Validate(p api.Portfolio) error` — two allocation checks |
| `internal/portfolio/validate_test.go` | Create | Unit tests for all validation cases |
| `cmd/portfolio.go` | Modify | Call `portfolio.Validate` in create and update before API write |
| `internal/mcp/portfolios.go` | Modify | Call `portfolio.Validate` in create and update before API write |
| `skills/kashmere.md` | Modify | Add Asset targets subsection explaining the two-level model |

---

## Task 1: Write `Validate` with failing tests

**Files:**
- Create: `internal/portfolio/validate_test.go`
- Create: `internal/portfolio/validate.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/portfolio/validate_test.go`:

```go
package portfolio_test

import (
	"strings"
	"testing"

	"github.com/mdmclean/kashmere-cli/internal/api"
	"github.com/mdmclean/kashmere-cli/internal/portfolio"
)

func ptr[T any](v T) *T { return &v }

func TestValidate_valid(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 60},
			{Category: "Canadian Equity", Percentage: 40},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: ptr(60.0)},
			{Category: "US Equity", TargetPercentage: ptr(40.0)},
			{Category: "Canadian Equity", TargetPercentage: ptr(100.0)},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_allocationSumNot100(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 60},
			{Category: "Canadian Equity", Percentage: 45},
		},
	}
	err := portfolio.Validate(p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "105.00%") {
		t.Errorf("expected sum in error message, got: %v", err)
	}
}

func TestValidate_allocationEmpty_noError(t *testing.T) {
	p := api.Portfolio{Allocations: []api.Allocation{}}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error for empty allocations, got: %v", err)
	}
}

func TestValidate_assetTargetSumNot100(t *testing.T) {
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 100},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: ptr(60.0)},
			{Category: "US Equity", TargetPercentage: ptr(45.0)},
		},
	}
	err := portfolio.Validate(p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `"US Equity"`) || !strings.Contains(err.Error(), "105.00%") {
		t.Errorf("expected class name and sum in error, got: %v", err)
	}
}

func TestValidate_assetTargetsPartial_noError(t *testing.T) {
	// One asset has a target, one doesn't — partial targets are skipped.
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 100},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: ptr(60.0)},
			{Category: "US Equity", TargetPercentage: nil},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error for partial targets, got: %v", err)
	}
}

func TestValidate_assetTargetsAbsent_noError(t *testing.T) {
	// No assets have targets set at all — valid.
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "US Equity", Percentage: 100},
		},
		Assets: []api.Asset{
			{Category: "US Equity", TargetPercentage: nil},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error when no targets set, got: %v", err)
	}
}

func TestValidate_floatRounding(t *testing.T) {
	// 33.33 + 33.33 + 33.34 = 100.00 — should not error.
	p := api.Portfolio{
		Allocations: []api.Allocation{
			{Category: "A", Percentage: 33.33},
			{Category: "B", Percentage: 33.33},
			{Category: "C", Percentage: 33.34},
		},
		Assets: []api.Asset{
			{Category: "A", TargetPercentage: ptr(33.33)},
			{Category: "A", TargetPercentage: ptr(33.33)},
			{Category: "A", TargetPercentage: ptr(33.34)},
			{Category: "B", TargetPercentage: ptr(100.0)},
			{Category: "C", TargetPercentage: ptr(100.0)},
		},
	}
	if err := portfolio.Validate(p); err != nil {
		t.Errorf("expected no error for valid float rounding, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go test ./internal/portfolio/... -run TestValidate -v
```

Expected: compilation error — `portfolio.Validate` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/portfolio/validate.go`:

```go
// internal/portfolio/validate.go
package portfolio

import (
	"fmt"
	"math"

	"github.com/mdmclean/kashmere-cli/internal/api"
)

const percentageTolerance = 0.01

// Validate checks that a portfolio's allocations and asset targets are internally
// consistent before the portfolio is sent to the API.
//
// Checks performed:
//  1. portfolio-level allocation percentages sum to 100 (skipped if empty)
//  2. asset targetPercentage values sum to 100 within each category,
//     but only for categories where ALL assets have a target set
func Validate(p api.Portfolio) error {
	if err := validateAllocationSum(p.Allocations); err != nil {
		return err
	}
	return validateAssetTargetSums(p.Assets)
}

func validateAllocationSum(allocations []api.Allocation) error {
	if len(allocations) == 0 {
		return nil
	}
	sum := 0.0
	for _, a := range allocations {
		sum += a.Percentage
	}
	if math.Abs(sum-100) > percentageTolerance {
		return fmt.Errorf("allocation percentages sum to %.2f%%, must equal 100%%", sum)
	}
	return nil
}

func validateAssetTargetSums(assets []api.Asset) error {
	// Group assets by category.
	type classInfo struct {
		sum     float64
		count   int
		hasNil  bool
	}
	classes := map[string]*classInfo{}
	for _, a := range assets {
		if _, ok := classes[a.Category]; !ok {
			classes[a.Category] = &classInfo{}
		}
		info := classes[a.Category]
		info.count++
		if a.TargetPercentage == nil {
			info.hasNil = true
		} else {
			info.sum += *a.TargetPercentage
		}
	}

	for category, info := range classes {
		// Skip classes where any asset lacks a target (partial = incomplete, not invalid).
		if info.hasNil {
			continue
		}
		// Skip classes with no assets (shouldn't happen but defensive).
		if info.count == 0 {
			continue
		}
		if math.Abs(info.sum-100) > percentageTolerance {
			return fmt.Errorf("asset target percentages for %q sum to %.2f%%, must equal 100%%", category, info.sum)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go test ./internal/portfolio/... -run TestValidate -v
```

Expected: all `TestValidate_*` tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/portfolio/validate.go internal/portfolio/validate_test.go
git commit -m "feat: add portfolio allocation validator"
```

---

## Task 2: Wire validation into CLI create/update

**Files:**
- Modify: `cmd/portfolio.go`

- [ ] **Step 1: Add validation to `portfolioCreateCmd`**

In `cmd/portfolio.go`, find the `portfolioCreateCmd` `RunE` function. After parsing assets (currently ends around the `pfAssetsJSON` block), add a validation call before the API post.

The current code ends with:

```go
		var assets []api.Asset
		if pfAssetsJSON != "" {
			if err := json.Unmarshal([]byte(pfAssetsJSON), &assets); err != nil {
				return fmt.Errorf("--assets must be valid JSON: %w", err)
			}
		}

		body := map[string]any{
```

Replace with:

```go
		var assets []api.Asset
		if pfAssetsJSON != "" {
			if err := json.Unmarshal([]byte(pfAssetsJSON), &assets); err != nil {
				return fmt.Errorf("--assets must be valid JSON: %w", err)
			}
		}

		if err := portfolio.Validate(api.Portfolio{Allocations: allocations, Assets: assets}); err != nil {
			return err
		}

		body := map[string]any{
```

- [ ] **Step 2: Add validation to `portfolioUpdateCmd`**

In `portfolioUpdateCmd` `RunE`, after the `if len(updates) == 0` check and before the `client.MergeAndUpdate` call, add:

```go
		// Only validate if allocation-sensitive fields are changing.
		if _, changingAllocs := updates["allocations"]; changingAllocs {
			if _, changingAssets := updates["assets"]; true {
				_ = changingAssets
			}
		}
		if updates["allocations"] != nil || updates["assets"] != nil {
			var current api.Portfolio
			if err := client.Get("/portfolios/"+args[0], &current); err != nil {
				outputError(err, 0)
			}
			if v, ok := updates["allocations"]; ok {
				current.Allocations = v.([]api.Allocation)
			}
			if v, ok := updates["assets"]; ok {
				current.Assets = v.([]api.Asset)
			}
			if err := portfolio.Validate(current); err != nil {
				return err
			}
		}
```

Wait — the above is messy. Here is the clean version. Find the block just before `client.MergeAndUpdate`:

```go
		var result api.Portfolio
		path := "/portfolios/" + args[0]
		if err := client.MergeAndUpdate(path, updates, &result); err != nil {
			outputError(err, 0)
		}
```

Replace with:

```go
		path := "/portfolios/" + args[0]

		// Validate merged state if allocation-sensitive fields are changing.
		if updates["allocations"] != nil || updates["assets"] != nil {
			var current api.Portfolio
			if err := client.Get(path, &current); err != nil {
				outputError(err, 0)
			}
			if v, ok := updates["allocations"]; ok {
				current.Allocations = v.([]api.Allocation)
			}
			if v, ok := updates["assets"]; ok {
				current.Assets = v.([]api.Asset)
			}
			if err := portfolio.Validate(current); err != nil {
				return err
			}
		}

		var result api.Portfolio
		if err := client.MergeAndUpdate(path, updates, &result); err != nil {
			outputError(err, 0)
		}
```

- [ ] **Step 3: Add import for portfolio package**

Ensure `cmd/portfolio.go` imports the portfolio package. Add to the import block:

```go
"github.com/mdmclean/kashmere-cli/internal/portfolio"
```

- [ ] **Step 4: Build to verify no compilation errors**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go build ./...
```

Expected: exits 0, no errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/portfolio.go
git commit -m "feat: validate allocations in CLI create/update"
```

---

## Task 3: Wire validation into MCP create/update

**Files:**
- Modify: `internal/mcp/portfolios.go`

- [ ] **Step 1: Add validation to `create_portfolio` handler**

In `internal/mcp/portfolios.go`, find the `create_portfolio` handler. Currently it builds `body` and calls `c.Post`. Before the `c.Post` call:

```go
		var created api.Portfolio
		if err := c.Post("/portfolios", body, &created); err != nil {
```

Add validation:

```go
		validateP := api.Portfolio{
			Allocations: in.Allocations,
			Assets:      in.Assets,
		}
		if err := portfolio.Validate(validateP); err != nil {
			return ErrResult(err), nil, nil
		}

		var created api.Portfolio
		if err := c.Post("/portfolios", body, &created); err != nil {
```

- [ ] **Step 2: Add validation to `update_portfolio` handler**

In the `update_portfolio` handler, find the block before `c.MergeAndUpdate`:

```go
		var updated api.Portfolio
		if err := c.MergeAndUpdate("/portfolios/"+in.ID, updates, &updated); err != nil {
```

Replace with:

```go
		path := "/portfolios/" + in.ID

		// Validate merged state if allocation-sensitive fields are changing.
		if in.Allocations != nil || in.Assets != nil {
			var current api.Portfolio
			if err := c.Get(path, &current); err != nil {
				return ErrResult(err), nil, nil
			}
			if in.Allocations != nil {
				current.Allocations = in.Allocations
			}
			if in.Assets != nil {
				current.Assets = in.Assets
			}
			if err := portfolio.Validate(current); err != nil {
				return ErrResult(err), nil, nil
			}
		}

		var updated api.Portfolio
		if err := c.MergeAndUpdate(path, updates, &updated); err != nil {
```

- [ ] **Step 3: Add import for portfolio package**

Ensure `internal/mcp/portfolios.go` imports the portfolio package. Add to the import block:

```go
"github.com/mdmclean/kashmere-cli/internal/portfolio"
```

- [ ] **Step 4: Build to verify no compilation errors**

```bash
cd /Users/michaelmclean/Documents/Code/Finances/kashmere-cli
go build ./...
```

Expected: exits 0, no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/portfolios.go
git commit -m "feat: validate allocations in MCP create/update"
```

---

## Task 4: Update skill doc

**Files:**
- Modify: `skills/kashmere.md`

- [ ] **Step 1: Add asset targets subsection**

In `skills/kashmere.md`, find the `### portfolio` section. After the flags list (ending with `--min-transaction-currency`), add a new subsection:

```markdown
#### Asset targets (`targetPercentage`)

Assets have an optional `targetPercentage` field. This is the asset's **share within its asset class**, not its share of the total portfolio.

Example — a portfolio with two allocation classes:

```json
"allocations": [
  { "category": "US Equity",       "percentage": 50 },
  { "category": "Canadian Equity", "percentage": 50 }
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
- `asset.targetPercentage` values must sum to 100% within each class — but only for classes where **all** assets have a target set. Partially-set targets are skipped.
```

- [ ] **Step 2: Commit**

```bash
git add skills/kashmere.md
git commit -m "docs: clarify asset targetPercentage semantics in kashmere skill"
```

---

## Self-Review

**Spec coverage:**
- [x] Check 1: allocation sum — Task 1 (`validateAllocationSum`)
- [x] Check 2: asset target sum per class, all-or-nothing — Task 1 (`validateAssetTargetSums`)
- [x] CLI create validation — Task 2
- [x] CLI update validation (merged state, skip if no allocation change) — Task 2
- [x] MCP create validation — Task 3
- [x] MCP update validation (merged state, skip if no allocation change) — Task 3
- [x] Error messages include actual sum — Task 1 (`%.2f%%`)
- [x] Skill doc update — Task 4
- [x] Float tolerance ±0.01 — Task 1 (`percentageTolerance`)
- [x] Empty allocations skipped — Task 1 test + impl
- [x] Partial targets skipped — Task 1 test + impl

**Placeholder scan:** No TBDs, TODOs, or vague steps. All code blocks are complete.

**Type consistency:**
- `portfolio.Validate(api.Portfolio)` — defined in Task 1, called in Tasks 2 and 3 with the same signature.
- `api.Portfolio{Allocations: ..., Assets: ...}` — struct fields match `internal/api/types.go`.
- `current.Allocations = v.([]api.Allocation)` — type assertion matches `updates["allocations"]` assignment in Task 2 which sets `updates["allocations"] = allocations` where `allocations` is `[]api.Allocation`.
- Same for `[]api.Asset`.
