"""
handlers/storage.py - Distributed storage and backup handlers.
"""

import logging
import html
import time
from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)

_MASTER_ROLES = {'server', 'master', 'control-plane'}


def _get_master(nodes: list) -> str:
    return next((n['name'] for n in nodes if n.get('role') in _MASTER_ROLES), "master-1")


async def handle_storage_menu(ctx, message: Message, edit: bool = False):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="💾 Longhorn Status", callback_data="storage_status")],
        [InlineKeyboardButton(text="🛡️ Velero Manual Backup", callback_data="storage_velero_trigger")],
        [InlineKeyboardButton(text="💾 CNPG Database Backup", callback_data="storage_cnpg_menu")],
        [InlineKeyboardButton(text="🔋 Trigger Simulated Backup", callback_data="storage_backup")],
        [InlineKeyboardButton(text="🔙 Back", callback_data="cancel_action")]
    ])
    text = "💾 <b>Storage & Disaster Recovery</b>\nSelect action:"
    if edit:
        await message.edit_text(text, reply_markup=kb)
    else:
        await message.answer(text, reply_markup=kb)


async def cb_storage(ctx, query: CallbackQuery):
    await query.answer()
    action = query.data.split("_", 1)[1]
    
    kb_back = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="🔙 Storage Menu", callback_data="storage_main")]
    ])

    if action == "main":
        await handle_storage_menu(ctx, query.message, edit=True)

    elif action == "status":
        await query.message.edit_text("⏳ Checking Longhorn status (via Ansible)...")
        success, out, err = await run_kgg_cmd(["storage", "status"])
        msg = f"💾 <b>Storage Status</b>\n\n<pre>{html.escape(out)}</pre>" if success else f"❌ <b>Storage Check Failed</b>\n\n<pre>{html.escape(err)}</pre>"
        await query.message.edit_text(msg, reply_markup=kb_back)

    elif action == "backup":
        await query.message.edit_text("🔋 <b>Triggering Backup...</b>\nHardware LEDs will pulse during the process.")
        success, out, err = await run_kgg_cmd(["app", "backup"])
        msg = f"✅ <b>Backup Process Started</b>\n\n<pre>{html.escape(out)}</pre>" if success else f"❌ <b>Backup Failed</b>\n\n<pre>{html.escape(err)}</pre>"
        await query.message.edit_text(msg, reply_markup=kb_back)

    elif action == "velero_trigger":
        now_str = time.strftime("%Y%m%d-%H%M%S")
        backup_name = f"manual-{now_str}"
        
        await query.message.edit_text(
            f"⏳ <b>Triggering Velero cluster backup...</b>\n"
            f"Backup Name: <code>{backup_name}</code>\n"
            f"<i>(This runs a full cluster backup to S3/R2)</i>"
        )
        
        success, out, err = await run_kgg_cmd(["ops", "backup-create", backup_name], timeout=300)
        
        if success:
            await query.message.edit_text(
                f"✅ <b>Velero Backup Completed!</b>\n"
                f"Backup: <code>{backup_name}</code>\n\n"
                f"<pre>{html.escape(out)}</pre>",
                reply_markup=kb_back
            )
        else:
            await query.message.edit_text(
                f"❌ <b>Velero Backup Failed:</b>\n"
                f"Backup: <code>{backup_name}</code>\n\n"
                f"<pre>{html.escape(err or out)}</pre>",
                reply_markup=kb_back
            )

    elif action == "cnpg_menu":
        await query.message.edit_text("⏳ <b>Scanning for CNPG database clusters...</b>")
        master = _get_master(ctx.nodes)
        cmd = "sudo k3s kubectl get clusters.postgresql.cnpg.io -A -o jsonpath='{range .items[*]}{.metadata.namespace}{\"/\"}{.metadata.name}{\"\\n\"}{end}'"
        success, out, err = await run_kgg_cmd(["ssh", master, cmd])
        
        if not success or not out.strip():
            await query.message.edit_text(
                "⚠️ <b>No CNPG database clusters found.</b>\n"
                "Please verify that CloudNativePG is deployed on the cluster.",
                reply_markup=kb_back
            )
            return
            
        db_clusters = []
        for line in out.splitlines():
            line = line.strip()
            if "/" in line:
                ns, name = line.split("/", 1)
                db_clusters.append((ns, name))
                
        buttons = []
        for ns, name in db_clusters:
            # callback format: storage_cnpg_run:<ns>:<name>
            buttons.append([InlineKeyboardButton(
                text=f"💾 {ns}/{name}",
                callback_data=f"storage_cnpg_run:{ns}:{name}"
            )])
        buttons.append([InlineKeyboardButton(text="🔙 Back", callback_data="storage_main")])
        kb = InlineKeyboardMarkup(inline_keyboard=buttons)
        
        await query.message.edit_text(
            "💾 <b>Select CNPG Database Cluster to backup:</b>",
            reply_markup=kb
        )

    elif action.startswith("cnpg_run:"):
        parts = action.replace("cnpg_run:", "").split(":")
        if len(parts) != 2:
            await query.message.edit_text("❌ <b>Error:</b> Invalid callback parameter.", reply_markup=kb_back)
            return
            
        ns, db_name = parts[0], parts[1]
        now_str = time.strftime("%Y%m%d-%H%M%S")
        backup_name = f"manual-{now_str}"
        
        await query.message.edit_text(
            f"⏳ <b>Triggering CNPG Database Backup...</b>\n"
            f"Database: <code>{ns}/{db_name}</code>\n"
            f"Backup Name: <code>{backup_name}</code>\n"
            f"<i>(This triggers a physical backup to S3/R2)</i>"
        )
        
        # kgg db backup create <cluster-name> --ns <namespace> --name <backup-name>
        success, out, err = await run_kgg_cmd(
            ["db", "backup", "create", db_name, "--ns", ns, "--name", backup_name],
            timeout=180
        )
        
        if success:
            await query.message.edit_text(
                f"✅ <b>Database Backup Completed!</b>\n"
                f"Database: <code>{ns}/{db_name}</code>\n"
                f"Backup Name: <code>{backup_name}</code>\n\n"
                f"<pre>{html.escape(out)}</pre>",
                reply_markup=kb_back
            )
        else:
            await query.message.edit_text(
                f"❌ <b>Database Backup Failed:</b>\n"
                f"Database: <code>{ns}/{db_name}</code>\n"
                f"Backup Name: <code>{backup_name}</code>\n\n"
                f"<pre>{html.escape(err or out)}</pre>",
                reply_markup=kb_back
            )
