"""
kgg_common.py - Shared utilities for kgg_agent and kgg_telegram.
Centralises code that was duplicated across both services.
"""

import json
import logging
import os

logger = logging.getLogger(__name__)


def load_json_env(name: str, default):
    """Parse a JSON value from an environment variable, with robust error handling.

    Handles Systemd's shell escaping which can produce double-encoded JSON strings.
    """
    val = os.getenv(name)
    if not val or not val.strip():
        return default
    try:
        content = val.strip(' \t\n\r"')
        data = json.loads(content)
        # Handle double-encoded JSON strings (Systemd shell escaping)
        if isinstance(data, str):
            try:
                return json.loads(data)
            except json.JSONDecodeError:
                return data
        return data
    except Exception as e:
        logger.error(f"Failed to parse {name} environment variable: {e} | Content: '{val}'")
        return default


def create_mqtt_client():
    """Create a paho-mqtt Client compatible with both paho-mqtt v1 and v2."""
    import paho.mqtt.client as mqtt
    try:
        return mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
    except AttributeError:
        return mqtt.Client()
