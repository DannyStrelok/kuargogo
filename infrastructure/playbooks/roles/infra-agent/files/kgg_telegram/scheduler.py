"""
scheduler.py - Daily summary scheduler for kgg_telegram.
"""

import asyncio
import datetime
import logging

from kgg_telegram.config import TG_TIMEZONE, TG_TIMEZONE_STR, TG_SUMMARY_TIME, TG_ADMIN_ID, TG_MAINTENANCE_MODE
from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)


async def scheduler_loop(ctx):
    """Send a proactive AI health summary at the configured daily time."""
    logger.info(f"Daily Summary Scheduler started. Target time: {TG_SUMMARY_TIME}")

    while True:
        now = datetime.datetime.now(TG_TIMEZONE)
        try:
            hour, minute = map(int, TG_SUMMARY_TIME.split(":"))
        except Exception as e:
            logger.error(
                f"Invalid TG_SUMMARY_TIME config '{TG_SUMMARY_TIME}': {e}. Defaulting to 08:30"
            )
            hour, minute = 8, 30

        target_time = now.replace(hour=hour, minute=minute, second=0, microsecond=0)
        if now >= target_time:
            target_time += datetime.timedelta(days=1)

        wait_seconds = (target_time - now).total_seconds()
        logger.info(f"Next daily summary in {wait_seconds/3600:.2f} hours (at {target_time})")

        await asyncio.sleep(wait_seconds)

        if TG_MAINTENANCE_MODE:
            logger.info("Maintenance Mode is ACTIVE. Skipping scheduled proactive health report.")
            continue

        try:
            logger.info("Triggering Proactive Daily Health Summary...")
            success, out, err = await run_kgg_cmd(["infra", "heartbeat", "--ai"])
            msg = (
                f"🌅 <b>Daily Cluster Status ({TG_SUMMARY_TIME} {TG_TIMEZONE_STR})</b>\n\n{out}"
                if success
                else f"🌅 <b>Daily Health Failed:</b>\n{err}"
            )
            await ctx.bot.send_message(TG_ADMIN_ID, msg)
        except Exception as e:
            logger.error(f"Failed to send daily summary: {e}")
