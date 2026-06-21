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


def format_heartbeat_report(out: str) -> str:
    """Format the raw heartbeat command output into a clean HTML layout for Telegram."""
    parts = out.split("------------------------")
    if len(parts) < 2:
        return f"🩺 <b>AI Health Report:</b>\n\n<pre>{html.escape(out)}</pre>"
    
    nodes_part = parts[0].replace("--- Heartbeat Report ---", "").strip()
    ai_part = parts[1].strip()
    
    if ai_part.startswith("🤖 AI Insight:"):
        ai_part = ai_part[len("🤖 AI Insight:"):].strip()
        
    repairs_part = ""
    if len(parts) > 2:
        repairs_part = parts[2].strip()
        
    formatted = "🩺 <b>Infrastructure Heartbeat Report</b>\n"
    formatted += "━━━━━━━━━━━━━━━━━━━━━━━━\n"
    formatted += "🖥 <b>Nodes Status:</b>\n"
    formatted += f"<pre>{html.escape(nodes_part)}</pre>\n"
    
    if ai_part:
        formatted += "━━━━━━━━━━━━━━━━━━━━━━━━\n"
        formatted += "🤖 <b>AI Analysis & Insight:</b>\n\n"
        formatted += f"{html.escape(ai_part)}\n"
        
    if repairs_part:
        formatted += "\n━━━━━━━━━━━━━━━━━━━━━━━━\n"
        formatted += f"✨ <b>Suggested Repairs:</b>\n"
        formatted += f"<code>{html.escape(repairs_part)}</code>\n"
        
    return formatted


async def handle_health_menu(ctx, message: Message, edit: bool = False):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🤖 Global AI Heartbeat", callback_data="hlth_global")],
        [InlineKeyboardButton(text="🔍 Node Hardware Health", callback_data="hlth_node_selector")]
    ])
    text = "🩺 <b>Health & Diagnostics</b>\nSelect the type of diagnostic to run:"
    if edit:
        await message.edit_text(text, reply_markup=kb)
    else:
        await message.answer(text, reply_markup=kb)


async def cb_health(ctx, query: CallbackQuery):
    await query.answer()
    data = query.data.replace("hlth_", "")

    if data == "global":
        await run_health_ai(ctx, query.message)
    elif data == "node_selector":
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text=f"🩺 {n['name']}", callback_data=f"hlth_node_run_{n['name']}")]
            for n in ctx.nodes
        ] + [[InlineKeyboardButton(text="⬅️ Back", callback_data="hlth_main")]])
        await query.message.edit_text("🔍 <b>Select a node to run detailed hardware checks:</b>", reply_markup=kb)
    elif data.startswith("node_run_"):
        target = data.replace("node_run_", "")
        await run_health_node(ctx, query.message, target)
    elif data == "main":
        await handle_health_menu(ctx, query.message, edit=True)


async def run_health_ai(ctx, message: Message):
    await message.edit_text(
        "🧠 <b>Running Global AI Infrastructure Heartbeat...</b>\n<i>(This takes about 20-30s)</i>",
        parse_mode="HTML"
    )
    success, out, err = await run_kgg_cmd(["infra", "heartbeat", "--ai"], timeout=60)
    
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="⬅️ Back to Health Menu", callback_data="hlth_main")]
    ])
    
    if success:
        formatted = format_heartbeat_report(out)
        await message.edit_text(formatted, reply_markup=kb)
    else:
        await message.edit_text(
            f"❌ Global Health check failed:\n<pre>{html.escape(err)}</pre>",
            reply_markup=kb
        )


async def run_health_node(ctx, message: Message, target: str):
    await message.edit_text(
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
    await message.edit_text(
        f"{icon} <b>Detailed Health ({target}):</b>\n<pre>{html.escape(res)}</pre>",
        reply_markup=kb
    )
