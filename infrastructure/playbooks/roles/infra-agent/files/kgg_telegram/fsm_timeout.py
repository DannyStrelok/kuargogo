"""
fsm_timeout.py - Background task to automatically timeout inactive FSM command sessions.
"""

import asyncio
import logging
import time

from aiogram.fsm.storage.base import StorageKey
from aiogram.fsm.context import FSMContext

from kgg_telegram.config import TG_ADMIN_ID

logger = logging.getLogger(__name__)


async def fsm_timeout_loop(bot):
    """Periodically check if the authorized user has an active FSM state and if it is inactive.

    Clears the FSM state and notifies the user after 5 minutes of inactivity.
    """
    logger.info("FSM Inactivity Timeout daemon started (Timeout: 5 minutes)")
    while True:
        try:
            await asyncio.sleep(15)
            if not TG_ADMIN_ID:
                continue

            # Check if there is an active FSM state
            key = StorageKey(bot_id=bot.bot.id, chat_id=TG_ADMIN_ID, user_id=TG_ADMIN_ID)
            state = FSMContext(storage=bot.dp.storage, key=key)
            current_state = await state.get_state()

            if current_state is not None:
                inactive_sec = time.time() - bot.last_activity_time
                if inactive_sec > 300:
                    logger.info(
                        f"Clearing FSM state {current_state} for admin {TG_ADMIN_ID} "
                        f"due to {inactive_sec:.1f}s inactivity."
                    )
                    await state.clear()
                    await bot.bot.send_message(
                        chat_id=TG_ADMIN_ID,
                        text=(
                            "🔌 <b>Session Closed</b>\n"
                            "Your active command session has been closed due to "
                            "5 minutes of inactivity."
                        )
                    )
        except Exception as e:
            logger.error(f"Error in FSM timeout loop: {e}", exc_info=True)
