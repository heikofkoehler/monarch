import argparse
import asyncio
import csv
import json
import os
from pathlib import Path

import monarchmoney
from portfolio_utils import extract_holding_fields

async def sync_monarch_to_sheets(credentials_path: str = "credentials.json", out_file: str = "portfolio.json", csv_file: str = None):
    # 1. Connect to Monarch
    mm = monarchmoney.MonarchMoney()
    
    # Reading credentials from a secure location is recommended for production use
    # Try credentials.json first, then fall back to environment variables
    def load_credentials(path: str = 'credentials.json'):
        p = Path(path)
        if p.exists():
            with p.open('r') as fh:
                data = json.load(fh)
            return data.get('email'), data.get('password')
        # Fallback to env vars
        return os.environ.get('MONARCH_EMAIL'), os.environ.get('MONARCH_PASSWORD')

    email, password = load_credentials(credentials_path)
    if not email or not password:
        raise RuntimeError(
            "Credentials not found. Create credentials.json with {'email':..., 'password':...} or set MONARCH_EMAIL and MONARCH_PASSWORD env vars."
        )

    try:
        await mm.login(email=email, password=password, save_session=False, use_saved_session=False)
    except monarchmoney.RequireMFAException as e:
        print(f"Failed to login, fail back to MFA: {e}")
        await mm.multi_factor_authenticate(email, password,
                                           input("Two Factor Code: "))  # This will prompt you to complete MFA in the console
    except Exception as e:
        print(f"Failed to login: {e}")
        raise

    # 2. Fetch entire investment portfolio and save to JSON
    portfolio = await mm.get_portfolio()
    with open(out_file, "w") as outfile:
        json.dump(portfolio, outfile, indent=4)
    
    # 3. Write to CSV file for easier consumption (optional, if csv_file is specified)
    if csv_file:
        write_portfolio_to_csv(portfolio, csv_file)

    print("Sync Complete!")


def write_portfolio_to_csv(portfolio: dict, csv_file: str) -> None:
    """
    Extracts holdings from portfolio and writes to CSV.
    Flattens nested structure for easier consumption.
    """
    holdings_list = []
    
    # Navigate through the portfolio structure to extract holdings
    if "portfolio" in portfolio and "aggregateHoldings" in portfolio["portfolio"]:
        edges = portfolio["portfolio"]["aggregateHoldings"].get("edges", [])
        for edge in edges:
            node = edge.get("node", {})
            security_info = node.get("security", {})
            holdings = node.get("holdings", [])
            
            for holding in holdings:
                record = extract_holding_fields(holding, security_info)
                holdings_list.append(record)
    
    if not holdings_list:
        print(f"No holdings found to write to CSV")
        return
    
    fieldnames = holdings_list[0].keys()
    with open(csv_file, "w", newline="") as csvfile:
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(holdings_list)
    
    print(f"Wrote {len(holdings_list)} holdings to {csv_file}")


def parse_args():
    parser = argparse.ArgumentParser(description="Fetch Monarch portfolio and save to JSON")
    parser.add_argument(
        "-c",
        "--credentials",
        default="credentials.json",
        help="Path to credentials JSON file (default: credentials.json)",
    )
    parser.add_argument(
        "-o",
        "--out",
        default="portfolio.json",
        help="Output JSON filename (default: portfolio.json)",
    )
    parser.add_argument(
        "--csv",
        default=None,
        help="Output CSV filename for holdings (optional)",
    )
    return parser.parse_args()


if __name__ == "__main__":
    args = parse_args()
    asyncio.run(sync_monarch_to_sheets(credentials_path=args.credentials, out_file=args.out, csv_file=args.csv))