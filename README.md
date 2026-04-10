# kashmere-cli

Command-line interface and Claude Code skill for [Kashmere](https://kashmere.app) —
a financial planning app. Designed for humans and AI agents.

## Quick Start

### Download

Grab the latest binary for your platform from the
[Releases page](https://github.com/mdmclean/kashmere-cli/releases/latest).

### Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/mdmclean/kashmere-cli
cd kashmere-cli
go build -o kashmere .
```

### Setup

```bash
# Email/password (no browser required — great for agents):
kashmere setup --email your@email.com

# OAuth (opens browser):
kashmere setup
```

## Agent Usage

Set `KASHMERE_PASSPHRASE` to skip the interactive passphrase prompt:

```bash
export KASHMERE_PASSPHRASE="your-passphrase"
kashmere portfolio list
```

For Claude Code agents, load the skill from this repo:
`skills/kashmere.md`

## Commands

```
kashmere setup
kashmere portfolio list|get|create|update
kashmere goal list|get|create|update
kashmere cashflow list|get|create|update
kashmere mortgage list|get|create|update
kashmere dashboard
kashmere history [--from <date>] [--to <date>]
kashmere price list [--symbols <csv>]
kashmere settings get|update
```

Use `--help` on any command for full flag documentation.
Use `--pretty` for human-readable JSON output.

## Security

All financial data is end-to-end encrypted using AES-256-GCM. Your passphrase
never leaves your machine. The API key grants access to your encrypted data,
but cannot decrypt it without your passphrase.