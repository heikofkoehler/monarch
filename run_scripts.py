#!/usr/bin/env python3
"""
Wrapper script to execute Monarch Money portfolio scripts within the venv.
Provides a unified interface for running getportfolio.py and parse_portfolio.py.
"""

import argparse
import subprocess
import sys
import os
from pathlib import Path


def get_venv_python():
    """Get the path to the Python interpreter in the virtual environment."""
    venv_path = Path(__file__).parent / ".venv"
    python_path = venv_path / "bin" / "python"
    
    if not python_path.exists():
        raise RuntimeError(
            f"Virtual environment not found at {venv_path}. "
            "Please create it with: python3 -m venv .venv"
        )
    return str(python_path)


def run_getportfolio(
    credentials: str = "credentials.json",
    output: str = "portfolio.json",
    csv: str = None,
) -> int:
    """
    Execute getportfolio.py to fetch portfolio from Monarch Money API.
    
    Args:
        credentials: Path to credentials JSON file (default: credentials.json)
        output: Output path for portfolio JSON (default: portfolio.json)
        csv: Optional path to save holdings as CSV
    
    Returns:
        Exit code from the script
    """
    venv_python = get_venv_python()
    script_path = Path(__file__).parent / "getportfolio.py"
    
    cmd = [venv_python, str(script_path)]
    cmd.extend(["-c", credentials])
    cmd.extend(["-o", output])
    
    if csv:
        cmd.extend(["--csv", csv])
    
    print(f"Running: {' '.join(cmd)}")
    return subprocess.call(cmd)


def run_parse_portfolio(
    input_file: str = "portfolio.json",
    output: str = "portfolio_holdings.csv",
    markdown: bool = False,
) -> int:
    """
    Execute parse_portfolio.py to parse portfolio JSON file.
    
    Args:
        input_file: Input portfolio JSON file (default: portfolio.json)
        output: Output file path (default: portfolio_holdings.csv)
        markdown: If True, output in Markdown format instead of CSV
    
    Returns:
        Exit code from the script
    """
    venv_python = get_venv_python()
    script_path = Path(__file__).parent / "parse_portfolio.py"
    
    cmd = [venv_python, str(script_path)]
    cmd.extend(["-i", input_file])
    cmd.extend(["-o", output])
    
    if markdown:
        cmd.append("--markdown")
    
    print(f"Running: {' '.join(cmd)}")
    return subprocess.call(cmd)


def run_full_pipeline(
    credentials: str = "credentials.json",
    portfolio_json: str = "portfolio.json",
    portfolio_csv: str = "portfolio_holdings.csv",
    portfolio_md: str = "portfolio_holdings.md",
    skip_fetch: bool = False,
) -> int:
    """
    Execute the full pipeline: fetch portfolio and parse it to CSV and Markdown.
    
    Args:
        credentials: Path to credentials JSON file
        portfolio_json: Intermediate portfolio JSON file
        portfolio_csv: Output CSV file for holdings
        portfolio_md: Output Markdown file for holdings
        skip_fetch: If True, skip fetching and only parse existing JSON
    
    Returns:
        Exit code (0 if successful, non-zero on error)
    """
    if not skip_fetch:
        print("\n=== Step 1: Fetching portfolio from Monarch Money ===")
        fetch_code = run_getportfolio(
            credentials=credentials,
            output=portfolio_json,
            csv=None,
        )
        if fetch_code != 0:
            print(f"Error fetching portfolio (exit code: {fetch_code})")
            return fetch_code
    
    print("\n=== Step 2: Parsing portfolio to CSV ===")
    csv_code = run_parse_portfolio(
        input_file=portfolio_json,
        output=portfolio_csv,
        markdown=False,
    )
    if csv_code != 0:
        print(f"Error parsing to CSV (exit code: {csv_code})")
        return csv_code
    
    print("\n=== Step 3: Parsing portfolio to Markdown ===")
    md_code = run_parse_portfolio(
        input_file=portfolio_json,
        output=portfolio_md,
        markdown=True,
    )
    if md_code != 0:
        print(f"Error parsing to Markdown (exit code: {md_code})")
        return md_code
    
    print("\n=== Pipeline completed successfully ===")
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Wrapper to execute Monarch Money scripts in the virtual environment"
    )
    
    subparsers = parser.add_subparsers(dest="command", help="Command to execute")
    
    # fetch subcommand
    fetch_parser = subparsers.add_parser(
        "fetch", help="Fetch portfolio from Monarch Money API"
    )
    fetch_parser.add_argument(
        "-c", "--credentials",
        default="credentials.json",
        help="Path to credentials JSON file (default: credentials.json)",
    )
    fetch_parser.add_argument(
        "-o", "--output",
        default="portfolio.json",
        help="Output JSON file path (default: portfolio.json)",
    )
    fetch_parser.add_argument(
        "--csv",
        help="Optional path to save holdings as CSV",
    )
    
    # parse subcommand
    parse_parser = subparsers.add_parser(
        "parse", help="Parse portfolio JSON file"
    )
    parse_parser.add_argument(
        "-i", "--input",
        default="portfolio.json",
        help="Input portfolio JSON file (default: portfolio.json)",
    )
    parse_parser.add_argument(
        "-o", "--output",
        default="portfolio_holdings.csv",
        help="Output file path (default: portfolio_holdings.csv)",
    )
    parse_parser.add_argument(
        "--markdown",
        action="store_true",
        help="Output in Markdown format instead of CSV",
    )
    
    # pipeline subcommand
    pipeline_parser = subparsers.add_parser(
        "pipeline", help="Execute full fetch and parse pipeline"
    )
    pipeline_parser.add_argument(
        "-c", "--credentials",
        default="credentials.json",
        help="Path to credentials JSON file (default: credentials.json)",
    )
    pipeline_parser.add_argument(
        "--portfolio-json",
        default="portfolio.json",
        help="Portfolio JSON file path (default: portfolio.json)",
    )
    pipeline_parser.add_argument(
        "--portfolio-csv",
        default="portfolio_holdings.csv",
        help="Output CSV file path (default: portfolio_holdings.csv)",
    )
    pipeline_parser.add_argument(
        "--portfolio-md",
        default="portfolio_holdings.md",
        help="Output Markdown file path (default: portfolio_holdings.md)",
    )
    pipeline_parser.add_argument(
        "--skip-fetch",
        action="store_true",
        help="Skip fetching, only parse existing portfolio JSON",
    )
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return 0
    
    try:
        if args.command == "fetch":
            return run_getportfolio(
                credentials=args.credentials,
                output=args.output,
                csv=args.csv,
            )
        elif args.command == "parse":
            return run_parse_portfolio(
                input_file=args.input,
                output=args.output,
                markdown=args.markdown,
            )
        elif args.command == "pipeline":
            return run_full_pipeline(
                credentials=args.credentials,
                portfolio_json=args.portfolio_json,
                portfolio_csv=args.portfolio_csv,
                portfolio_md=args.portfolio_md,
                skip_fetch=args.skip_fetch,
            )
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
