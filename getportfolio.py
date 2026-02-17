import asyncio
import json
import os
from pathlib import Path

import monarchmoney

async def sync_monarch_to_sheets():
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

    email, password = load_credentials()
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
    with open("portfolio.json", "w") as outfile:
        json.dump(portfolio, outfile, indent=4)

    print("Sync Complete!")

if __name__ == "__main__":
    asyncio.run(sync_monarch_to_sheets()) 