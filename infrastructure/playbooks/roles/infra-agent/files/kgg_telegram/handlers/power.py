"""
handlers/power.py - Power management handlers (WoL, Reboot, Shutdown).
"""

import asyncio
import logging
import html

from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)


async def handle_power_menu(ctx, message: Message):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [
            InlineKeyboardButton(text="⚡ Turn ON (WoL)", callback_data="pwr_on_menu"),
            InlineKeyboardButton(text="🔄 Reboot", callback_data="pwr_reboot_menu"),
            InlineKeyboardButton(text="🔌 Shutdown", callback_data="pwr_off_menu"),
        ],
        [
            InlineKeyboardButton(text="⚡ Power All ON", callback_data="pwr_bulk_on"),
            InlineKeyboardButton(text="🔌 Shutdown All", callback_data="pwr_bulk_off"),
        ]
    ])
    await message.answer("⚡ <b>Power Management</b>\nSelect an action:", reply_markup=kb)


async def cb_power(ctx, callback: CallbackQuery):
    data = callback.data.replace("pwr_", "")

    if data == "main":
        await handle_power_menu(ctx, callback.message)
        return

    # Bulk ON
    if data == "bulk_on":
        targets = [n for n in ctx.nodes if n.get('mac') and n.get('role') != 'infra-manager']
        if not targets:
            await callback.message.edit_text("❌ No nodes eligible for bulk WoL (need MAC address).")
            return
        names = ", ".join(n['name'] for n in targets)
        kb = InlineKeyboardMarkup(inline_keyboard=[[
            InlineKeyboardButton(text=f"⚡ YES, Power ON all ({len(targets)} nodes)", callback_data="confirm_bulk_on_ALL"),
            InlineKeyboardButton(text="❌ Cancel", callback_data="pwr_main")
        ]])
        await callback.message.edit_text(
            f"⚡ <b>Power ALL ON via WoL?</b>\nTargets: <code>{names}</code>\n<i>(infra-manager excluded)</i>",
            reply_markup=kb
        )
        await callback.answer()
        return

    # Bulk OFF
    if data == "bulk_off":
        targets = [n for n in ctx.nodes if n.get('role') != 'infra-manager']
        if not targets:
            await callback.message.edit_text("❌ No nodes eligible for bulk shutdown.")
            return
        names = ", ".join(n['name'] for n in targets)
        kb = InlineKeyboardMarkup(inline_keyboard=[[
            InlineKeyboardButton(text=f"⚠️ YES, Shutdown all ({len(targets)} nodes)", callback_data="confirm_bulk_off_ALL"),
            InlineKeyboardButton(text="❌ Cancel", callback_data="pwr_main")
        ]])
        await callback.message.edit_text(
            f"🔌 <b>SHUTDOWN ALL?</b>\nTargets: <code>{names}</code>\n\n"
            "<b>⚠️ WARNING:</b> <i>infra-manager excluded to preserve remote access.</i>",
            reply_markup=kb
        )
        await callback.answer()
        return

    action, target = data.split("_", 1)

    if target == "menu":
        nodes_to_show = ctx.nodes
        if action == "on":
            nodes_to_show = [n for n in ctx.nodes if n.get('mac')]
        if not nodes_to_show:
            await callback.message.edit_text(f"❌ <b>Error:</b> No nodes found suitable for <b>{action.upper()}</b>")
            return
        kb = InlineKeyboardMarkup(inline_keyboard=[
            [InlineKeyboardButton(text=n['name'], callback_data=f"pwr_{action}_{n['name']}")]
            for n in nodes_to_show
        ])
        await callback.message.edit_text(f"Select node to <b>{action.upper()}</b>:", reply_markup=kb)
        return

    # Non-destructive ON: skip confirmation
    if action == "on":
        await callback.message.edit_text(f"🚀 Executing <b>{action}</b> on <b>{target}</b>...", parse_mode="HTML")
        success, out, err = await run_kgg_cmd(["pwr", "on", target])
        if success:
            await callback.message.edit_text(f"✅ <b>Success:</b>\n{out}")
        else:
            await callback.message.edit_text(f"❌ <b>Failed:</b>\n<pre>{html.escape(err)}</pre>")
        return

    # Destructive actions: require confirmation
    kb = InlineKeyboardMarkup(inline_keyboard=[[
        InlineKeyboardButton(text="✅ Yes, Proceed", callback_data=f"confirm_{action}_{target}"),
        InlineKeyboardButton(text="❌ Cancel", callback_data="pwr_main")
    ]])
    icon = "🔄" if action == "reboot" else "🛑"
    await callback.message.edit_text(
        f"{icon} Are you sure you want to <b>{action}</b> node <b>{target}</b>?",
        reply_markup=kb, parse_mode="HTML"
    )
    await callback.answer()


async def cb_confirm(ctx, callback: CallbackQuery):
    await callback.answer()
    data = callback.data.replace("confirm_", "")
    action, target = data.split("_", 1)

    # Bulk execution
    if action == "bulk":
        bulk_action, _ = target.split("_", 1)
        if bulk_action == "on":
            targets = [n for n in ctx.nodes if n.get('mac') and n.get('role') != 'infra-manager']
            op = "on"
        else:
            targets = [n for n in ctx.nodes if n.get('role') != 'infra-manager']
            op = "off"

        await callback.message.edit_text(f"🚀 Executing bulk <b>{op.upper()}</b> on {len(targets)} nodes...")
        
        # Run all commands concurrently to avoid sequential blocking and cumulative timeouts
        tasks = [run_kgg_cmd(["pwr", op, n["name"]], timeout=30) for n in targets]
        res_tuples = await asyncio.gather(*tasks)
        
        results = []
        for n, (ok, out, err) in zip(targets, res_tuples):
            results.append(f"{'✅' if ok else '❌'} {n['name']}: {out or err}")
            
        result_text = "\n".join(results)
        await callback.message.edit_text(f"🔌 <b>Bulk {op.upper()} Results:</b>\n<pre>{html.escape(result_text)}</pre>")
        return

    # Single node
    await callback.message.edit_text(f"🚀 Executing <b>{action}</b> on <b>{target}</b>...", parse_mode="HTML")
    cmd_map = {"reboot": ["pwr", "reboot", target], "off": ["pwr", "off", target], "on": ["pwr", "on", target]}
    cmd_args = cmd_map.get(action, [])
    if not cmd_args:
        await callback.message.edit_text(f"❌ Unknown action: {action}")
        return

    success, out, err = await run_kgg_cmd(cmd_args)
    if success:
        await callback.message.edit_text(f"✅ <b>Success:</b>\n{out}")
    else:
        await callback.message.edit_text(f"❌ <b>Failed:</b>\n<pre>{html.escape(err)}</pre>")
    ctx.pending_actions.pop(next(iter(ctx.pending_actions), None), None)
