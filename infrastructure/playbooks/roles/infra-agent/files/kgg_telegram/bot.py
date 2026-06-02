"""
bot.py - RackBot: the main bot class, handler wiring, and startup.
"""

import logging
import os

from aiogram import Bot, Dispatcher, F, types
from aiogram.client.default import DefaultBotProperties
from aiogram.enums import ParseMode
from aiogram.filters import Command
from aiogram.types import Message, CallbackQuery
import asyncio
import time

from kgg_telegram.config import TG_TOKEN, TG_ADMIN_ID, TG_NODES
from kgg_telegram.states import BotStates
from kgg_telegram.fsm_timeout import fsm_timeout_loop

# Handler imports
from kgg_telegram.handlers.status import handle_status
from kgg_telegram.handlers.health import handle_health_menu, cb_health
from kgg_telegram.handlers.logs import handle_logs_menu, cb_logs, cb_tail_logs
from kgg_telegram.handlers.power import handle_power_menu, cb_power, cb_confirm
from kgg_telegram.handlers.ssh import (
    handle_ssh_menu, process_ssh_node, process_ssh_mode,
    process_ssh_command, handle_ssh_close,
)
from kgg_telegram.handlers.k3s import (
    handle_k3s_menu, cb_k3s, process_k3s_name, process_k3s_replicas,
)
from kgg_telegram.handlers.network import handle_net_menu, cb_net
from kgg_telegram.handlers.files import handle_files_menu, cb_files, process_file_path
from kgg_telegram.handlers.maintenance import handle_maint_menu, cb_maint
from kgg_telegram.handlers.incidents import handle_incidents, cb_clear_incidents
from kgg_telegram.handlers.nl import (
    handle_nl, handle_cancel_action, cb_ai_analyze,
)
from kgg_telegram.handlers.gitops import handle_gitops_menu, cb_gitops
from kgg_telegram.handlers.storage import handle_storage_menu, cb_storage
from kgg_telegram.handlers.cloud import handle_cloud_menu, cb_cloud
from kgg_telegram.handlers.kargo import handle_kargo_menu, cb_kargo
from kgg_telegram.mqtt_bridge import mqtt_alert_listener
from functools import partial
from kgg_telegram.scheduler import scheduler_loop

__version__ = "0.2.0"

logger = logging.getLogger(__name__)


class RackBot:
    """Central bot class. Acts as a dependency-injection container for all handlers."""

    def __init__(self):
        self.bot = Bot(token=TG_TOKEN, default=DefaultBotProperties(parse_mode=ParseMode.HTML))
        self.dp = Dispatcher()
        self.nodes = TG_NODES
        self.pending_actions: dict = {}
        self.last_activity_time = time.time()

        self._register_handlers()

    # ------------------------------------------------------------------
    # Middleware
    # ------------------------------------------------------------------

    def _register_handlers(self):
        @self.dp.message.outer_middleware()
        async def auth_message_middleware(handler, event: Message, data):
            user_id = event.from_user.id if event.from_user else None
            if user_id != TG_ADMIN_ID:
                logger.warning(f"Unauthorized message access: {user_id} (Expected {TG_ADMIN_ID})")
                if user_id:
                    await event.answer(
                        f"⚠️ <b>Access Denied</b>\nYour ID: <code>{user_id}</code> is not "
                        "authorized for this rack.\nPlease update <code>TELEGRAM_ADMIN_ID</code>."
                    )
                return
            self.last_activity_time = time.time()
            return await handler(event, data)

        @self.dp.callback_query.outer_middleware()
        async def auth_callback_middleware(handler, event: CallbackQuery, data):
            if event.from_user.id != TG_ADMIN_ID:
                logger.warning(f"Unauthorized callback access: {event.from_user.id}")
                await event.answer("Unauthorized", show_alert=True)
                return
            self.last_activity_time = time.time()
            return await handler(event, data)

        ctx = self  # captured as 'ctx' in every handler closure

        # ------------------------------------------------------------------
        # Static button / command routes
        # ------------------------------------------------------------------
        self.dp.message(Command("start", "help"))(self.handle_help)
        self.dp.message(F.text == "❓ Help")(self.handle_help)
        self.dp.message(F.text == "📊 Status")(partial(handle_status, ctx))
        self.dp.message(F.text == "🩺 Health")(partial(handle_health_menu, ctx))
        self.dp.message(F.text == "⚡ Power")(partial(handle_power_menu, ctx))
        self.dp.message(F.text == "📝 Logs")(partial(handle_logs_menu, ctx))
        self.dp.message(F.text == "🛠 Maint")(partial(handle_maint_menu, ctx))
        self.dp.message(F.text == "💻 SSH Console")(partial(handle_ssh_menu, ctx))
        self.dp.message(F.text == "🐳 K3s Ops")(partial(handle_k3s_menu, ctx))
        self.dp.message(F.text == "📡 Network")(partial(handle_net_menu, ctx))
        self.dp.message(F.text == "📋 Incidents")(partial(handle_incidents, ctx))
        self.dp.message(F.text == "📂 Files")(partial(handle_files_menu, ctx))
        self.dp.message(F.text == "⛵ GitOps")(partial(handle_gitops_menu, ctx))
        self.dp.message(F.text == "💾 Storage")(partial(handle_storage_menu, ctx))
        self.dp.message(F.text == "☁️ Cloud")(partial(handle_cloud_menu, ctx))
        self.dp.message(F.text == "🚢 Kargo")(partial(handle_kargo_menu, ctx))
        self.dp.message(Command("kargo"))(partial(handle_kargo_menu, ctx))

        self.dp.message(Command("nodes"))(partial(handle_status, ctx))
        self.dp.message(Command("logs"))(partial(handle_logs_menu, ctx))

        # ------------------------------------------------------------------
        # Callback routes
        # ------------------------------------------------------------------
        self.dp.callback_query(F.data == "cancel_action")(partial(handle_cancel_action, ctx))
        self.dp.callback_query(F.data == "ssh_close")(partial(handle_ssh_close, ctx))
        self.dp.callback_query(F.data.startswith("logs_"))(partial(cb_logs, ctx))
        self.dp.callback_query(F.data.startswith("taillogs:"))(partial(cb_tail_logs, ctx))
        self.dp.callback_query(F.data.startswith("pwr_"))(partial(cb_power, ctx))
        self.dp.callback_query(F.data.startswith("k3s_"))(partial(cb_k3s, ctx))
        self.dp.callback_query(F.data.startswith("maint_"))(partial(cb_maint, ctx))
        self.dp.callback_query(F.data.startswith("hlth_"))(partial(cb_health, ctx))
        self.dp.callback_query(F.data == "confirm_clear_incidents")(partial(cb_clear_incidents, ctx))
        self.dp.callback_query(F.data.startswith("confirm_"))(partial(cb_confirm, ctx))
        self.dp.callback_query(F.data == "infra_health")(partial(handle_health_menu, ctx))
        self.dp.callback_query(F.data == "alert_ignore")(partial(handle_cancel_action, ctx))
        self.dp.callback_query(F.data.startswith("files_"))(partial(cb_files, ctx))
        self.dp.callback_query(F.data.startswith("ai_"))(partial(cb_ai_analyze, ctx))
        self.dp.callback_query(F.data.startswith("net:"))(partial(cb_net, ctx))
        self.dp.callback_query(F.data.startswith("gitops_"))(partial(cb_gitops, ctx))
        self.dp.callback_query(F.data.startswith("storage_"))(partial(cb_storage, ctx))
        self.dp.callback_query(F.data.startswith("cloud_"))(partial(cb_cloud, ctx))
        self.dp.callback_query(F.data.startswith("kargo_"))(partial(cb_kargo, ctx))

        # ------------------------------------------------------------------
        # SSH Console FSM
        # ------------------------------------------------------------------
        self.dp.callback_query(F.data.startswith("ssh_node_"))(partial(process_ssh_node, ctx))
        self.dp.callback_query(F.data.startswith("ssh_mode_"))(partial(process_ssh_mode, ctx))
        self.dp.message(BotStates.waiting_for_command)(partial(process_ssh_command, ctx))

        # ------------------------------------------------------------------
        # K3s FSM
        # ------------------------------------------------------------------
        self.dp.message(BotStates.waiting_for_k3s_name)(partial(process_k3s_name, ctx))
        self.dp.message(BotStates.waiting_for_k3s_replicas)(partial(process_k3s_replicas, ctx))

        # ------------------------------------------------------------------
        # File Browser FSM
        # ------------------------------------------------------------------
        self.dp.message(BotStates.waiting_for_path)(partial(process_file_path, ctx))

        # ------------------------------------------------------------------
        # Catch-all NL fallback (must be last)
        # ------------------------------------------------------------------
        self.dp.message()(partial(handle_nl, ctx))

    # ------------------------------------------------------------------
    # Help / Main Menu
    # ------------------------------------------------------------------

    async def handle_help(self, message: Message):
        kb = types.ReplyKeyboardMarkup(
            keyboard=[
                [types.KeyboardButton(text="📊 Status"), types.KeyboardButton(text="🩺 Health")],
                [types.KeyboardButton(text="📝 Logs"), types.KeyboardButton(text="⚡ Power")],
                [types.KeyboardButton(text="💻 SSH Console"), types.KeyboardButton(text="🐳 K3s Ops")],
                [types.KeyboardButton(text="📂 Files"), types.KeyboardButton(text="📡 Network")],
                [types.KeyboardButton(text="🛠 Maint"), types.KeyboardButton(text="📋 Incidents")],
                [types.KeyboardButton(text="⛵ GitOps"), types.KeyboardButton(text="🚢 Kargo"), types.KeyboardButton(text="💾 Storage")],
                [types.KeyboardButton(text="☁️ Cloud"), types.KeyboardButton(text="❓ Help")]
            ],
            resize_keyboard=True
        )

        nodes_list = ", ".join([n["name"] for n in self.nodes])
        help_text = (
            "🖥 <b>Kuargogo Control</b>\n\n"
            "Use the buttons below to manage your rack. For node-specific actions, "
            "press a button and then select the node.\n\n"
            f"📡 <b>Active Nodes:</b> {nodes_list}"
        )

        if message.text and message.text.startswith("/start"):
            base_dir = os.path.dirname(os.path.abspath(__file__))
            logo_path = os.path.join(base_dir, "..", "rack_logo.png")
            if os.path.exists(logo_path):
                try:
                    from aiogram.types import FSInputFile
                    await message.answer_photo(
                        photo=FSInputFile(logo_path), caption=help_text, reply_markup=kb
                    )
                    return
                except Exception as e:
                    logger.error(f"Failed to send logo: {e}")

        await message.answer(help_text, reply_markup=kb)

    # ------------------------------------------------------------------
    # Startup
    # ------------------------------------------------------------------

    async def start(self):
        logger.info(f"Starting kgg-telegram bot (v{__version__})...")
        asyncio.create_task(scheduler_loop(self))
        asyncio.create_task(mqtt_alert_listener(self))
        asyncio.create_task(fsm_timeout_loop(self))
        await self.bot.delete_webhook(drop_pending_updates=True)
        await self.dp.start_polling(self.bot)
