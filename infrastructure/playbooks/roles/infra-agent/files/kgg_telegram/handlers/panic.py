"""
handlers/panic.py - Telegram handlers for triggering and restoring homelab panic isolation.
"""

import logging
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)


async def handle_panic_menu(ctx, message: Message):
    msg = (
        "🚨 <b>HOMELAB PANIC SYSTEM</b> 🚨\n\n"
        "You are about to access the network and cluster killswitch interface.\n\n"
        "• <b>Isolate:</b> Pauses GitOps auto-sync, scales Cloudflare tunnel to 0, and disables or quarantines the Uplink switch port.\n"
        "• <b>Restore:</b> Re-enables the Uplink port, scales Cloudflare back to 1, and re-activates GitOps auto-sync.\n\n"
        "Please select an option:"
    )

    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🚨 CONFIRM PANIC ISOLATION", callback_data="panic_trigger_confirm")],
        [InlineKeyboardButton(text="🔓 Restore Normal Operations", callback_data="panic_restore_confirm")]
    ])
    await message.answer(msg, reply_markup=kb)


async def cb_panic(ctx, query: CallbackQuery):
    action = query.data

    if action == "panic_trigger_confirm":
        await query.answer("Initiating homelab panic isolation...", show_alert=True)
        await query.message.edit_text("⏳ <b>Executing Panic Isolation...</b>\nPlease wait while the homelab is isolated.")

        success, out, err = await run_kgg_cmd(["network", "panic", "--confirm"])
        if success:
            result = f"🚨 <b>Homelab Isolated Successfully!</b>\n\n<pre>{html.escape(out)}</pre>"
        else:
            result = f"❌ <b>Panic Isolation Failed!</b>\n\nError: <pre>{html.escape(err or out)}</pre>"

        kb = InlineKeyboardMarkup(inline_keyboard=[[
            InlineKeyboardButton(text="⬅️ Back to Menu", callback_data="panic_back")
        ]])
        await query.message.edit_text(result, reply_markup=kb)

    elif action == "panic_restore_confirm":
        await query.answer("Initiating recovery...", show_alert=True)
        await query.message.edit_text("⏳ <b>Restoring Homelab from Panic Mode...</b>\nPlease wait while operations are normalized.")

        success, out, err = await run_kgg_cmd(["network", "panic", "restore"])
        if success:
            result = f"🔓 <b>Normal Operations Restored!</b>\n\n<pre>{html.escape(out)}</pre>"
        else:
            result = f"❌ <b>Restoration Failed!</b>\n\nError: <pre>{html.escape(err or out)}</pre>"

        kb = InlineKeyboardMarkup(inline_keyboard=[[
            InlineKeyboardButton(text="⬅️ Back to Menu", callback_data="panic_back")
        ]])
        await query.message.edit_text(result, reply_markup=kb)

    elif action == "panic_back":
        await query.answer()
        # Edit the message to show the initial menu
        msg = (
            "🚨 <b>HOMELAB PANIC SYSTEM</b> 🚨\n\n"
            "You are about to access the network and cluster killswitch interface.\n\n"
            "Please select an option:"
        )
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="🚨 CONFIRM PANIC ISOLATION", callback_data="panic_trigger_confirm")],
            [InlineKeyboardButton(text="🔓 Restore Normal Operations", callback_data="panic_restore_confirm")]
        ])
        await query.message.edit_text(msg, reply_markup=kb)
