"""
mqtt_bridge.py - MQTT alert listener that forwards rack alerts to Telegram.
Robust auto-reconnect and resubscription handling via background threads.
"""

import asyncio
import logging

from kgg_telegram.config import MQTT_HOST, MQTT_PORT, TG_ADMIN_ID
from kgg_telegram.db import log_incident_to_db
from kgg_common import create_mqtt_client

logger = logging.getLogger(__name__)


async def mqtt_alert_listener(ctx):
    """Async worker: subscribes to rack/alerts and forwards to Telegram chat.
    
    Uses on_connect resubscription callbacks to ensure no alerts are lost
    after broker restarts or temporary network drops.
    """
    logger.info("MQTT Alert Bridge initializing...")

    loop = asyncio.get_running_loop()
    queue: asyncio.Queue = asyncio.Queue()

    def on_connect(client, userdata, flags, rc, *args, **kwargs):
        """Fires whenever the client connects/reconnects. Resubscribes to keep session alive."""
        logger.info(f"Connected to MQTT broker (result code: {rc}). Subscribing to rack/alerts...")
        client.subscribe("rack/alerts")

    def on_message(client, userdata, msg):
        try:
            payload = msg.payload.decode()
            loop.call_soon_threadsafe(queue.put_nowait, payload)
        except Exception as e:
            logger.error(f"MQTT message decode error: {e}")

    client = create_mqtt_client()
    client.on_connect = on_connect
    client.on_message = on_message

    logger.info(f"Connecting background loop to MQTT broker at {MQTT_HOST}:{MQTT_PORT}...")
    try:
        client.connect_async(MQTT_HOST, MQTT_PORT, 60)
        client.loop_start()
    except Exception as e:
        logger.error(f"Failed to start async MQTT loop: {e}")

    while True:
        try:
            alert_msg = await queue.get()
            logger.warning(f"Proactive Alert Received: {alert_msg}")

            lower_msg = alert_msg.lower()
            if "critical" in lower_msg or "error" in lower_msg or "fail" in lower_msg:
                icon, level = "🚨", "CRITICAL"
            elif "warn" in lower_msg or "high" in lower_msg:
                icon, level = "⚠️", "WARNING"
            else:
                icon, level = "🔔", "INFO"

            log_incident_to_db(level, "System", alert_msg)

            try:
                from aiogram.types import InlineKeyboardMarkup, InlineKeyboardButton
                kb_list = [[
                    InlineKeyboardButton(text="🩺 Diagnose System", callback_data="infra_health"),
                    InlineKeyboardButton(text="🔕 Ignore", callback_data="alert_ignore")
                ]]
                import re
                node_match = re.search(r"Kubernetes Node '([^']+)' is NotReady!", alert_msg)
                if node_match:
                    node_name = node_match.group(1)
                    kb_list.insert(0, [
                        InlineKeyboardButton(text="🛠️ Remediate Node", callback_data=f"k3s_remediate:{node_name}")
                    ])

                if "SMART Check Failure" in alert_msg or "❌ Node:" in alert_msg:
                    storage_match = re.search(r"Node:\s*([^\s|]+).*Disk:\s*([^\s|]+)", alert_msg)
                    if storage_match:
                        node_name = storage_match.group(1)
                        disk_id = storage_match.group(2)
                        kb_list.insert(0, [
                            InlineKeyboardButton(text="🛠️ Evict Disk", callback_data=f"k3s_evict_disk:{node_name}:{disk_id}")
                        ])

                kb = InlineKeyboardMarkup(inline_keyboard=kb_list)
                await ctx.bot.send_message(
                    chat_id=TG_ADMIN_ID,
                    text=f"{icon} <b>Proactive Alert:</b>\n{alert_msg}",
                    reply_markup=kb
                )
            except Exception as e:
                logger.error(f"Failed to forward alert to Telegram: {e}")

        except Exception as e:
            logger.error(f"MQTT Alert Bridge processing error: {e}. Retrying in 10s...")
            await asyncio.sleep(10)
