---
name: kashmere
description: Use when the user asks about their Kashmere financial data — portfolio
             values, allocation drift, trade recommendations, goal progress, mortgage
             details, cash flows, prices, or net worth.
---

# Kashmere

Kashmere is a financial planning app. Use the MCP server (preferred in Claude Code)
or the `kashmere` CLI directly.

## MCP Server (Claude Code)

The kashmere MCP server runs automatically when Claude Code starts from a directory
with a `.mcp.json` that includes the kashmere server. All tools are available
directly — no bash required.

**Available tools:** `get_dashboard`, `list_portfolios`, `get_portfolio`,
`create_portfolio`, `update_portfolio`, `delete_portfolio`, `list_institutions`,
`get_top_trades`, `list_goals`, `get_goal`, `create_goal`, `update_goal`,
`list_cashflows`, `get_cashflow`, `create_cashflow`, `update_cashflow`,
`list_mortgages`, `get_mortgage`, `create_mortgage`, `update_mortgage`,
`list_snapshots`, `create_snapshot`, `list_prices`, `get_settings`, `update_settings`

## CLI Setup

```bash
# Install
go install github.com/mdmclean/kashmere-cli@latest

# First-time setup (email/password)
kashmere setup --email your@email.com

# Set passphrase for agent use
export KASHMERE_PASSPHRASE="your-passphrase"
```

## CLI Commands

### portfolio

```bash
kashmere portfolio list
kashmere portfolio get <id>
kashmere portfolio create \
  --name "TFSA" \
  --institution "Questrade" \
  --owner person1 \
  --goal-id <goal-id> \
  --total-value 50000 \
  --allocations '[{"category":"US Equity","percentage":60},{"category":"Canadian Equity","percentage":40}]'
kashmere portfolio update <id> --total-value 55000
kashmere portfolio top-trades [portfolioId]
```

Flags for create/update:
- `--name`, `--description`, `--institution`
- `--owner` — `person1` | `person2` | `joint`
- `--management-type` — `self` (default) | `auto`
- `--goal-id` — ID of the goal this portfolio is assigned to
- `--total-value` — current total value in display currency
- `--allocations` — JSON array of `{category, percentage}` objects summing to 100
- `--assets` — JSON array of asset objects
- `--min-transaction-amount`, `--min-transaction-currency` — `CAD` | `USD`

#### Asset targets (`targetPercentage`)

Assets have an optional `targetPercentage` field — the asset's share **within its
asset class**, not its share of the total portfolio.

```json
"allocations": [
  { "category": "US Equity",       "percentage": 50 },
  { "category": "Canadian Equity", "percentage": 50 }
]
"assets": [
  { "ticker": "VTI", "category": "US Equity",       "targetPercentage": 60 },
  { "ticker": "VOO", "category": "US Equity",       "targetPercentage": 40 },
  { "ticker": "VCN", "category": "Canadian Equity", "targetPercentage": 100 }
]
```

VTI is 60% of US Equity (50% of portfolio) → VTI = **30% of total portfolio**.

**Rules enforced (hard errors):**
- `allocations[].percentage` must sum to 100%
- `asset.targetPercentage` must sum to 100% within each class (only when all assets in the class have a target set)

### goal

```bash
kashmere goal list
kashmere goal get <id>
kashmere goal create --name "Retirement" --target-type fixed --target-value 1000000
kashmere goal update <id> --target-value 1200000
```

Flags: `--name`, `--description`, `--target-type` (`fixed`|`percentage`), `--target-value`, `--allocations`

### cashflow

```bash
kashmere cashflow list [--portfolio-id <id>]
kashmere cashflow get <id>
kashmere cashflow create \
  --portfolio-id <id> \
  --type deposit \
  --amount 5000 \
  --date 2026-04-09
kashmere cashflow update <id> --amount 6000
```

Flags: `--portfolio-id`, `--type` (`deposit`|`withdrawal`), `--amount`, `--date` (YYYY-MM-DD), `--cash-asset-id`, `--description`

### mortgage

```bash
kashmere mortgage list
kashmere mortgage get <id>
kashmere mortgage create \
  --name "Main Home" \
  --owner joint \
  --institution "TD Bank" \
  --original-principal 600000 \
  --current-balance 520000 \
  --interest-rate 5.25 \
  --payment-amount 3200 \
  --payment-frequency monthly \
  --start-date 2022-01-01 \
  --term-end-date 2027-01-01 \
  --amortization-years 25
kashmere mortgage update <id> --current-balance 510000
```

Flags: `--name`, `--description`, `--owner`, `--institution`, `--original-principal`,
`--current-balance`, `--interest-rate`, `--payment-amount`,
`--payment-frequency` (`monthly`|`bi-weekly`|`accelerated-bi-weekly`|`weekly`),
`--start-date`, `--term-end-date` (YYYY-MM-DD), `--amortization-years`, `--extra-payment`

### dashboard

```bash
kashmere dashboard
```

Returns total portfolio value, weighted allocations, goal summaries, and net worth.

### history

```bash
kashmere history
kashmere history --from 2025-01-01 --to 2026-01-01
kashmere history --portfolio-id <id>
```

### price

```bash
kashmere price list
kashmere price list --symbols VCN,VFV,XEQT
```

### settings

```bash
kashmere settings get
kashmere settings update --person1-name "Alice" --display-currency CAD
```

Flags: `--person1-name`, `--person2-name`, `--account-type` (`single`|`couple`),
`--display-currency` (`CAD`|`USD`), `--onboarding-dismissed`

## Output

All commands output JSON to stdout. Use `--pretty` for readable output:

```bash
kashmere portfolio list --pretty
```

Errors go to stderr as JSON with a non-zero exit code:
```json
{"error": "API error 404: Portfolio not found", "status": 0}
```

## Notes

- All financial data is end-to-end encrypted. The CLI and MCP server handle this transparently.
- Write operations always fetch the full current object before updating (required by E2E encryption — the server cannot merge partial encrypted updates).
- `KASHMERE_PASSPHRASE` env var skips the interactive passphrase prompt.
- `KASHMERE_CONFIG` overrides the config file path (`~/.kashmere/config.json`).
- `KASHMERE_API_BASE_URL` overrides the API base URL.
