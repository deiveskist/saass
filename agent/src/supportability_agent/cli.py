"""Entry point: `python -m supportability_agent.cli <ticket_id>`."""

from __future__ import annotations

import argparse
import asyncio

from dotenv import load_dotenv

from .investigate import investigate


def main() -> None:
    load_dotenv()

    parser = argparse.ArgumentParser(
        description="Investigate a support ticket using the supportability-mcp tools."
    )
    parser.add_argument("ticket_id", help="External ticket ID (e.g. Zendesk ID)")
    args = parser.parse_args()

    result = asyncio.run(investigate(args.ticket_id))
    print(result)


if __name__ == "__main__":
    main()
