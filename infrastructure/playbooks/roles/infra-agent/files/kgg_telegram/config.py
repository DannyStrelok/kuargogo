"""
config.py - Environment variable loading and global constants for kgg_telegram.
All configuration is injected via systemd Environment= directives.
"""

import os
import logging
import pytz
from kgg_common import load_json_env

logger = logging.getLogger(__name__)

# --- Telegram Credentials ---
TG_TOKEN: str = os.getenv("TELEGRAM_TOKEN", "")
try:
    TG_ADMIN_ID: int = int(os.getenv("TELEGRAM_ADMIN_ID", "0"))
except (ValueError, TypeError):
    TG_ADMIN_ID = 0

# --- Timezone & Scheduler ---
TG_TIMEZONE_STR: str = os.getenv("TELEGRAM_TIMEZONE", "Europe/Madrid")
TG_TIMEZONE = pytz.timezone(TG_TIMEZONE_STR)
TG_SUMMARY_TIME: str = os.getenv("TELEGRAM_SUMMARY_TIME", "08:30")

# --- Operational Flags ---
TG_MAINTENANCE_MODE: bool = os.getenv("KGG_MAINTENANCE_MODE", "false").lower() == "true"

# --- Binary & Cluster ---
TG_KGG_BIN: str = os.getenv("KGG_BIN", "/usr/local/bin/kgg")
TG_NODES: list = load_json_env("KGG_NODES", [])
TG_ALLOWED_PATHS: list = load_json_env("ALLOWED_PATHS", ["/var/log", "/etc/kgg"])

# --- Database ---
DB_PATH: str = os.path.expanduser("~/rack.db")

# --- MQTT Broker ---
MQTT_HOST: str = "localhost"
MQTT_PORT: int = 1883

# --- Message size constants (Telegram hard limit: 4096 chars) ---
TG_MAX_MESSAGE_LEN: int = 4000
TG_MAX_FILE_DOWNLOAD_BYTES: int = 10 * 1024 * 1024  # 10 MB
