"""
handlers/k3s.py - K3s infrastructure operation handlers.
Restart and scale deployments via Telegram FSM.
"""

import logging
import html

from aiogram import types
from aiogram.types import Message, CallbackQuery
from aiogram.fsm.context import FSMContext

from kgg_telegram.helpers import run_kgg_cmd, truncate
from kgg_telegram.security import sanitize_shell_arg
from kgg_telegram.states import BotStates

logger = logging.getLogger(__name__)

_MASTER_ROLES = {'server', 'master', 'control-plane'}


def _get_master(nodes: list) -> str:
    return next((n['name'] for n in nodes if n.get('role') in _MASTER_ROLES), "master-1")


async def handle_k3s_menu(ctx, message: Message):
    from aiogram.types import InlineKeyboardMarkup, InlineKeyboardButton
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🔄 Cluster Nodes Status", callback_data="k3s_nodes_status")],
        [InlineKeyboardButton(text="⚠️ Problematic Pods", callback_data="k3s_pods_fail")],
        [InlineKeyboardButton(text="🔄 Restart Deployment", callback_data="k3s_restart_menu")],
        [InlineKeyboardButton(text="⚖️ Scale Deployment", callback_data="k3s_scale_menu")]
    ])
    await message.answer("🐳 <b>K3s Infrastructure Operations</b>\nSelect action:", reply_markup=kb)


async def cb_k3s(ctx, callback: CallbackQuery, state: FSMContext):
    await callback.answer()
    data = callback.data.replace("k3s_", "")

    if data == "nodes_status":
        await _run_k3s_nodes(ctx, callback.message)
    elif data == "pods_fail":
        await _run_k3s_pods_fail(ctx, callback.message)
    elif data == "restart_menu":
        await callback.message.edit_text("🔄 Enter the name of the K3s <b>deployment</b> to restart:")
        await state.set_state(BotStates.waiting_for_k3s_name)
        await state.update_data(k3s_action="restart")
    elif data == "scale_menu":
        await callback.message.edit_text("⚖️ Enter the name of the K3s <b>deployment</b> to scale:")
        await state.set_state(BotStates.waiting_for_k3s_name)
        await state.update_data(k3s_action="scale")
    elif data.startswith("remediate:"):
        node_name = data.replace("remediate:", "")
        await run_manual_remediation(ctx, callback.message, node_name)
    elif data.startswith("evict_disk:"):
        parts = data.replace("evict_disk:", "").split(":")
        if len(parts) == 2:
            node_name, disk_id = parts[0], parts[1]
            await run_manual_eviction(ctx, callback.message, node_name, disk_id)

async def run_manual_remediation(ctx, message: Message, node_name: str):
    try:
        node_name = sanitize_shell_arg(node_name, label="node name")
    except ValueError as e:
        await message.answer(f"🚫 <b>Invalid node name:</b> {e}")
        return

    sent = await message.answer(f"🛠️ <b>Starting Manual K3s Node Remediation for {html.escape(node_name)}...</b>\nThis may take a few minutes...")

    success, out, err = await run_kgg_cmd(["cluster", "remediate", "--name", node_name], timeout=300)

    try:
        if success:
            formatted = f"✅ <b>Manual K3s Node Remediation Success!</b>\nNode: <code>{html.escape(node_name)}</code>\n\n<pre>{html.escape(truncate(out))}</pre>"
            await sent.edit_text(formatted)
        else:
            formatted = f"❌ <b>Manual K3s Node Remediation Failed!</b>\nNode: <code>{html.escape(node_name)}</code>\n\nError: <pre>{html.escape(truncate(err or out))}</pre>"
            await sent.edit_text(formatted)
    except Exception as e:
        logger.error(f"Failed to edit message: {e}")
        # Fallback to sending a new message
        if success:
            await message.answer(f"✅ Manual K3s Node Remediation Success for {html.escape(node_name)}!")
        else:
            await message.answer(f"❌ Manual K3s Node Remediation Failed for {html.escape(node_name)}!")


async def run_manual_eviction(ctx, message: Message, node_name: str, disk_id: str):
    try:
        node_name = sanitize_shell_arg(node_name, label="node name")
        disk_id = sanitize_shell_arg(disk_id, label="disk ID")
    except ValueError as e:
        await message.answer(f"🚫 <b>Invalid argument:</b> {e}")
        return

    sent = await message.answer(f"🛠️ <b>Starting Manual Disk Eviction for {html.escape(node_name)} (Disk: {html.escape(disk_id)})...</b>")

    master = _get_master(ctx.nodes)
    patch_cmd = f"sudo k3s kubectl patch nodes.longhorn.io {node_name} -n longhorn-system --type merge -p '{{\"spec\": {{\"disks\": {{\"{disk_id}\": {{\"allowScheduling\": false, \"evictionRequested\": true}}}}}}}}'"

    success, out, err = await run_kgg_cmd(["ssh", master, patch_cmd], timeout=60)

    try:
        if success:
            await sent.edit_text(f"✅ <b>Manual Disk Eviction Initiated!</b>\nNode: <code>{html.escape(node_name)}</code>\nDisk: <code>{html.escape(disk_id)}</code>\n\nReplicas are being evacuated off this disk.")
        else:
            await sent.edit_text(f"❌ <b>Manual Disk Eviction Failed!</b>\nNode: <code>{html.escape(node_name)}</code>\nDisk: <code>{html.escape(disk_id)}</code>\n\nError: <pre>{html.escape(err or out)}</pre>")
    except Exception as e:
        logger.error(f"Failed to edit message: {e}")
        if success:
            await message.answer(f"✅ Manual Disk Eviction Initiated for {html.escape(node_name)} (Disk: {html.escape(disk_id)})!")
        else:
            await message.answer(f"❌ Manual Disk Eviction Failed for {html.escape(node_name)} (Disk: {html.escape(disk_id)})!")


async def _run_k3s_nodes(ctx, message: Message):
    sent = await message.answer("⏳ <b>Fetching K3s nodes status...</b>")
    master = _get_master(ctx.nodes)
    success, out, err = await run_kgg_cmd(["ssh", master, "sudo k3s kubectl get nodes -o wide"])
    if success:
        await sent.edit_text(f"🐳 <b>K3s Cluster Nodes:</b>\n<pre>{html.escape(out)}</pre>")
    else:
        await sent.edit_text(f"❌ Failed to fetch nodes:\n<pre>{html.escape(err)}</pre>")


async def _run_k3s_pods_fail(ctx, message: Message):
    sent = await message.answer("⏳ <b>Scanning for problematic pods...</b>")
    master = _get_master(ctx.nodes)
    cmd = "sudo k3s kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded"
    success, out, err = await run_kgg_cmd(["ssh", master, cmd])
    if success:
        if not out or out.strip() == "" or "No resources found" in out:
            await sent.edit_text("✅ <b>All pods are Running or Succeeded.</b>")
        else:
            await sent.edit_text(f"⚠️ <b>Problematic Pods:</b>\n<pre>{html.escape(out)}</pre>")
    else:
        await sent.edit_text(f"❌ Failed to scan pods:\n<pre>{html.escape(err)}</pre>")


async def process_k3s_name(ctx, message: types.Message, state: FSMContext):
    raw = message.text.strip()
    data = await state.get_data()
    action = data.get("k3s_action")

    if raw.lower() == "cancel":
        await state.clear()
        await message.reply("❌ K3s operation cancelled.")
        return

    try:
        deploy_name = sanitize_shell_arg(raw, label="deployment name")
    except ValueError as e:
        await message.reply(f"🚫 <b>Invalid deployment name:</b> {e}")
        return

    await state.update_data(k3s_deploy=deploy_name)

    if action == "restart":
        await message.answer(f"🔄 Restarting deployment <code>{deploy_name}</code>...")
        master = _get_master(ctx.nodes)
        success, out, err = await run_kgg_cmd(
            ["ssh", master, f"sudo k3s kubectl rollout restart deployment {deploy_name}"]
        )
        if success:
            await message.answer(f"✅ Restarted {deploy_name}:\n<pre>{html.escape(out)}</pre>")
        else:
            await message.answer(f"❌ Failed to restart {deploy_name}:\n<pre>{html.escape(err)}</pre>")
        await state.clear()
    elif action == "scale":
        await message.answer(f"🔢 How many replicas for <code>{deploy_name}</code>?")
        await state.set_state(BotStates.waiting_for_k3s_replicas)


async def process_k3s_replicas(ctx, message: types.Message, state: FSMContext):
    replicas = message.text.strip()
    if replicas.lower() == "cancel":
        await state.clear()
        await message.reply("❌ K3s scale cancelled.")
        return

    if not replicas.isdigit():
        await message.reply("⚠️ Please enter a valid number (e.g., 3).")
        return

    data = await state.get_data()
    deploy_name = data.get("k3s_deploy")
    await state.clear()

    await message.answer(f"⚖️ Scaling <code>{deploy_name}</code> to <b>{replicas}</b> replicas...")
    master = _get_master(ctx.nodes)
    success, out, err = await run_kgg_cmd(
        ["ssh", master, f"sudo k3s kubectl scale deployment {deploy_name} --replicas={replicas}"]
    )
    if success:
        await message.answer(f"✅ Scaled {deploy_name} to {replicas}:\n<pre>{html.escape(out)}</pre>")
    else:
        await message.answer(f"❌ Failed to scale {deploy_name}:\n<pre>{html.escape(err)}</pre>")
