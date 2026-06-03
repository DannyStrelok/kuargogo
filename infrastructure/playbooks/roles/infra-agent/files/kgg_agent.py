"""
kgg_agent.py - Kuargogo Infrastructure Super-Agent ("The Brain")
Monitors node health, triggers autonomous recovery, and publishes alerts via MQTT.
All configuration is injected via environment variables from systemd.
"""

import time
import json
import threading
import sqlite3
import os
import subprocess
import logging
from logging.handlers import RotatingFileHandler
import html

__version__ = "0.2.0"

# --- Logging Setup ---
LOG_PATH = os.path.expanduser("~/rack.log")
log_handler = RotatingFileHandler(LOG_PATH, maxBytes=1_000_000, backupCount=3)
log_handler.setFormatter(logging.Formatter('%(asctime)s [%(levelname)s] %(message)s'))
logger = logging.getLogger("rack-brain")
logger.setLevel(logging.INFO)
logger.addHandler(log_handler)
logger.addHandler(logging.StreamHandler())

# --- Configuration constants ---
# Cooldown between repeat alerts for the same node (seconds)
ALERT_COOLDOWN_SECONDS = int(os.getenv("KGG_ALERT_COOLDOWN", "300"))
# Consecutive ping failures before triggering Level-1 recovery
RECOVERY_L1_THRESHOLD = int(os.getenv("KGG_RECOVERY_L1", "3"))
# Consecutive ping failures before triggering Level-2 recovery (WoL)
RECOVERY_L2_THRESHOLD = int(os.getenv("KGG_RECOVERY_L2", "6"))
# Disk usage percentage to trigger a warning alert
DISK_WARN_THRESHOLD = int(os.getenv("KGG_DISK_WARN_PCT", "85"))
# How often to check disk usage (seconds)
DISK_CHECK_INTERVAL = int(os.getenv("KGG_DISK_CHECK_INTERVAL", "600"))
# How often to check node health (seconds)
NODE_CHECK_INTERVAL = int(os.getenv("KGG_NODE_CHECK_INTERVAL", "30"))

# --- Configuration (Read from environment variables) ---
from kgg_common import load_json_env

AGENT_DB_PATH = os.path.expanduser("~/rack.db")
AGENT_KGG_BIN = os.getenv("KGG_BIN", "/usr/local/bin/kgg")
AGENT_NODES = load_json_env("KGG_NODES", [])
AGENT_CLUSTER_KEY_PATH = os.getenv("KGG_CLUSTER_KEY_PATH", "")
AGENT_WOL_INTERFACE = os.getenv("KGG_WOL_INTERFACE", "eth0")
AGENT_NODE_NAME = os.getenv("KGG_NODE_NAME", "unknown-node")

# --- MQTT Config ---
AGENT_MQTT_HOST = "localhost"
AGENT_MQTT_PORT = 1883
AGENT_MQTT_CLIENT = None

# Identify local IP to filter self-alerts
AGENT_SELF_IP = next((n.get('ip') for n in AGENT_NODES if n.get('name') == AGENT_NODE_NAME), None)

db_lock = threading.Lock()
alert_lock = threading.Lock()

# Alert State (Cooldown to prevent spam)
alert_state = {
    "node_alerts": {},       # {node_name: last_alert_time}
    "node_failures": {},     # {node_name: consecutive_failed_pings}
    "recovering_nodes": set(), # Nodes currently being recovery-processed
    "k3s_node_alerts": {},   # {node_name: alert_sent}
    "k3s_node_failures": {}, # {node_name: consecutive_notready_checks}
    "k3s_volume_alerts": {}, # {vol_name: alert_sent}
}

# --- Alerting ---
def send_alert(message):
    """Publish a proactive alert to MQTT. (The Voice service handles Telegram forwarding)."""
    logger.warning(f"ALERT: {message}")
    if AGENT_MQTT_CLIENT:
        try:
            AGENT_MQTT_CLIENT.publish("rack/alerts", message)
        except Exception as e:
            logger.error(f"Failed to publish alert to MQTT: {e}")
    publish_status({"alert": message})

def log_incident(level_str, node_name, message):
    """Log an incident to the database and logger."""
    try:
        logger.info(f"Incident [{level_str}] {node_name}: {message}")
        with db_lock:
            with sqlite3.connect(AGENT_DB_PATH) as conn:
                conn.execute(
                    "INSERT INTO alerts (timestamp, node, level, message) VALUES (?, ?, ?, ?)",
                    (int(time.time()), node_name, level_str, message)
                )
    except Exception as e:
        logger.warning(f"Failed to log incident: {e}")

def attempt_node_recovery(node, attempt_level):
    """Attempt autonomous recovery for a frozen/dead node using kuargogo."""
    if load_setting("maintenance_mode", "0") == "1" or \
       load_setting(f"maintenance_{node['name']}", "0") == "1":
        logger.info(f"Aborting recovery for '{node['name']}': Node is in maintenance mode.")
        return False

    try:
        if attempt_level == 1:
            msg = f"Attempting Recovery [Level 1: SSH Reboot] for '{node['name']}'..."
            send_alert(f"🔄 {msg}")
            log_incident("RECOVERY", node['name'], msg)
            cmd = [AGENT_KGG_BIN, "pwr", "reboot", node['name']]
            try:
                res = subprocess.run(cmd, capture_output=True, text=True, timeout=20)
                if res.returncode == 0:
                    logger.info(f"SSH Reboot command sent via kuargogo to {node['name']}")
                    return True
                logger.warning(f"kgg pwr reboot failed for {node['name']}: {res.stderr}")
            except Exception as e:
                logger.warning(f"kuargogo recovery failed for {node['name']}: {e}")

        elif attempt_level == 2:
            msg = f"Attempting Recovery [Level 2: WoL] for '{node['name']}'..."
            send_alert(f"⚡ {msg}")
            log_incident("RECOVERY", node['name'], msg)
            cmd = [AGENT_KGG_BIN, "pwr", "on", node['name']]
            try:
                res = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
                if res.returncode == 0:
                    logger.info(f"WoL packet sent via kuargogo to {node['name']}")
                    return True
                logger.warning(f"kgg pwr on failed for {node['name']}: {res.stderr}")
            except Exception as e:
                logger.warning(f"kuargogo WoL recovery failed for {node['name']}: {e}")

        return False
    finally:
        with alert_lock:
            alert_state["recovering_nodes"].discard(node['name'])

def check_node_health():
    """Ping all nodes and trigger alerts/recovery for failures."""
    if load_setting("maintenance_mode", "0") == "1":
        logger.debug("Skipping node health checks: Global Maintenance Mode is active.")
        return

    now = time.time()
    for node in AGENT_NODES:
        if load_setting(f"maintenance_{node['name']}", "0") == "1":
            continue
        try:
            rc = subprocess.call(
                ['ping', '-c', '1', '-W', '1', node['ip']],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL
            )
            if rc != 0:
                with alert_lock:
                    failures = alert_state["node_failures"].get(node['name'], 0) + 1
                    alert_state["node_failures"][node['name']] = failures
                    last_alert = alert_state["node_alerts"].get(node['name'], 0)
                    if now - last_alert > ALERT_COOLDOWN_SECONDS:
                        msg = f"Node '{node['name']}' ({node['ip']}) is OFFLINE! (Ping failures: {failures})"
                        send_alert(msg)
                        log_incident("DOWN", node['name'], msg)
                        publish_node_status(node['name'], "offline")
                        alert_state["node_alerts"][node['name']] = now

                with alert_lock:
                    is_recovering = node['name'] in alert_state["recovering_nodes"]

                if not is_recovering:
                    if failures == RECOVERY_L1_THRESHOLD:
                        with alert_lock:
                            alert_state["recovering_nodes"].add(node['name'])
                        attempt_node_recovery(node, 1)
                    elif failures == RECOVERY_L2_THRESHOLD:
                        with alert_lock:
                            alert_state["recovering_nodes"].add(node['name'])
                        attempt_node_recovery(node, 2)
                    elif failures % 10 == 0:
                        logger.warning(f"Node '{node['name']}' still down ({failures} consecutive ping failures)")
            else:
                with alert_lock:
                    prev_failures = alert_state["node_failures"].get(node['name'], 0)
                    if prev_failures > 0:
                        msg = f"Node '{node['name']}' has RECOVERED after {prev_failures} failures."
                        logger.info(msg)
                        send_alert(f"✅ {msg}")
                        log_incident("UP", node['name'], msg)
                        publish_node_status(node['name'], "online")
                    alert_state["node_failures"][node['name']] = 0
                    alert_state["node_alerts"].pop(node['name'], None)
        except Exception as e:
            logger.debug(f"Node health check error for {node.get('name', '?')}: {e}")

def run_k3s_node_remediation(node_name):
    """Run the kgg cluster remediate command in a background thread."""
    if load_setting("maintenance_mode", "0") == "1" or \
       load_setting(f"maintenance_{node_name}", "0") == "1":
        logger.info(f"Aborting autonomous remediation for '{node_name}': Node is in maintenance mode.")
        with alert_lock:
            alert_state["recovering_nodes"].discard(node_name)
        return

    try:
        msg = f"🐳 Kubernetes Node '{node_name}' has been NotReady for 20 checks (10 minutes). Initiating autonomous K3s Node Remediation..."
        send_alert(msg)
        log_incident("RECOVERY", node_name, msg)

        # Run kgg command with 5-minute timeout
        cmd = [AGENT_KGG_BIN, "cluster", "remediate", "--name", node_name]
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=300)

        if res.returncode == 0:
            success_msg = f"✅ Autonomous remediation for Kubernetes Node '{node_name}' completed successfully."
            send_alert(success_msg)
            log_incident("RECOVERY", node_name, success_msg)
        else:
            stderr_esc = html.escape(res.stderr or res.stdout or "Unknown error")
            err_msg = f"❌ Autonomous remediation for Kubernetes Node '{node_name}' failed!\nError: {stderr_esc}"
            send_alert(err_msg)
            log_incident("RECOVERY", node_name, err_msg)
    except Exception as e:
        err_msg = f"❌ Autonomous remediation error for Kubernetes Node '{node_name}': {html.escape(str(e))}"
        send_alert(err_msg)
        log_incident("RECOVERY", node_name, err_msg)
    finally:
        with alert_lock:
            alert_state["recovering_nodes"].discard(node_name)

def check_k3s_health():
    """Check K3s node states and Longhorn volume states via kgg ssh to a master node."""
    if load_setting("k3s_monitoring", "0") == "0":
        return

    master = next(
        (n for n in AGENT_NODES if n.get('role') in ['server', 'master', 'control-plane']),
        None
    )
    if not master:
        return

    with alert_lock:
        failures = alert_state["node_failures"].get(master['name'], 0)
    if failures > 0:
        logger.debug(f"Skipping K3s health checks: Master '{master['name']}' is currently failing pings.")
        return

    # Check K3s node readiness
    try:
        res = subprocess.run(
            [AGENT_KGG_BIN, "ssh", master['name'], "sudo k3s kubectl get nodes -o json"],
            capture_output=True, text=True, timeout=15
        )
        if res.returncode == 0:
            data = json.loads(res.stdout)
            active_nodes = set()
            for item in data.get('items', []):
                node_name = item['metadata']['name']
                active_nodes.add(node_name)
                conditions = item.get('status', {}).get('conditions', [])
                ready = next((c for c in conditions if c['type'] == 'Ready'), None)
                with alert_lock:
                    is_ready = ready and ready['status'] == 'True'
                    if not is_ready:
                        # Increment consecutive failures
                        notready_count = alert_state["k3s_node_failures"].get(node_name, 0) + 1
                        alert_state["k3s_node_failures"][node_name] = notready_count

                        if not alert_state["k3s_node_alerts"].get(node_name):
                            send_alert(f"🐳 Kubernetes Node '{node_name}' is NotReady!\nReason: {ready.get('reason') if ready else 'Unknown'}")
                            alert_state["k3s_node_alerts"][node_name] = True

                        # Handle autonomous remediation
                        remediation_enabled = load_setting("k3s_remediation", "0") == "1"
                        if remediation_enabled and node_name not in alert_state["recovering_nodes"]:
                            if notready_count == 19:
                                send_alert(f"⚠️ Warning: Kubernetes Node '{node_name}' has been NotReady for 19 checks (9.5 minutes). Autonomous remediation will trigger in 30 seconds.")
                            elif notready_count >= 20:
                                alert_state["recovering_nodes"].add(node_name)
                                threading.Thread(
                                    target=run_k3s_node_remediation,
                                    args=(node_name,),
                                    daemon=True
                                ).start()
                    else:
                        alert_state["k3s_node_failures"][node_name] = 0
                        if alert_state["k3s_node_alerts"].get(node_name):
                            send_alert(f"✅ Kubernetes Node '{node_name}' has recovered and is Ready.")
                            alert_state["k3s_node_alerts"][node_name] = False

            # Cleanup stale node state
            with alert_lock:
                for name in list(alert_state["k3s_node_failures"].keys()):
                    if name not in active_nodes:
                        alert_state["k3s_node_failures"].pop(name, None)
                        alert_state["k3s_node_alerts"].pop(name, None)

    except Exception as e:
        logger.debug(f"K3s node health check failed: {e}")

    # Check Longhorn volume health
    try:
        res = subprocess.run(
            [AGENT_KGG_BIN, "ssh", master['name'],
             "sudo k3s kubectl get volumes.longhorn.io -n longhorn-system -o json"],
            capture_output=True, text=True, timeout=20
        )
        if res.returncode == 0:
            data = json.loads(res.stdout)
            for item in data.get('items', []):
                vol_name = item['metadata']['name']
                robustness = item.get('status', {}).get('robustness', 'unknown')
                with alert_lock:
                    if robustness in ['degraded', 'faulted']:
                        if not alert_state["k3s_volume_alerts"].get(vol_name):
                            send_alert(f"💾 Longhorn Volume '{vol_name}' is {robustness.upper()}!")
                            alert_state["k3s_volume_alerts"][vol_name] = True
                    elif robustness == 'healthy':
                        if alert_state["k3s_volume_alerts"].get(vol_name):
                            send_alert(f"✅ Longhorn Volume '{vol_name}' has recovered and is healthy.")
                            alert_state["k3s_volume_alerts"][vol_name] = False
    except Exception as e:
        logger.debug(f"Longhorn volume health check failed: {e}")

# --- Database ---
def init_db():
    """Initialize database tables if they don't exist."""
    with db_lock:
        with sqlite3.connect(AGENT_DB_PATH) as conn:
            conn.executescript('''
                CREATE TABLE IF NOT EXISTS history
                    (timestamp INTEGER, metric TEXT, value REAL);
                CREATE TABLE IF NOT EXISTS settings
                    (key TEXT PRIMARY KEY, value TEXT);
                CREATE TABLE IF NOT EXISTS alerts
                    (timestamp INTEGER, node TEXT, level TEXT, message TEXT);
            ''')

def save_setting(key, value):
    """Persist a key/value pair to the settings table."""
    try:
        with db_lock:
            with sqlite3.connect(AGENT_DB_PATH) as conn:
                conn.execute(
                    "INSERT OR REPLACE INTO settings VALUES (?, ?)",
                    (key, str(value))
                )
    except Exception as e:
        logger.warning(f"Failed to save setting '{key}': {e}")

def load_setting(key, default=None):
    """Read a setting from the database, returning default if not found."""
    try:
        with db_lock:
            with sqlite3.connect(AGENT_DB_PATH) as conn:
                row = conn.execute(
                    "SELECT value FROM settings WHERE key = ?", (key,)
                ).fetchone()
        return row[0] if row else default
    except Exception as e:
        logger.debug(f"Failed to load setting '{key}': {e}")
        return default

def log_data(metric, value):
    """Record a time-series metric datapoint."""
    try:
        with db_lock:
            with sqlite3.connect(AGENT_DB_PATH) as conn:
                conn.execute(
                    "INSERT INTO history VALUES (?, ?, ?)",
                    (int(time.time()), metric, value)
                )
    except Exception as e:
        logger.debug(f"Failed to log metric '{metric}': {e}")

# --- MQTT Helpers ---
from kgg_common import create_mqtt_client

def setup_mqtt():
    """Connect to the local MQTT broker and start the background loop."""
    global AGENT_MQTT_CLIENT
    try:
        AGENT_MQTT_CLIENT = create_mqtt_client()
        AGENT_MQTT_CLIENT.connect(AGENT_MQTT_HOST, AGENT_MQTT_PORT, 60)
        AGENT_MQTT_CLIENT.loop_start()
        logger.info(f"Connected to local MQTT broker at {AGENT_MQTT_HOST}:{AGENT_MQTT_PORT}")
        return True
    except Exception as e:
        logger.warning(f"Failed to connect to MQTT: {e}. Alerts will not be published.")
        return False

def publish_status(status_dict):
    """Publish a status dict to rack/status and individual metric topics."""
    if AGENT_MQTT_CLIENT:
        try:
            AGENT_MQTT_CLIENT.publish("rack/status", json.dumps(status_dict))
            for key, val in status_dict.items():
                AGENT_MQTT_CLIENT.publish(f"rack/metrics/{key}", str(val))
        except Exception as e:
            logger.debug(f"MQTT publish_status failed: {e}")

def publish_node_status(node_name, state):
    """Publish per-node online/offline state."""
    if AGENT_MQTT_CLIENT:
        try:
            AGENT_MQTT_CLIENT.publish(f"rack/nodes/{node_name}/state", state, qos=1)
        except Exception as e:
            logger.debug(f"MQTT publish node status failed: {e}")

# --- Disk Usage Monitoring ---
def check_disk_usage():
    """Check disk usage on all online nodes and alert if above threshold."""
    for node in AGENT_NODES:
        if load_setting(f"maintenance_{node['name']}", "0") == "1":
            continue
        with alert_lock:
            if alert_state["node_failures"].get(node['name'], 0) > 0:
                continue
        try:
            res = subprocess.run(
                [AGENT_KGG_BIN, "ssh", node['name'],
                 "df --output=target,pcent -x tmpfs -x devtmpfs | tail -n +2"],
                capture_output=True, text=True, timeout=15
            )
            if res.returncode == 0:
                for line in res.stdout.strip().split('\n'):
                    parts = line.split()
                    if len(parts) >= 2:
                        mount = parts[0]
                        try:
                            pct = int(parts[1].replace('%', ''))
                        except ValueError:
                            continue
                        alert_key = f"disk_{node['name']}_{mount}"
                        with alert_lock:
                            if pct >= DISK_WARN_THRESHOLD:
                                if not alert_state.get(alert_key):
                                    send_alert(
                                        f"💾 Disk usage WARNING on {node['name']}:{mount} — {pct}% full!"
                                    )
                                    alert_state[alert_key] = True
                            else:
                                if alert_state.get(alert_key):
                                    send_alert(
                                        f"✅ Disk usage on {node['name']}:{mount} back to normal ({pct}%)"
                                    )
                                    alert_state[alert_key] = False
        except Exception as e:
            logger.debug(f"Disk check failed for {node.get('name', '?')}: {e}")

# --- UDP Alert Listener ---
def udp_alert_listener():
    """Listen for incoming UDP alerts from other nodes (e.g. SSH logins)."""
    import socket
    UDP_PORT = 5555
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        sock.bind(("0.0.0.0", UDP_PORT))
        logger.info(f"UDP Alert Listener started on port {UDP_PORT}")
    except Exception as e:
        logger.error(f"Failed to start UDP listener on port {UDP_PORT}: {e}")
        return

    while True:
        try:
            data, addr = sock.recvfrom(1024)
            payload = json.loads(data.decode())
            if payload.get('event') == 'ssh_login':
                node = payload.get('node', 'unknown')
                user = payload.get('user', 'unknown')
                ip = payload.get('ip', 'unknown')

                if load_setting(f"maintenance_{node}", "0") == "1":
                    continue
                # Suppress alerts from the agent's own IP
                if AGENT_SELF_IP and ip == AGENT_SELF_IP:
                    continue

                msg = (
                    f"🔐 <b>Remote SSH Login</b>\n"
                    f"Node: <code>{node}</code>\n"
                    f"User: <code>{user}</code>\n"
                    f"From IP: <code>{ip}</code>"
                )
                send_alert(msg)
                log_incident("SECURITY", node, f"Remote SSH Login by {user} from {ip}")
        except json.JSONDecodeError as e:
            logger.debug(f"UDP Listener: invalid JSON from {addr}: {e}")
        except Exception as e:
            logger.warning(f"UDP Listener error: {e}")
            time.sleep(1)

# --- Main Logic Loop ---
def brain_loop():
    """Main monitoring loop. Spawns health checks on their configured intervals."""
    last_node_check = time.time() - NODE_CHECK_INTERVAL + 5  # First check after 5s
    last_disk_check = time.time()

    while True:
        now = time.time()
        if now - last_node_check >= NODE_CHECK_INTERVAL:
            last_node_check = now
            threading.Thread(target=check_node_health, daemon=True).start()
            threading.Thread(target=check_k3s_health, daemon=True).start()

        if now - last_disk_check >= DISK_CHECK_INTERVAL:
            last_disk_check = now
            threading.Thread(target=check_disk_usage, daemon=True).start()

        time.sleep(1)

# --- Entry Point ---
if __name__ == "__main__":
    logger.info(f"=== Kuargogo Brain Starting (v{__version__}) ===")
    logger.info(f"Monitoring {len(AGENT_NODES)} node(s) | Node check: {NODE_CHECK_INTERVAL}s | Disk check: {DISK_CHECK_INTERVAL}s")

    try:
        pid_dir = "/run/kgg-agent"
        if os.path.exists(pid_dir):
            with open(f"{pid_dir}/kgg-agent.pid", "w") as f:
                f.write(str(os.getpid()))
    except Exception as e:
        logger.warning(f"Failed to write PID file: {e}")

    init_db()
    setup_mqtt()

    t_brain = threading.Thread(target=brain_loop, daemon=True)
    t_brain.start()

    t_udp = threading.Thread(target=udp_alert_listener, daemon=True)
    t_udp.start()

    t_brain.join()
    t_udp.join()
