"""
handlers/ssh.py - SSH Console FSM handlers.
Provides One-Shot and persistent Keep-Open terminal modes.
"""

import logging
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton
from aiogram.fsm.context import FSMContext

from kgg_telegram.helpers import run_kgg_cmd, truncate
from kgg_telegram.security import SHELL_INJECTION_RE
from kgg_telegram.states import BotStates

logger = logging.getLogger(__name__)


async def handle_ssh_menu(ctx, message: Message, state: FSMContext):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text=f"🖥 {n['name']}", callback_data=f"ssh_node_{n['name']}")]
        for n in ctx.nodes
    ])
    await message.answer(
        "⚡ <b>Secure SSH Console</b>\n"
        "Select a node, then choose <b>One-Shot</b> or <b>Keep Open</b> mode:",
        reply_markup=kb
    )


async def process_ssh_node(ctx, callback: CallbackQuery, state: FSMContext):
    node_name = callback.data.split("ssh_node_")[1]
    await state.update_data(ssh_node=node_name, ssh_persistent=False)
    kb = InlineKeyboardMarkup(inline_keyboard=[[
        InlineKeyboardButton(text="⚡ One-Shot", callback_data=f"ssh_mode_shot_{node_name}"),
        InlineKeyboardButton(text="🖥 Keep Open", callback_data=f"ssh_mode_open_{node_name}")
    ]])
    await callback.message.edit_text(
        f"🖥 <b>Node: {node_name}</b>\n\n"
        "⚡ <b>One-Shot</b>: Run one command and close automatically.\n"
        "🖥 <b>Keep Open</b>: Persistent session — type <code>exit</code> to close.",
        reply_markup=kb
    )
    await callback.answer()


async def process_ssh_mode(ctx, callback: CallbackQuery, state: FSMContext):
    mode = callback.data.replace("ssh_mode_", "").split("_")[0]
    await state.update_data(ssh_mode=mode, ssh_cwd="~")

    data = await state.get_data()
    node = data.get("ssh_node")
    mode_text = "🖥 Keep Open" if mode == "open" else "⚡ One-Shot"
    await callback.message.edit_text(
        f"<b>Terminal: {node}</b> ({mode_text})\n"
        f"Current path: <code>~</code>\n\n"
        f"Enter command (type <code>exit</code> to close):"
    )
    await state.set_state(BotStates.waiting_for_command)
    await callback.answer()


async def process_ssh_command(ctx, message: Message, state: FSMContext):
    data = await state.get_data()
    node = data.get("ssh_node")
    mode = data.get("ssh_mode", "shot")
    cwd = data.get("ssh_cwd", "~")
    command = message.text.strip()

    if command.lower() in ["cancel", "exit", "quit"]:
        await state.clear()
        await message.reply("🔌 SSH Session closed.")
        return

    if not node:
        await message.reply("❌ Error: No node selected. Terminating session.")
        await state.clear()
        return

    # Defence-in-depth: block the most dangerous injection vectors.
    # The user is already authenticated as TELEGRAM_ADMIN_ID.
    if SHELL_INJECTION_RE.search(command):
        await message.reply(
            "🚫 <b>Blocked:</b> Command contains disallowed shell metacharacters.\n"
            "<i>Backticks, <code>$(...)</code>, <code>${...}</code>, <code>||</code>, "
            "<code>&&</code>, and <code>;;</code> are not allowed in the SSH console.</i>"
        )
        return

    sent = await message.answer(f"⏳ <code>[{cwd}] $ {command}</code>")

    # Persistent CWD: wrap command to capture resulting PWD
    delim = "---PWD_SYNC---"
    wrapped_cmd = f"cd {cwd} && {command}; echo '{delim}'; pwd"

    success, out, err = await run_kgg_cmd(["ssh", node, wrapped_cmd], timeout=45)

    res_text = out if success else err
    new_cwd = cwd

    if delim in out:
        parts = out.rsplit(delim, 1)
        res_text = parts[0].strip()
        if len(parts) > 1:
            new_cwd = parts[1].strip()
            await state.update_data(ssh_cwd=new_cwd)

    if not res_text or res_text.isspace():
        res_text = "(no output)"
    res_text = truncate(res_text)

    icon = "✅" if success else "❌"
    kb = None
    if mode == "open":
        kb = InlineKeyboardMarkup(inline_keyboard=[[
            InlineKeyboardButton(text="🔌 Terminate SSH Session", callback_data="ssh_close")
        ]])

    await sent.edit_text(
        f"{icon} <b>Result ({node}):</b>\n<pre>{html.escape(res_text)}</pre>\n\n"
        f"📍 <code>{new_cwd}</code>\n"
        f"<i>🖥 Session Active: type next command or use button below</i>",
        reply_markup=kb,
        parse_mode="HTML"
    )

    if mode == "shot":
        await state.clear()


async def handle_ssh_close(ctx, query: CallbackQuery, state: FSMContext):
    await state.clear()
    await query.answer("Session closed")
    await query.message.edit_text(
        "🔌 <b>SSH Session closed.</b>\nTerminal access terminated.",
        parse_mode="HTML"
    )
