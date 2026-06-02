"""
handlers/cloud.py - Cloudflare and external network handlers.
"""

import logging
from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd
import html

logger = logging.getLogger(__name__)

async def handle_cloud_menu(ctx, message: Message):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="☁️ Sync Cloudflare Services", callback_data="cloud_sync")],
        [InlineKeyboardButton(text="🔙 Back", callback_data="cancel_action")]
    ])
    await message.answer("☁️ <b>External Connectivity (Cloudflare)</b>\nSelect action:", reply_markup=kb)

async def cb_cloud(ctx, query: CallbackQuery):
    await query.answer()
    action = query.data.split("_", 1)[1]
    
    if action == "sync":
        await query.message.edit_text("☁️ <b>Syncing Cloudflare...</b>\nReconciling Tunnels and Zero Trust policies.")
        success, out, err = await run_kgg_cmd(["cloudflare", "sync"])
        msg = f"✅ <b>Cloudflare Sync Complete</b>\n\n<pre>{html.escape(out)}</pre>" if success else f"❌ <b>Cloudflare Sync Failed</b>\n\n<pre>{html.escape(err)}</pre>"
        await query.message.answer(msg)
