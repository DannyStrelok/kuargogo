"""
handlers/status.py - Cluster status overview handler.
"""

import json
import logging
import html

from aiogram.types import Message, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.db import get_setting
from kgg_telegram.helpers import run_kgg_cmd, render_bar, parse_usage

logger = logging.getLogger(__name__)


async def handle_status(ctx, message: Message):
    """Show cluster status by calling 'kgg node status --json'."""
    sent = await message.answer("⏳ <b>Scanning cluster node status...</b>")
    success, out, err = await run_kgg_cmd(["node", "status", "--json"], timeout=90)
    if not success:
        await sent.edit_text(f"❌ <b>Error fetching status:</b>\n<pre>{html.escape(err)}</pre>")
        return

    try:
        data = json.loads(out)
        msg = "📊 <b>Cluster Status Report</b>\n\n"
        buttons = []

        for node in data:
            status = node.get("status", "unknown")
            maint = get_setting(f"maintenance_{node['name']}", "0") == "1"
            maint_tag = " 🛠" if maint else ""

            needs_diagnostic = False
            if status != "online":
                status_icon = "🔴"
                needs_diagnostic = True
            else:
                cpu_p = parse_usage(node.get('cpu', '0'))
                ram_p = parse_usage(node.get('ram', '0'))
                disk_p = parse_usage(node.get('disk', '0'))
                max_p = max(cpu_p, ram_p, disk_p)
                if max_p > 90:
                    status_icon = "🔴"
                    needs_diagnostic = True
                elif max_p > 75:
                    status_icon = "🟠"
                    needs_diagnostic = True
                else:
                    status_icon = "🟢"

            msg += f"{status_icon} <b>{node['name']}</b> ({node['role']}){maint_tag}\n"
            if status == "online":
                msg += f"   CPU  {render_bar(node.get('cpu', '0%'))}\n"
                msg += f"   RAM  {render_bar(node.get('ram', '0%'))}\n"
                msg += f"   Disk {render_bar(node.get('disk', '0%'))}\n"
                if node.get('uptime'):
                    msg += f"   ⏱ <i>{node['uptime']}</i>\n"
            else:
                msg += "   <i>Offline — no metrics available</i>\n"
            msg += "------------------\n"

            if needs_diagnostic:
                buttons.append([InlineKeyboardButton(
                    text=f"🩺 Diagnose {node['name']}",
                    callback_data=f"hlth_node_run_{node['name']}"
                )])

        kb = InlineKeyboardMarkup(inline_keyboard=buttons) if buttons else None
        await sent.edit_text(msg, reply_markup=kb)
    except Exception as e:
        await sent.edit_text(f"❌ <b>Parse error:</b>\n<pre>{html.escape(str(e))}</pre>")
