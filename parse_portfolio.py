import argparse
import json
import pandas as pd

from portfolio_utils import extract_holding_fields


def process_portfolio(json_file):
    with open(json_file, 'r') as f:
        data = json.load(f)

    # Navigate through the GraphQL structure to the edges
    edges = data.get('portfolio', {}).get('aggregateHoldings', {}).get('edges', [])

    holdings_list = []

    for edge in edges:
        node = edge.get('node', {})
        # Each node contains an array of holdings across different accounts
        individual_holdings = node.get('holdings', [])
        
        for holding in individual_holdings:
            security_info = node.get('security', {})
            extracted_data = extract_holding_fields(holding, security_info)
            holdings_list.append(extracted_data)

    # Convert to DataFrame and sort by value for better readability
    df = pd.DataFrame(holdings_list)
    df = df.sort_values(by='value', ascending=False)
    
    return df

def parse_args():
    parser = argparse.ArgumentParser(description="Parse Monarch portfolio JSON and output as CSV/Markdown")
    parser.add_argument(
        "-i",
        "--input",
        default="portfolio.json",
        help="Input JSON portfolio file (default: portfolio.json)",
    )
    parser.add_argument(
        "-o",
        "--output",
        default="portfolio_holdings.csv",
        help="Output CSV filename (default: portfolio_holdings.csv)",
    )
    parser.add_argument(
        "--markdown",
        action="store_true",
        help="Display output as markdown table (default: false)",
    )
    return parser.parse_args()


if __name__ == "__main__":
    args = parse_args()
    df_holdings = process_portfolio(args.input)
    
    if args.markdown:
        print(df_holdings.to_markdown(index=False))
    
    df_holdings.to_csv(args.output, index=False)
    print(f"Saved {len(df_holdings)} holdings to {args.output}")