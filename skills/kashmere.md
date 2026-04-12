---
name: kashmere
description: Use when the user wants to manage their Kashmere finances — portfolios,
             goals, cash flows, mortgages, settings, prices, or history. Handles
             installation, setup, and all financial operations via the kashmere CLI.
---

# Kashmere CLI

Kashmere is a financial planning app. This skill helps you use the `kashmere`
CLI to manage accounts, portfolios, goals, and more.

## Installation

Download the binary for your platform from:
https://github.com/mdmclean/kashmere-cli/releases/latest

Or build from source (requires Go 1.22+):
```bash
git clone https://github.com/mdmclean/kashmere-cli
cd kashmere-cli
go build -o kashmere .
```

## First-time Setup

```bash
# Email/password users (no browser needed):
kashmere setup --email your@email.com

# OAuth/browser users:
kashmere setup
# Opens https://kashmere.app in browser. Complete login, then return here.
```

Setup stores `~/.kashmere/config.json` with your API key and encryption salt.
Your passphrase is NEVER stored — provide it via env var for agent use:

```bash
export KASHMERE_PASSPHRASE="your-passphrase"
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `KASHMERE_PASSPHRASE` | Encryption passphrase (skips interactive prompt) |
| `KASHMERE_CONFIG` | Override config file path |
| `KASHMERE_API_BASE_URL` | Override API base URL |

## Commands

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
```

Flags for create/update:
- `--name` — portfolio name
- `--description` — optional description
- `--institution` — institution name (Questrade, Wealthsimple, etc.)
- `--owner` — `person1` | `person2` | `joint`
- `--management-type` — `self` (default) | `auto`
- `--goal-id` — ID of the goal this portfolio is assigned to
- `--total-value` — current total value in display currency
- `--allocations` — JSON array of `{category, percentage}` objects summing to 100
- `--assets` — JSON array of asset objects
- `--min-transaction-amount` — minimum trade size
- `--min-transaction-currency` — `CAD` | `USD`

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
  { "ticker": "VOO", "category": "US Equity",       "targetPercentage": 40 },
  { "ticker": "VCN", "category": "Canadian Equity", "targetPercentage": 100 }
]
```

VTI is 60% of the US Equity class, which is 50% of the portfolio → VTI = **30% of total portfolio**.

**Rules enforced by the CLI/MCP (hard errors):**

- `allocations[].percentage` must sum to 100%.
- `asset.targetPercentage` values must sum to 100% within each class — but only for classes where **all** assets have a target set. Partially-set targets are skipped.

### goal

```bash
kashmere goal list
kashmere goal get <id>
kashmere goal create --name "Retirement" --target-type fixed --target-value 1000000
kashmere goal update <id> --target-value 1200000
```

Flags for create/update:
- `--name` — goal name
- `--description` — optional description
- `--target-type` — `fixed` (dollar amount) | `percentage` (% of total portfolio)
- `--target-value` — target value (dollars when fixed, 0–100 when percentage)
- `--allocations` — target allocation JSON array

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

Flags for create/update:
- `--portfolio-id` — target portfolio
- `--type` — `deposit` | `withdrawal`
- `--amount` — transaction amount
- `--date` — date in `YYYY-MM-DD` format
- `--cash-asset-id` — cash asset within the portfolio
- `--description` — optional note

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

Flags for create/update:
- `--name`, `--description`, `--owner` (`person1`|`person2`|`joint`), `--institution`
- `--original-principal`, `--current-balance`, `--interest-rate`
- `--payment-amount`, `--payment-frequency` (`monthly`|`bi-weekly`|`accelerated-bi-weekly`|`weekly`)
- `--start-date`, `--term-end-date` (YYYY-MM-DD)
- `--amortization-years`, `--extra-payment`

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

Flags for update:
- `--person1-name`, `--person2-name`
- `--account-type` (`single`|`couple`)
- `--display-currency` (`CAD`|`USD`)
- `--onboarding-dismissed`

## Output

All commands output JSON to stdout. Use `--pretty` for readable output:

```bash
kashmere portfolio list --pretty
```

Errors are printed to stderr as JSON and exit with a non-zero status:

```json
{"error": "API error 404: Portfolio not found", "status": 0}
```

## Notes

- All financial data is end-to-end encrypted. The CLI handles this transparently.
- For write operations, the CLI always fetches the full current object before updating
  (required by E2E encryption — the server cannot merge partial encrypted updates).
