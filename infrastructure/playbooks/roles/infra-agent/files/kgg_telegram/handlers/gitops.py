"""
handlers/gitops.py - GitOps and ArgoCD operation handlers.
"""

import logging
import json
import html
from aiogram.types import Message, CallbackQuery, InlineKeyboardMarkup, InlineKeyboardButton

from kgg_telegram.helpers import run_kgg_cmd

logger = logging.getLogger(__name__)

async def handle_gitops_menu(ctx, message: Message):
    kb = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="⛵ List Projects & Apps", callback_data="gitops_list")],
        [InlineKeyboardButton(text="🔄 Sync ArgoCD State", callback_data="gitops_sync")],
        [InlineKeyboardButton(text="🔐 Sync Pull Secrets", callback_data="gitops_pull_secrets")],
        [InlineKeyboardButton(text="🚢 Kargo Promotions", callback_data="kargo_menu")],
        [InlineKeyboardButton(text="🔙 Back", callback_data="cancel_action")]
    ])
    await message.answer("⛵ <b>GitOps Management (ArgoCD & Kargo)</b>\nSelect action:", reply_markup=kb)

async def cb_gitops(ctx, query: CallbackQuery):
    await query.answer()
    action = query.data.split("_", 1)[1]
    
    if action == "list":
        await query.message.edit_text("⏳ Fetching GitOps inventory...")
        success, out, err = await run_kgg_cmd(["gitops", "list", "--json"])
        if not success:
            await query.message.edit_text(f"❌ <b>Error:</b>\n<code>{html.escape(err)}</code>")
            return
            
        try:
            data = json.loads(out)
            projects = data.get("projects", [])
            creds = data.get("credentials", [])
            
            res = "⛵ <b>GitOps Inventory</b>\n\n"
            
            if creds:
                res += "🔑 <b>Private Repos:</b>\n"
                for c in creds:
                    res += f"  • <code>{c.get('url')}</code>\n"
                res += "\n"
                
            if not projects:
                res += "<i>No projects configured.</i>"
            else:
                for p in projects:
                    res += f"📂 <b>Project:</b> {p.get('name')}\n"
                    apps = p.get("apps", [])
                    if not apps:
                        res += "  <i>(no apps)</i>\n"
                    for a in apps:
                        res += f"  └── 📦 <code>{a.get('name')}</code> ({a.get('namespace')})\n"
                    res += "\n"
            
            await query.message.edit_text(res)
        except Exception as e:
            logger.error(f"Failed to parse gitops list JSON: {e}")
            await query.message.edit_text(f"⛵ <b>GitOps Inventory (Raw):</b>\n<pre>{html.escape(out)}</pre>")

    elif action == "sync":
        await query.message.edit_text("🔄 <b>Syncing ArgoCD...</b>\nThis will reconcile your local config with the cluster.")
        success, out, err = await run_kgg_cmd(["gitops", "sync"])
        msg = f"✅ <b>ArgoCD Sync Complete</b>\n\n<pre>{html.escape(out)}</pre>" if success else f"❌ <b>ArgoCD Sync Failed</b>\n\n<pre>{html.escape(err)}</pre>"
        await query.message.answer(msg)

    elif action == "pull_secrets":
        await query.message.edit_text("🔐 <b>Syncing Pull Secrets...</b>\nDistributing registry credentials to all app namespaces.")
        success, out, err = await run_kgg_cmd(["gitops", "sync-pull-secrets"])
        msg = f"✅ <b>Pull Secrets Synced</b>\n\n<pre>{html.escape(out)}</pre>" if success else f"❌ <b>Sync Failed</b>\n\n<pre>{html.escape(err)}</pre>"
        await query.message.answer(msg)
