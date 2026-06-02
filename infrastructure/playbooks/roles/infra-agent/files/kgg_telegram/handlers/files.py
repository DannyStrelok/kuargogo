"""
handlers/files.py - Secure file browser handlers.
All paths are validated against TG_ALLOWED_PATHS before any SSH call.
"""

import logging
import os
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton
from aiogram.fsm.context import FSMContext

from kgg_telegram.config import TG_ALLOWED_PATHS, TG_ADMIN_ID, TG_MAX_FILE_DOWNLOAD_BYTES
from kgg_telegram.helpers import run_kgg_cmd, run_kgg_cmd_raw
from kgg_telegram.security import is_safe_path
from kgg_telegram.states import BotStates

logger = logging.getLogger(__name__)


async def handle_files_menu(ctx, message: Message):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text=f"🖥 {n['name']}", callback_data=f"files_node_{n['name']}")]
        for n in ctx.nodes
    ])
    await message.answer("📂 <b>Secure File Browser</b>\nSelect node to explore:", reply_markup=kb)


async def process_file_path(ctx, message: Message, state: FSMContext):
    path = message.text.strip()
    data = await state.get_data()
    node = data.get("browser_node")

    if not node:
        await message.reply("❌ Session timed out. Please start from the Files menu again.")
        await state.clear()
        return

    await state.set_state(None)

    # Enforce path allowlist for ALL users — no admin bypass.
    if not is_safe_path(path, TG_ALLOWED_PATHS):
        await message.reply(
            f"🚫 <b>Access Denied:</b> <code>{path}</code> is not within an allowed base path.\n"
            f"Allowed bases: <code>{', '.join(TG_ALLOWED_PATHS)}</code>"
        )
        return

    sent = await message.answer(f"⏳ Listing <code>{path}</code> on {node}...")
    await _browse_path(ctx, sent, node, path, state)


async def cb_files(ctx, query: CallbackQuery, state: FSMContext):
    data = query.data.split("_")
    state_data = await state.get_data()
    node = state_data.get('browser_node')

    if data[1] == "node":
        node = data[2]
        await state.update_data(browser_node=node)
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text=f"📁 {p}", callback_data=f"files_path_{p}")]
            for p in TG_ALLOWED_PATHS
        ] + [[InlineKeyboardButton(text="⌨️ Custom Path", callback_data="files_custom")]])
        await query.message.edit_text(
            f"📂 <b>Node: {node}</b>\nSelect a secured base path:",
            reply_markup=kb
        )
        await query.answer()

    elif data[1] == "custom":
        await query.message.edit_text(
            f"📂 <b>Node: {node}</b>\nEnter the absolute path you want to explore:"
        )
        await state.set_state(BotStates.waiting_for_path)
        await query.answer()

    elif data[1] == "path":
        path = query.data[len("files_path_"):]
        await query.answer()
        await _browse_path(ctx, query.message, node, path, state)

    elif data[1] == "view":
        path = query.data[len("files_view_"):]
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="📄 View Tail (20 lines)", callback_data=f"files_tail_{path}")],
            [InlineKeyboardButton(text="📥 Download File", callback_data=f"files_get_{path}")],
            [InlineKeyboardButton(text="⬅️ Back", callback_data=f"files_path_{os.path.dirname(path)}")],
        ])
        await query.message.edit_text(
            f"📄 <b>File:</b> <code>{os.path.basename(path)}</code>\nLocation: <code>{path}</code>",
            reply_markup=kb
        )
        await query.answer()

    elif data[1] == "tail":
        path = query.data[len("files_tail_"):]
        await query.answer(f"Tailing {os.path.basename(path)}...")
        success, out, err = await run_kgg_cmd(["ssh", node, f"tail -n 20 '{path}'"])
        res = out if success else err
        await state.update_data(last_analysis_text=res)
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text="🤖 Analyze Log", callback_data=f"ai_analyze_file_{node}")],
            [InlineKeyboardButton(text="⬅️ Back", callback_data=f"files_view_{path}")]
        ])
        await query.message.answer(f"📄 <b>Tail: {path}</b>\n<pre>{html.escape(res)}</pre>", reply_markup=kb)

    elif data[1] == "get":
        path = query.data[len("files_get_"):]
        await query.answer("Downloading...")
        success, out, err = await run_kgg_cmd(["ssh", node, f"stat -c %s '{path}'"])
        try:
            size = int(out.split()[0]) if success and out.strip() else 0
        except Exception:
            size = 0

        if size > TG_MAX_FILE_DOWNLOAD_BYTES:
            await query.message.answer(
                f"⚠️ <b>File too large:</b> {size / 1024 / 1024:.1f} MB\n"
                "Cannot download files larger than 10 MB via Telegram."
            )
            return

        success, file_bytes, err_bytes = await run_kgg_cmd_raw(["ssh", node, f"cat '{path}'"])
        if not success:
            err_text = err_bytes.decode('utf-8', errors='replace')
            await query.message.answer(
                f"❌ <b>Download Failed:</b>\n"
                f"<pre>{html.escape(err_text or 'Unknown error reading file')}</pre>"
            )
            return

        from aiogram.types import BufferedInputFile
        file = BufferedInputFile(file_bytes, filename=os.path.basename(path))
        await ctx.bot.send_document(
            TG_ADMIN_ID, file,
            caption=f"📥 File from {node}: <code>{html.escape(path)}</code>"
        )


async def _browse_path(ctx, message: Message, node: str, path: str, state: FSMContext):
    success, out, err = await run_kgg_cmd(["ssh", node, f"ls -p {path}"])
    if not success:
        await message.edit_text(
            f"❌ <b>Error listing path:</b>\n<code>{html.escape(path)}</code>\n<code>{html.escape(err or out)}</code>"
        )
        return

    items = out.splitlines()
    buttons = []
    for item in items:
        full_path = os.path.join(path, item).replace("\\", "/")
        icon = "📁" if item.endswith("/") else "📄"
        cb_data = f"files_path_{full_path}" if item.endswith("/") else f"files_view_{full_path}"
        buttons.append([InlineKeyboardButton(text=f"{icon} {item}", callback_data=cb_data)])

    buttons.append([InlineKeyboardButton(text="⬅️ Back to base", callback_data=f"files_node_{node}")])
    kb = InlineKeyboardMarkup(inline_keyboard=buttons)
    await message.edit_text(
        f"📂 <b>Exploring:</b> <code>{path}</code>\nNode: <b>{node}</b>",
        reply_markup=kb
    )
