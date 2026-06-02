"""
handlers/storage.py - Distributed storage and backup handlers.
"""

import logging
from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd
import html

logger = logging.getLogger(__name__)

async def handle_storage_menu(ctx, message: Message):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="💾 Longhorn Status", callback_data="storage_status")],
        [InlineKeyboardButton(text="🔋 Trigger System Backup", callback_data="storage_backup")],
        [InlineKeyboardButton(text="🔙 Back", callback_data="cancel_action")]
    ])
    await message.answer("💾 <b>Storage & Disaster Recovery</b>\nSelect action:", reply_markup=kb)

async def cb_storage(ctx, query: CallbackQuery):
    await query.answer()
    action = query.data.split("_", 1)[1]
    
    if action == "status":
        await query.message.edit_text("⏳ Checking Longhorn status (via Ansible)...")
        success, out, err = await run_kgg_cmd(["storage", "status"])
        msg = f"💾 <b>Storage Status</b>\n\n<pre>{html.escape(out)}</pre>" if success else f"❌ <b>Storage Check Failed</b>\n\n<pre>{html.escape(err)}</pre>"
        await query.message.answer(msg)

    elif action == "backup":
        await query.message.edit_text("🔋 <b>Triggering Backup...</b>\nHardware LEDs will pulse during the process.")
        success, out, err = await run_kgg_cmd(["app", "backup"])
        msg = f"✅ <b>Backup Process Started</b>\n\n<pre>{html.escape(out)}</pre>" if success else f"❌ <b>Backup Failed</b>\n\n<pre>{html.escape(err)}</pre>"
        await query.message.answer(msg)
