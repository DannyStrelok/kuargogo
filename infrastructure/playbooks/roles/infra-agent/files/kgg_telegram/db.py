"""
db.py - Database helpers for kgg_telegram.
All access uses sqlite3 context managers to prevent connection leaks.
"""

import logging
import os
import sqlite3

from kgg_telegram.config import DB_PATH

logger = logging.getLogger(__name__)


def get_setting(key: str, default: str = "0") -> str:
    """Read a setting value from the DB. Returns default if not found."""
    try:
        with sqlite3.connect(DB_PATH) as conn:
            row = conn.execute(
                "SELECT value FROM settings WHERE key = ?", (key,)
            ).fetchone()
            return row[0] if row else default
    except Exception as e:
        logger.debug(f"get_setting('{key}') failed: {e}")
        return default


def set_setting(key: str, value) -> None:
    """Persist a key/value pair to the settings table."""
    try:
        with sqlite3.connect(DB_PATH) as conn:
            conn.execute(
                "INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)",
                (key, str(value))
            )
    except Exception as e:
        logger.error(f"set_setting('{key}') DB error: {e}")


def fetch_incidents(limit: int = 10) -> list:
    """Fetch the most recent alerts/incidents from the DB."""
    if not os.path.exists(DB_PATH):
        return []
    try:
        with sqlite3.connect(DB_PATH) as conn:
            return conn.execute(
                "SELECT timestamp, level, node, message FROM alerts "
                "ORDER BY timestamp DESC LIMIT ?",
                (limit,)
            ).fetchall()
    except Exception as e:
        logger.error(f"Failed to fetch incidents: {e}")
        return []


def log_incident_to_db(level: str, node: str, message: str) -> None:
    """Write an incident record to the alerts table."""
    import datetime
    try:
        with sqlite3.connect(DB_PATH) as conn:
            conn.execute(
                "INSERT INTO alerts (timestamp, level, node, message) VALUES (?, ?, ?, ?)",
                (datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S"), level, node, message)
            )
    except Exception as e:
        logger.error(f"Failed to log incident to DB: {e}")


def clear_incidents() -> None:
    """Delete all records from the alerts table."""
    try:
        with sqlite3.connect(DB_PATH) as conn:
            conn.execute("DELETE FROM alerts")
    except Exception as e:
        logger.error(f"Failed to clear incidents from DB: {e}")
