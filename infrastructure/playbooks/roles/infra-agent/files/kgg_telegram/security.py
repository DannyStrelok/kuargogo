"""
security.py - Input validation and path safety for kgg_telegram.
All user-supplied values that reach shell commands must pass through
these validators before use.
"""

import os
import re

# Allowlist for shell arguments (deployment names, node names, etc.)
_SAFE_ARG_RE = re.compile(r'^[a-zA-Z0-9._/@:\-]+$')

# Injection pattern: blocks backticks, $(), ${}, ||, &&, ;;
SHELL_INJECTION_RE = re.compile(r'[`$]|\$\(|\$\{|\|\||&&|;;')


def sanitize_shell_arg(value: str, label: str = "input") -> str:
    """Validate that a user-supplied argument contains only safe characters.

    Blocks shell metacharacters: ; | & $ ` ( ) { } < > ! # ~ ^ *
    Raises ValueError if the value is unsafe or empty.
    """
    value = value.strip()
    if not value:
        raise ValueError(f"{label} must not be empty")
    if not _SAFE_ARG_RE.match(value):
        raise ValueError(
            f"Unsafe {label}: '{value}'. "
            "Only alphanumeric characters, '.', '-', '_', '/', '@', ':' are allowed."
        )
    return value


def is_safe_path(path: str, allowed_bases: list) -> bool:
    """Return True if path is under one of the allowed base directories.

    Resolves '..' segments via os.path.normpath to prevent path traversal.
    Does NOT require the path to exist on the local filesystem.
    """
    try:
        normalized = os.path.normpath(path)
    except Exception:
        return False
    # Must be an absolute Linux path
    if not normalized.startswith('/'):
        return False
    return any(
        normalized.startswith(os.path.normpath(base) + '/') or
        normalized == os.path.normpath(base)
        for base in allowed_bases
    )
