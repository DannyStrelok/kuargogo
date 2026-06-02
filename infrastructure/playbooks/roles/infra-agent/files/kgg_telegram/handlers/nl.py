"""
handlers/nl.py - Natural language fallback and confirmation handlers.
"""

import datetime
import json
import logging
import re
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.config import TG_ADMIN_ID
from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)


async def interpret_intent(text: str):
    """Call Go backend to interpret a natural language intent."""
    success, stdout, stderr = await run_kgg_cmd(["ai", "interpret", text])
    if success:
        try:
            return json.loads(stdout)
        except json.JSONDecodeError:
            return None
    return None


async def handle_nl(ctx, message: Message):
    """Catch-all: try keyword matching first, then AI intent parsing."""
    if not message.text:
        return
    text = message.text.lower().strip()

    if text in ["status", "nodos", "nodes", "📊 status"]:
        from kgg_telegram.handlers.status import handle_status
        await handle_status(ctx, message)
        return
    if text in ["health", "salud", "diagnostic", "🩺 health"]:
        from kgg_telegram.handlers.health import handle_health_menu
        await handle_health_menu(ctx, message)
        return

    reboot_match = re.search(r"(reboot|reiniciar)\s+([\w-]+)", text)
    shutdown_match = re.search(r"(shutdown|apagar)\s+([\w-]+)", text)

    if reboot_match:
        await ask_confirmation(ctx, message, "reboot", reboot_match.group(2))
        return
    if shutdown_match:
        await ask_confirmation(ctx, message, "off", shutdown_match.group(2))
        return

    intent = await interpret_intent(message.text)
    if not intent or intent.get("action") == "unknown":
        await message.reply(
            "🤔 I'm not sure what you mean. Try the buttons or be more specific "
            "(e.g., 'reboot node-1')."
        )
        return

    action = intent["action"]
    target = intent["target"]

    if action == "health":
        from kgg_telegram.handlers.health import handle_health_menu
        await handle_health_menu(ctx, message)
    elif action == "status":
        from kgg_telegram.handlers.status import handle_status
        await handle_status(ctx, message)
    elif action in ["reboot", "off"]:
        await ask_confirmation(ctx, message, action, target)
    else:
        await message.reply(
            f"I understood: <b>{action}</b> on <b>{target}</b>, "
            "but I haven't implemented that NL handler yet."
        )


async def ask_confirmation(ctx, message: Message, action: str, target: str):
    expires = datetime.datetime.now().timestamp() + 60
    ctx.pending_actions[TG_ADMIN_ID] = {"action": action, "target": target, "expires": expires}

    kb = InlineKeyboardMarkup(inline_keyboard=[[
        InlineKeyboardButton(text="✅ YES, Execute", callback_data=f"confirm_{action}_{target}"),
        InlineKeyboardButton(text="❌ No, Cancel", callback_data="cancel_action")
    ]])
    await message.answer(
        f"⚠️ <b>CONFIRMATION REQUIRED</b>\n\n"
        f"Are you sure you want to <b>{action.upper()}</b> <code>{target}</code>?\n\n"
        f"<i>This request expires in 60 seconds.</i>",
        reply_markup=kb
    )


async def handle_cancel_action(ctx, query: CallbackQuery):
    ctx.pending_actions.pop(TG_ADMIN_ID, None)
    await query.answer("Cancelled")
    await query.message.edit_text("❌ Operation cancelled by user.")


async def cb_ai_analyze(ctx, query: CallbackQuery, state):
    data = await state.get_data()
    text = data.get("last_analysis_text")

    if not text:
        await query.answer("⚠️ Analysis context lost. Re-run the command.", show_alert=True)
        return

    await query.answer("Analyzing with AI...")
    await query.message.edit_text(f"{query.message.text}\n\n🤖 <b>AI is analyzing...</b>")
    success, out, err = await run_kgg_cmd(["ai", "explain", text], timeout=60)

    if success:
        await query.message.answer(f"🤖 <b>AI Analysis Report:</b>\n\n{out}")
    else:
        await query.message.answer(f"❌ AI analysis failed:\n<pre>{html.escape(err)}</pre>")
