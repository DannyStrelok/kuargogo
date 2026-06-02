"""
handlers/maintenance.py - Node and global maintenance mode handlers.
"""

import logging

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.config import TG_NODES, TG_MAX_MESSAGE_LEN
from kgg_telegram.db import get_setting, set_setting
from kgg_telegram.helpers import run_kgg_cmd
import html

logger = logging.getLogger(__name__)


async def handle_maint_menu(ctx, message: Message):
    global_maint = get_setting("maintenance_mode", "0") == "1"
    global_icon = "🛠 ACTIVE" if global_maint else "✅ inactive"
    toggle_text = "🔴 Disable Global Maint" if global_maint else "🟢 Enable Global Maint"
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text=toggle_text, callback_data="maint_global_toggle")],
        [InlineKeyboardButton(text="🔧 Node Maintenance Toggles", callback_data="maint_nodes")],
        [InlineKeyboardButton(text="🧹 Deep Cleanup", callback_data="maint_clear_logs")],
        [InlineKeyboardButton(text="📄 Fetch RPi Logs", callback_data="maint_rpi_logs")],
        [InlineKeyboardButton(text="🏥 Generate Diagnostic Report", callback_data="maint_doctor")]
    ])
    await message.answer(
        f"🛠 <b>Maintenance Menu</b>\nGlobal Mode: <code>{global_icon}</code>",
        reply_markup=kb
    )


async def cb_maint(ctx, callback: CallbackQuery):
    action = callback.data.replace("maint_", "")

    if action == "main":
        await callback.answer()
        await handle_maint_menu(ctx, callback.message)
        return

    if action == "global_toggle":
        current = get_setting("maintenance_mode", "0")
        new_val = "1" if current == "0" else "0"
        set_setting("maintenance_mode", new_val)
        status = "ACTIVE 🛠" if new_val == "1" else "inactive ✅"
        await callback.answer(f"Global maintenance mode: {status}", show_alert=True)
        await handle_maint_menu(ctx, callback.message)
        return

    if action == "nodes":
        await callback.answer()
        await _cb_maint_nodes(ctx, callback)
        return

    if action.startswith("toggle_"):
        node_name = action.replace("toggle_", "")
        current = get_setting(f"maintenance_{node_name}", "0")
        new_val = "1" if current == "0" else "0"
        set_setting(f"maintenance_{node_name}", new_val)
        await callback.answer(f"Node {node_name} maintenance flipped to {'ON' if new_val == '1' else 'OFF'}")
        await _cb_maint_nodes(ctx, callback)
        return

    # Maintenance commands
    cmd_map = {
        "clear_logs": (["ops", "update"], "🧹 Deep Cleanup (Ops Update)..."),
        "rpi_logs":   (["ssh", "localhost", "sudo journalctl -u kgg-telegram -n 50 --no-pager"], "📄 Fetching RPi Logs..."),
        "doctor":     (["doctor"], "🏥 Running Diagnostic Report..."),
    }

    if action not in cmd_map:
        await callback.answer("⚠️ Unknown maintenance action", show_alert=True)
        return

    cmd, msg = cmd_map[action]
    await callback.answer()
    await callback.message.edit_text(f"⏳ {msg}")
    success, out, err = await run_kgg_cmd(cmd, timeout=60)

    res_text = out if success else err
    if len(res_text) > TG_MAX_MESSAGE_LEN:
        res_text = res_text[-TG_MAX_MESSAGE_LEN:]
    icon = "✅" if success else "❌"

    await callback.message.answer(
        f"{icon} <b>Result:</b>\n\n<code>{html.escape(res_text)}</code>",
        parse_mode="HTML"
    )


async def _cb_maint_nodes(ctx, callback: CallbackQuery):
    buttons = []
    for n in TG_NODES:
        node_name = n['name']
        is_maint = get_setting(f"maintenance_{node_name}", "0") == "1"
        icon = "🛠" if is_maint else "✅"
        buttons.append([InlineKeyboardButton(
            text=f"{icon} {node_name}",
            callback_data=f"maint_toggle_{node_name}"
        )])
    buttons.append([InlineKeyboardButton(text="⬅️ Back", callback_data="maint_main")])
    kb = InlineKeyboardMarkup(inline_keyboard=buttons)
    await callback.message.edit_text(
        "🛠 <b>Per-Node Maintenance</b>\nClick a node to toggle status:",
        reply_markup=kb
    )
    await callback.answer()
