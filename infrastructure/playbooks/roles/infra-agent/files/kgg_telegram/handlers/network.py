"""
handlers/network.py - Network cross-diagnostics (ping + iperf3).
"""

import logging
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)


async def handle_net_menu(ctx, message: Message):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text=f"📡 Source: {n['name']}", callback_data=f"net:src:{n['name']}")]
        for n in ctx.nodes
    ])
    await message.answer("🌐 <b>Network Cross-Diagnostics</b>\nSelect SOURCE node:", reply_markup=kb)


async def cb_net(ctx, query: CallbackQuery):
    parts = query.data.split(":")
    if len(parts) < 3:
        await query.answer("⚠️ Invalid network callback", show_alert=True)
        return

    action = parts[1]

    if action == "src":
        await query.answer()
        src = parts[2]
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(
                text=f"🎯 Target: {n['name']}",
                callback_data=f"net:tgt:{src}:{n['name']}"
            )]
            for n in ctx.nodes if n['name'] != src
        ])
        await query.message.edit_text(
            f"🌐 <b>Source: {src}</b>\nSelect TARGET node:",
            reply_markup=kb
        )
    elif action == "tgt":
        if len(parts) < 4:
            await query.answer("⚠️ Malformed network callback", show_alert=True)
            return
        src, tgt = parts[2], parts[3]
        await query.answer("Running network test (10s)...")
        await query.message.edit_text(
            f"⏳ Testing <code>{src}</code> ➟ <code>{tgt}</code> (iperf3 & ping)..."
        )

        # 1. Resolve node IPs to bypass any DNS/hostname/IPv6 resolution issues
        src_node = next((n for n in ctx.nodes if n['name'] == src), None)
        tgt_node = next((n for n in ctx.nodes if n['name'] == tgt), None)
        src_ip = src_node['ip'] if src_node else src
        tgt_ip = tgt_node['ip'] if tgt_node else tgt

        # 2. Pre-emptively ensure iperf3 server daemon is running on target node
        await run_kgg_cmd(["ssh", tgt, "iperf3 -s -D || true"])

        # 3. Run the diagnostics using target IP and force IPv4 (-4) since IPv6 is disabled in sysctl
        success, out, err = await run_kgg_cmd(
            ["ssh", src, f"ping -c 4 {tgt_ip}; iperf3 -4 -c {tgt_ip} -t 5 2>&1 || true"],
            timeout=65
        )
        result = (
            f"🌐 <b>Network Report ({src} ➟ {tgt}):</b>\n<pre>{html.escape(out or err)}</pre>\n\n"
            "<i>Note: iperf3 requires iperf3 to be running as server on target.</i>"
        )
        kb = InlineKeyboardMarkup(inline_keyboard=[[
            InlineKeyboardButton(text="⬅️ Back to Network", callback_data="net:main")
        ]])
        await query.message.edit_text(result, reply_markup=kb)
    elif action == "main":
        await query.answer()
        await handle_net_menu(ctx, query.message)
    else:
        await query.answer("⚠️ Unknown network action", show_alert=True)
