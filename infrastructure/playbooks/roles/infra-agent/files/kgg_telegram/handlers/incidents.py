"""
handlers/incidents.py - Incident history and AI analysis handler.
"""

import logging

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton
from aiogram.fsm.storage.base import StorageKey

from kgg_telegram.config import TG_ADMIN_ID
from kgg_telegram.db import fetch_incidents

logger = logging.getLogger(__name__)


async def handle_incidents(ctx, message: Message):
    incidents = fetch_incidents()
    if not incidents:
        await message.answer("✅ <b>No active incidents in the database.</b>")
        return

    msg = "📋 <b>Active Incidents Report</b>\n\n"
    for inc in incidents:
        ts, lvl, node, txt = inc
        icon = "🔴" if lvl == "CRITICAL" else "🟡"
        msg += f"{icon} <b>[{ts}]</b> {node}: {txt}\n"

    raw_txt = "\n".join([f"{lvl} on {node}: {txt}" for _, lvl, node, txt in incidents])
    key = StorageKey(bot_id=ctx.bot.id, chat_id=TG_ADMIN_ID, user_id=TG_ADMIN_ID)
    await ctx.dp.storage.update_data(key=key, data={"last_analysis_text": raw_txt})

    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🤖 Explain all with AI", callback_data="ai_analyze_incidents")],
        [InlineKeyboardButton(text="🧹 Clear Incidents", callback_data="confirm_clear_incidents")]
    ])
    await message.answer(msg, reply_markup=kb)


async def cb_clear_incidents(ctx, query: CallbackQuery):
    await query.answer()
    from kgg_telegram.db import clear_incidents
    clear_incidents()
    await query.message.edit_text("🧹 <b>All incidents have been cleared from the database.</b>")
