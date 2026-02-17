# Monarch Money Portfolio Tools

Tools to fetch and analyze your Monarch Money investment portfolio.

## Overview

This project provides utilities to:
- **Fetch portfolio data** from Monarch Money API and save to JSON
- **Parse portfolio JSON** and export holdings to CSV or Markdown format
- **Analyze portfolio holdings** with consistent field extraction across scripts

## Setup

### Prerequisites
- Python 3.9+
- Monarch Money account

### Installation

1. Clone the repository:
```bash
git clone git@github.com:heikofkoehler/monarch.git
cd monarch
```

2. Install dependencies:
```bash
python3 -m pip install -r requirements.txt
```

### Configuration

Create a `credentials.json` file in the project root:
```json
{
  "email": "your-email@example.com",
  "password": "your-password"
}
```

Alternatively, set environment variables:
```bash
export MONARCH_EMAIL="your-email@example.com"
export MONARCH_PASSWORD="your-password"
```

## Usage

### Fetch Portfolio (`getportfolio.py`)

Fetch your portfolio from Monarch Money and save to JSON (optionally CSV).

```bash
# Default: saves to portfolio.json
python3 getportfolio.py

# With custom credentials file
python3 getportfolio.py -c mycreds.json

# Save JSON and CSV
python3 getportfolio.py -o out.json --csv holdings.csv

# All options
python3 getportfolio.py -c mycreds.json -o portfolio.json --csv holdings.csv
```

**Options:**
- `-c/--credentials` (default: credentials.json) — Path to credentials JSON file
- `-o/--out` (default: portfolio.json) — Output JSON filename
- `--csv` (optional) — Output CSV filename for holdings

### Parse Portfolio (`parse_portfolio.py`)

Parse a portfolio JSON file and export holdings to CSV or display as Markdown.

```bash
# Default: reads portfolio.json, writes portfolio_holdings.csv
python3 parse_portfolio.py

# Custom input/output
python3 parse_portfolio.py -i my_portfolio.json -o my_holdings.csv

# Display markdown table (and save CSV)
python3 parse_portfolio.py --markdown

# All options
python3 parse_portfolio.py -i data.json -o output.csv --markdown
```

**Options:**
- `-i/--input` (default: portfolio.json) — Input JSON portfolio file
- `-o/--output` (default: portfolio_holdings.csv) — Output CSV filename
- `--markdown` — Display output as markdown table (optional)

## Data Fields

Both scripts extract the following fields per holding:

| Field | Description |
|-------|-------------|
| account_id | Account UUID |
| account_name | Account display name |
| account_mask | Last 4 digits or masked identifier |
| institution_name | Financial institution name |
| holding_name | Security/holding name |
| ticker | Stock/fund ticker symbol |
| type | Security type (e.g., stock, mutual fund) |
| type_display | Human-readable security type |
| quantity | Number of shares held |
| closing_price | Last closing price |
| value | Total value of holding |
| security_id | Security UUID |
| security_name | Security name |
| security_ticker | Security ticker |
| current_price | Current market price |
| price_updated | Timestamp of last price update |

## Shared Utilities (`portfolio_utils.py`)

The `extract_holding_fields()` function is shared between scripts to ensure consistent data extraction.

## Example Workflow

```bash
# 1. Fetch latest portfolio data
python3 getportfolio.py -c credentials.json -o portfolio.json --csv holdings.csv

# 2. Parse and display portfolio
python3 parse_portfolio.py -i portfolio.json --markdown

# 3. Create analysis-ready CSV
python3 parse_portfolio.py -i portfolio.json -o analysis.csv
```

## Requirements

- asyncio
- aiohttp>=3.8.4
- gql>=4.0
- oathtool>=2.3.1
- pandas

## Project Structure

```
monarch/
├── getportfolio.py           # Fetch portfolio from Monarch API
├── parse_portfolio.py         # Parse portfolio JSON to CSV/Markdown
├── portfolio_utils.py         # Shared utility functions
├── monarchmoney.py            # Monarch Money API client
├── main.py                    # Entry point
├── __init__.py
├── requirements.txt           # Python dependencies
├── queries/                   # GraphQL query files
│   └── Web_GetPortfolio.gql  # Portfolio query
└── README.md                  # This file
```

## Security Notes

- **Never commit credentials.json to version control** — it will be ignored by default (add to .gitignore)
- Use environment variables for automated runs
- Store credentials in a secure location
- Consider using a `.env` file with tools like `python-dotenv` for local development

## License

MIT

## Contributing

Contributions welcome! Please fork and submit pull requests.
