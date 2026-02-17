"""Shared utilities for portfolio data extraction and formatting."""


def extract_holding_fields(holding: dict, security_info: dict) -> dict:
    """
    Extract standardized fields from a holding.
    Used by both getportfolio.py and parse_portfolio.py for consistency.
    """
    account_info = holding.get('account', {})
    institution_info = account_info.get('institution', {})
    
    return {
        'account_id': account_info.get('id', ''),
        'account_name': account_info.get('displayName', ''),
        'account_mask': account_info.get('mask', ''),
        'institution_name': institution_info.get('name', ''),
        'holding_name': holding.get('name', ''),
        'ticker': holding.get('ticker', ''),
        'type': holding.get('type', ''),
        'type_display': holding.get('typeDisplay', ''),
        'quantity': holding.get('quantity', 0),
        'closing_price': holding.get('closingPrice', 0),
        'value': holding.get('value', 0),
        'security_id': security_info.get('id', ''),
        'security_name': security_info.get('name', ''),
        'security_ticker': security_info.get('ticker', ''),
        'current_price': security_info.get('currentPrice', 0),
        'price_updated': security_info.get('currentPriceUpdatedAt', ''),
    }
