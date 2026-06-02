"""Entry point: python -m kgg_telegram"""

import asyncio
import logging

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("kgg-telegram")

from kgg_telegram import RackBot

if __name__ == "__main__":
    try:
        logger.info("=== Kuargogo Bot Bridge Initializing ===")
        bot = RackBot()
        asyncio.run(bot.start())
    except KeyboardInterrupt:
        logger.info("Bot stopped by user.")
    except Exception as e:
        logger.critical(f"FATAL: Bot failed to start: {e}", exc_info=True)
        raise SystemExit(1)
