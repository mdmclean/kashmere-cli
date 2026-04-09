# kashemere-cli

Command-line interface and Claude Code skill for [Kashemere](https://kashemere.app) —
a financial planning app. Designed for humans and AI agents.

## Quick Start

### Download

Grab the latest binary for your platform from the
[Releases page](https://github.com/mdmclean/kashmere-cli/releases/latest).

### Build from source

Requires Go 1.22+.

```bash
git clone https://github.com/mdmclean/kashmere-cli
cd kashemere-cli
go build -o kashemere .
```

### Setup

```bash
# Email/password (no browser required — great for agents):
kashemere setup --email your@email.com

# OAuth (opens browser):
kashemere setup
```

## Agent Usage

Set `KASHEMERE_PASSPHRASE` to skip the interactive passphrase prompt:

```bash
export KASHEMERE_PASSPHRASE="your-passphrase"
kashemere portfolio list
```

For Claude Code agents, load the skill from this repo:
`skills/kashemere.md`

## Commands

```
kashemere setup
kashemere portfolio list|get|create|update
kashemere goal list|get|create|update
kashemere cashflow list|get|create|update
kashemere mortgage list|get|create|update
kashemere dashboard
kashemere history [--from <date>] [--to <date>]
kashemere price list [--symbols <csv>]
kashemere settings get|update
```

Use `--help` on any command for full flag documentation.
Use `--pretty` for human-readable JSON output.

## Security

All financial data is end-to-end encrypted using AES-256-GCM. Your passphrase
never leaves your machine. The API key grants access to your encrypted data,
but cannot decrypt it without your passphrase.

## MCP Server

For MCP-native agents, Kashemere also provides an MCP server. See the main
app repository for configuration instructions.
