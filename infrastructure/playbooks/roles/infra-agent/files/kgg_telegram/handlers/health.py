"""
handlers/health.py - Health diagnostics and AI heartbeat handlers.
"""

import logging
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton
from aiogram.fsm.storage.base import StorageKey

from kgg_telegram.config import TG_ADMIN_ID, TG_MAX_MESSAGE_LEN
from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)


async def handle_health_menu(ctx, message: Message):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🤖 Global AI Heartbeat", callback_data="hlth_global")],
        [InlineKeyboardButton(text="🔍 Node Hardware Health", callback_data="hlth_node_selector")]
    ])
    await message.answer("🩺 <b>Health & Diagnostics</b>\nSelect the type of diagnostic to run:", reply_markup=kb)


async def cb_health(ctx, query: CallbackQuery):
    await query.answer()
    data = query.data.replace("hlth_", "")

    if data == "global":
        await run_health_ai(ctx, query.message)
    elif data == "node_selector":
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text=f"🩺 {n['name']}", callback_data=f"hlth_node_run_{n['name']}")]
            for n in ctx.nodes
        ])
        await query.message.edit_text("🔍 <b>Select a node to run detailed hardware checks:</b>", reply_markup=kb)
    elif data.startswith("node_run_"):
        target = data.replace("node_run_", "")
        await run_health_node(ctx, query.message, target)
    elif data == "main":
        await handle_health_menu(ctx, query.message)


async def run_health_ai(ctx, message: Message):
    status_msg = await message.answer(
        "🧠 <b>Running Global AI Infrastructure Heartbeat...</b>\n<i>(This takes about 20-30s)</i>",
        parse_mode="HTML"
    )
    success, out, err = await run_kgg_cmd(["infra", "heartbeat", "--ai"], timeout=60)
    if success:
        await status_msg.edit_text(f"🩺 <b>AI Health Report:</b>\n\n{out}")
    else:
        await status_msg.edit_text(f"❌ Global Health check failed:\n<pre>{html.escape(err)}</pre>")


async def run_health_node(ctx, message: Message, target: str):
    status_msg = await message.answer(
        f"🔍 <b>Running hardware health checks on <code>{target}</code>...</b>\n"
        "<i>(Checking SMART, Memory, CPU temps)</i>"
    )
    success, out, err = await run_kgg_cmd(["node", "health", target], timeout=45)

    res = out if success else err
    if not res or res.isspace():
        res = "No output returned or node unreachable."

    # Save for possible AI analysis
    key = StorageKey(bot_id=ctx.bot.id, chat_id=TG_ADMIN_ID, user_id=TG_ADMIN_ID)
    await ctx.dp.storage.update_data(key=key, data={"last_analysis_text": res})

    if len(res) > TG_MAX_MESSAGE_LEN:
        res = res[:TG_MAX_MESSAGE_LEN] + "...(truncated)"

    icon = "✅" if success else "❌"
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🤖 Explain with AI", callback_data="ai_analyze_health")],
        [InlineKeyboardButton(text="⬅️ Back to Health Menu", callback_data="hlth_main")]
    ])
    await status_msg.edit_text(
        f"{icon} <b>Detailed Health ({target}):</b>\n<pre>{html.escape(res)}</pre>",
        reply_markup=kb
    )
