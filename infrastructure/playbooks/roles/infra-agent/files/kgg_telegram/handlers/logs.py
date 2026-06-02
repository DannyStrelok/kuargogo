"""
handlers/logs.py - Node log streaming handlers.
"""

import asyncio
import logging
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd, truncate

logger = logging.getLogger(__name__)


async def handle_logs_menu(ctx, message: Message):
    """Show node selector for log retrieval."""
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text=f"📝 {n['name']}", callback_data=f"logs_{n['name']}")]
        for n in ctx.nodes
    ])
    await message.answer("Select node to check logs:", reply_markup=kb)


async def cb_logs(ctx, query: CallbackQuery):
    node = query.data.split("_")[1]
    await query.answer(f"Fetching logs for {node}...")
    success, out, err = await run_kgg_cmd(["ssh", node, "sudo journalctl -n 20 --no-pager"])
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🔄 Tail Logs Live (30s)", callback_data=f"taillogs:{node}")]
    ])
    await query.message.edit_text(f"📝 <b>Logs for {node}:</b>\n<pre>{html.escape(out)}</pre>", reply_markup=kb)


async def cb_tail_logs(ctx, query: CallbackQuery):
    node = query.data.split(":")[1]
    await query.answer("Starting live tail (30s)...")
    asyncio.create_task(_tail_logs_task(query.message, node))


async def _tail_logs_task(message: Message, node: str):
    last_out = ""
    for i in range(10):  # 10 iterations × 3s = 30s
        success, out, err = await run_kgg_cmd(["ssh", node, "sudo journalctl -n 15 --no-pager"])
        if success and out != last_out:
            try:
                remaining = 30 - (i * 3)
                await message.edit_text(
                    f"🔄 <b>Live Logs for {node} ({remaining}s):</b>\n<pre>{html.escape(truncate(out))}</pre>"
                )
                last_out = out
            except Exception as e:
                if "message is not modified" not in str(e).lower():
                    break
        await asyncio.sleep(3)
    try:
        await message.edit_text(
            f"🛑 <b>Live Tail Finished ({node}):</b>\n<pre>{html.escape(truncate(last_out))}</pre>"
        )
    except Exception as e:
        logger.debug(f"_tail_logs_task final edit failed: {e}")
