"""
helpers.py - Core async utilities and UI rendering helpers for kgg_telegram.
"""

import asyncio
import logging

from kgg_telegram.config import TG_KGG_BIN, TG_MAX_MESSAGE_LEN

logger = logging.getLogger(__name__)


async def run_kgg_cmd(args: list, timeout: int = 90) -> tuple[bool, str, str]:
    """Run a kuargogo sub-command asynchronously.

    Returns (success, stdout, stderr).
    Kills the subprocess and returns an error tuple on timeout.
    """
    if not args:
        return False, "", "No command provided"
    try:
        proc = await asyncio.create_subprocess_exec(
            TG_KGG_BIN, *args,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=timeout)
        except asyncio.TimeoutError:
            proc.kill()
            await proc.communicate()  # Drain to prevent resource leak
            return False, "", f"Command timed out after {timeout}s"
        return proc.returncode == 0, stdout.decode('utf-8', errors='replace').strip(), stderr.decode('utf-8', errors='replace').strip()
    except Exception as e:
        return False, "", str(e)


async def run_kgg_cmd_raw(args: list, timeout: int = 90) -> tuple[bool, bytes, bytes]:
    """Run a kuargogo sub-command asynchronously and return raw bytes."""
    if not args:
        return False, b"", b"No command provided"
    try:
        proc = await asyncio.create_subprocess_exec(
            TG_KGG_BIN, *args,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=timeout)
        except asyncio.TimeoutError:
            proc.kill()
            await proc.communicate()  # Drain to prevent resource leak
            return False, b"", f"Command timed out after {timeout}s".encode()
        return proc.returncode == 0, stdout, stderr
    except Exception as e:
        return False, b"", str(e).encode()


def truncate(text: str, limit: int = TG_MAX_MESSAGE_LEN) -> str:
    """Truncate text to Telegram's safe message length."""
    if len(text) > limit:
        return text[:limit] + "...(truncated)"
    return text


def parse_usage(percentage) -> float:
    """Parse a percentage string like '72%' into a float."""
    try:
        return float(str(percentage).replace("%", "").strip())
    except Exception:
        return 0.0


def render_bar(percentage, length: int = 10) -> str:
    """Render a Unicode block progress bar for a percentage value."""
    try:
        p = float(str(percentage).replace("%", "").strip())
    except Exception:
        return "[----------]"
    filled = max(0, min(length, int((p / 100) * length)))
    bar = "█" * filled + "▒" * (length - filled)
    return f"<code>{bar}</code> {int(p)}%"
