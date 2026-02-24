#!/usr/bin/env python3
"""Bridge srsRAN gNB JSON metrics to Prometheus.

srsRAN exposes metrics via a WebSocket server on its remote_control port
(default 8001).  A client connects, sends {"cmd": "metrics_subscribe"},
and receives a continuous stream of JSON objects.

This script subscribes to that WebSocket, parses the JSON, and serves a
/metrics endpoint in Prometheus exposition format.

No external dependencies — uses only the Python standard library.
"""
import argparse
import base64
import hashlib
import json
import os
import socket
import struct
import threading
import time
from contextlib import suppress
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Dict, Tuple
from urllib.parse import urlparse

MetricKey = Tuple[str, Tuple[Tuple[str, str], ...]]


class MetricStore:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._values: Dict[MetricKey, float] = {}
        self._helps: Dict[str, str] = {
            # Scheduler cell metrics
            "srsran_sched_error_indication_count": "Scheduler error indications received from lower layers.",
            "srsran_sched_average_latency_us": "Average scheduler decision latency in microseconds.",
            "srsran_sched_max_latency_us": "Maximum scheduler decision latency in microseconds.",
            "srsran_sched_late_dl_harqs": "Failed PDSCH allocations due to late HARQs.",
            "srsran_sched_late_ul_harqs": "Failed PUSCH allocations due to late HARQs.",
            "srsran_sched_avg_prach_delay_slots": "Average PRACH delay in slots.",
            "srsran_sched_nof_failed_pdcch_allocs": "Failed PDCCH allocation attempts.",
            "srsran_sched_nof_failed_uci_allocs": "Failed UCI allocation attempts.",
            # Per-UE scheduler metrics
            "srsran_ue_dl_nof_ok": "Successful DL HARQ transmissions per UE.",
            "srsran_ue_dl_nof_nok": "Failed DL HARQ transmissions per UE.",
            "srsran_ue_ul_nof_ok": "Successful UL HARQ transmissions per UE.",
            "srsran_ue_ul_nof_nok": "Failed UL HARQ transmissions per UE.",
            "srsran_ue_dl_brate_bps": "DL bitrate in bits per second per UE.",
            "srsran_ue_ul_brate_bps": "UL bitrate in bits per second per UE.",
            "srsran_ue_dl_mcs": "DL MCS index per UE.",
            "srsran_ue_ul_mcs": "UL MCS index per UE.",
            "srsran_ue_cqi": "Channel Quality Indicator per UE.",
            "srsran_ue_ri": "Rank Indicator per UE.",
            "srsran_ue_pucch_snr_db": "PUCCH signal-to-noise ratio in dB per UE.",
            "srsran_ue_pusch_snr_db": "PUSCH signal-to-noise ratio in dB per UE.",
            "srsran_ue_bsr": "Buffer Status Report value per UE.",
            "srsran_ue_last_phr": "Last Power Headroom Report per UE.",
            # Cell aggregate metrics
            "srsran_active_ues": "Number of active UEs reported by the scheduler.",
            # MAC DL metrics
            "srsran_mac_dl_average_latency_us": "Average MAC DL slot handling latency in microseconds.",
            "srsran_mac_dl_max_latency_us": "Maximum MAC DL slot handling latency in microseconds.",
            "srsran_mac_dl_min_latency_us": "Minimum MAC DL slot handling latency in microseconds.",
            "srsran_mac_dl_cpu_usage_percent": "MAC DL slot handling CPU usage percent.",
            # Exporter self-monitoring
            "srsran_exporter_ws_messages_total": "Total WebSocket messages received by exporter.",
            "srsran_exporter_ws_parse_errors_total": "Total WebSocket messages that failed JSON parsing.",
            "srsran_exporter_ws_connections_total": "Total WebSocket connection attempts.",
            "srsran_exporter_ws_connected": "Whether the exporter is currently connected to the gNB (1=yes, 0=no).",
            "srsran_exporter_last_message_unixtime": "Unix timestamp of last WebSocket message reception.",
        }
        self._types: Dict[str, str] = {
            "srsran_exporter_ws_messages_total": "counter",
            "srsran_exporter_ws_parse_errors_total": "counter",
            "srsran_exporter_ws_connections_total": "counter",
            "srsran_exporter_ws_connected": "gauge",
            "srsran_exporter_last_message_unixtime": "gauge",
        }
        self.set("srsran_exporter_ws_messages_total", 0)
        self.set("srsran_exporter_ws_parse_errors_total", 0)
        self.set("srsran_exporter_ws_connections_total", 0)
        self.set("srsran_exporter_ws_connected", 0)
        self.set("srsran_exporter_last_message_unixtime", 0)

    @staticmethod
    def _key(name: str, labels: Dict[str, str] | None = None) -> MetricKey:
        label_items = tuple(sorted((labels or {}).items()))
        return name, label_items

    def set(self, name: str, value: float, labels: Dict[str, str] | None = None) -> None:
        with self._lock:
            self._values[self._key(name, labels)] = float(value)

    def inc(self, name: str, amount: float = 1, labels: Dict[str, str] | None = None) -> None:
        with self._lock:
            key = self._key(name, labels)
            self._values[key] = self._values.get(key, 0.0) + float(amount)

    def render(self) -> str:
        with self._lock:
            grouped: Dict[str, list[Tuple[Tuple[Tuple[str, str], ...], float]]] = {}
            for (name, labels), value in self._values.items():
                grouped.setdefault(name, []).append((labels, value))

        lines: list[str] = []
        for name in sorted(grouped.keys()):
            help_text = self._helps.get(name, f"{name} exported from srsRAN JSON metrics.")
            metric_type = self._types.get(name, "gauge")
            lines.append(f"# HELP {name} {help_text}")
            lines.append(f"# TYPE {name} {metric_type}")
            for labels, value in grouped[name]:
                if labels:
                    label_text = ",".join(f'{k}="{v}"' for k, v in labels)
                    lines.append(f"{name}{{{label_text}}} {value}")
                else:
                    lines.append(f"{name} {value}")
        return "\n".join(lines) + "\n"


class MetricsHandler(BaseHTTPRequestHandler):
    store: MetricStore

    def do_GET(self) -> None:
        if self.path != "/metrics":
            self.send_response(404)
            self.end_headers()
            return

        payload = self.store.render().encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format: str, *args) -> None:
        return


def parse_number(value):
    if isinstance(value, (int, float)):
        return float(value)
    return None


def set_if_number(store: MetricStore, name: str, labels: Dict[str, str], value) -> None:
    parsed = parse_number(value)
    if parsed is not None:
        store.set(name, parsed, labels)


def parse_scheduler_metrics(store: MetricStore, message: dict) -> None:
    cells = message.get("cells")
    if not isinstance(cells, list):
        return

    for idx, cell in enumerate(cells):
        if not isinstance(cell, dict):
            continue
        cell_metrics = cell.get("cell_metrics", {})
        if not isinstance(cell_metrics, dict):
            continue

        pci = cell_metrics.get("pci", cell.get("pci", idx))
        labels = {"pci": str(pci)}

        set_if_number(store, "srsran_sched_error_indication_count", labels, cell_metrics.get("error_indication_count"))
        set_if_number(store, "srsran_sched_average_latency_us", labels, cell_metrics.get("average_latency"))
        set_if_number(store, "srsran_sched_max_latency_us", labels, cell_metrics.get("max_latency"))
        set_if_number(store, "srsran_sched_late_dl_harqs", labels, cell_metrics.get("late_dl_harqs"))
        set_if_number(store, "srsran_sched_late_ul_harqs", labels, cell_metrics.get("late_ul_harqs"))
        set_if_number(store, "srsran_sched_avg_prach_delay_slots", labels, cell_metrics.get("avg_prach_delay"))
        set_if_number(store, "srsran_sched_nof_failed_pdcch_allocs", labels, cell_metrics.get("nof_failed_pdcch_allocs"))
        set_if_number(store, "srsran_sched_nof_failed_uci_allocs", labels, cell_metrics.get("nof_failed_uci_allocs"))

        # Active UE count
        ue_list = cell.get("ue_list", [])
        if isinstance(ue_list, list):
            store.set("srsran_active_ues", float(len(ue_list)), labels)
            for ue in ue_list:
                if not isinstance(ue, dict):
                    continue
                ue_pci = str(ue.get("pci", pci))
                ue_rnti = str(ue.get("rnti", "unknown"))
                ue_labels = {"pci": ue_pci, "rnti": ue_rnti}
                set_if_number(store, "srsran_ue_dl_nof_ok", ue_labels, ue.get("dl_nof_ok"))
                set_if_number(store, "srsran_ue_dl_nof_nok", ue_labels, ue.get("dl_nof_nok"))
                set_if_number(store, "srsran_ue_ul_nof_ok", ue_labels, ue.get("ul_nof_ok"))
                set_if_number(store, "srsran_ue_ul_nof_nok", ue_labels, ue.get("ul_nof_nok"))
                set_if_number(store, "srsran_ue_dl_brate_bps", ue_labels, ue.get("dl_brate"))
                set_if_number(store, "srsran_ue_ul_brate_bps", ue_labels, ue.get("ul_brate"))
                set_if_number(store, "srsran_ue_dl_mcs", ue_labels, ue.get("dl_mcs"))
                set_if_number(store, "srsran_ue_ul_mcs", ue_labels, ue.get("ul_mcs"))
                set_if_number(store, "srsran_ue_cqi", ue_labels, ue.get("cqi"))
                set_if_number(store, "srsran_ue_ri", ue_labels, ue.get("dl_ri"))
                set_if_number(store, "srsran_ue_pucch_snr_db", ue_labels, ue.get("pucch_snr_db"))
                set_if_number(store, "srsran_ue_pusch_snr_db", ue_labels, ue.get("pusch_snr_db"))
                set_if_number(store, "srsran_ue_bsr", ue_labels, ue.get("bsr"))
                set_if_number(store, "srsran_ue_last_phr", ue_labels, ue.get("last_phr"))


def parse_mac_metrics(store: MetricStore, message: dict) -> None:
    du = message.get("du")
    if not isinstance(du, dict):
        return

    du_high = du.get("du_high")
    if not isinstance(du_high, dict):
        return

    mac = du_high.get("mac")
    if not isinstance(mac, dict):
        return

    dl_cells = mac.get("dl")
    if not isinstance(dl_cells, list):
        return

    for idx, entry in enumerate(dl_cells):
        if not isinstance(entry, dict):
            continue

        pci = entry.get("pci", idx)
        labels = {"pci": str(pci)}

        set_if_number(store, "srsran_mac_dl_average_latency_us", labels, entry.get("average_latency_us"))
        set_if_number(store, "srsran_mac_dl_max_latency_us", labels, entry.get("max_latency_us"))
        set_if_number(store, "srsran_mac_dl_min_latency_us", labels, entry.get("min_latency_us"))
        set_if_number(store, "srsran_mac_dl_cpu_usage_percent", labels, entry.get("cpu_usage_percent"))


def process_message(store: MetricStore, message: dict) -> None:
    parse_scheduler_metrics(store, message)
    parse_mac_metrics(store, message)


# ---------------------------------------------------------------------------
# Minimal RFC 6455 WebSocket client (stdlib only, no external dependencies)
# ---------------------------------------------------------------------------

_WS_MAGIC = b"258EAFA5-E914-47DA-95CA-5AB5DC525DA0"
_OP_TEXT = 0x1
_OP_CLOSE = 0x8
_OP_PING = 0x9
_OP_PONG = 0xA


def _ws_connect(url: str) -> socket.socket:
    """Perform the WebSocket opening handshake and return the connected socket."""
    parsed = urlparse(url)
    host = parsed.hostname or "127.0.0.1"
    port = parsed.port or (443 if parsed.scheme == "wss" else 80)
    path = parsed.path or "/"

    sock = socket.create_connection((host, port), timeout=10)
    sock.settimeout(None)

    key = base64.b64encode(os.urandom(16)).decode()
    request = (
        f"GET {path} HTTP/1.1\r\n"
        f"Host: {host}:{port}\r\n"
        f"Upgrade: websocket\r\n"
        f"Connection: Upgrade\r\n"
        f"Sec-WebSocket-Key: {key}\r\n"
        f"Sec-WebSocket-Version: 13\r\n"
        f"\r\n"
    )
    sock.sendall(request.encode())

    # Read the HTTP response headers.
    response = b""
    while b"\r\n\r\n" not in response:
        chunk = sock.recv(4096)
        if not chunk:
            raise ConnectionError("Connection closed during handshake")
        response += chunk

    status_line = response.split(b"\r\n", 1)[0]
    if b"101" not in status_line:
        raise ConnectionError(f"WebSocket handshake failed: {status_line.decode(errors='replace')}")

    return sock


def _ws_send_text(sock: socket.socket, text: str) -> None:
    """Send a masked text frame (RFC 6455 §5.1: client MUST mask)."""
    payload = text.encode()
    mask = os.urandom(4)

    # Build frame header.
    header = bytearray()
    header.append(0x80 | _OP_TEXT)  # FIN + opcode
    length = len(payload)
    if length < 126:
        header.append(0x80 | length)  # MASK bit set
    elif length < 65536:
        header.append(0x80 | 126)
        header.extend(struct.pack("!H", length))
    else:
        header.append(0x80 | 127)
        header.extend(struct.pack("!Q", length))
    header.extend(mask)

    masked = bytearray(b ^ mask[i % 4] for i, b in enumerate(payload))
    sock.sendall(bytes(header) + bytes(masked))


def _ws_send_pong(sock: socket.socket, data: bytes) -> None:
    """Send a masked pong frame."""
    mask = os.urandom(4)
    header = bytearray()
    header.append(0x80 | _OP_PONG)
    header.append(0x80 | len(data))
    header.extend(mask)
    masked = bytearray(b ^ mask[i % 4] for i, b in enumerate(data))
    sock.sendall(bytes(header) + bytes(masked))


def _recv_exactly(sock: socket.socket, n: int) -> bytes:
    """Read exactly n bytes from sock."""
    buf = bytearray()
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise ConnectionError("Connection closed")
        buf.extend(chunk)
    return bytes(buf)


def _ws_recv_frame(sock: socket.socket) -> tuple:
    """Read one WebSocket frame. Returns (opcode, payload_bytes)."""
    head = _recv_exactly(sock, 2)
    opcode = head[0] & 0x0F
    is_masked = bool(head[1] & 0x80)
    length = head[1] & 0x7F

    if length == 126:
        length = struct.unpack("!H", _recv_exactly(sock, 2))[0]
    elif length == 127:
        length = struct.unpack("!Q", _recv_exactly(sock, 8))[0]

    mask = _recv_exactly(sock, 4) if is_masked else None
    payload = bytearray(_recv_exactly(sock, length))
    if mask:
        for i in range(len(payload)):
            payload[i] ^= mask[i % 4]
    return opcode, bytes(payload)


def ws_receiver(store: MetricStore, ws_url: str, reconnect_delay: float) -> None:
    """Connect to srsRAN WebSocket, subscribe to metrics, and parse incoming JSON."""
    while True:
        store.inc("srsran_exporter_ws_connections_total")
        print(f"Connecting to {ws_url} ...", flush=True)
        sock = None
        try:
            sock = _ws_connect(ws_url)
            _ws_send_text(sock, json.dumps({"cmd": "metrics_subscribe"}))
            store.set("srsran_exporter_ws_connected", 1)
            print("Subscribed to metrics.", flush=True)

            while True:
                opcode, payload = _ws_recv_frame(sock)
                if opcode == _OP_TEXT:
                    _handle_ws_message(store, payload.decode("utf-8", errors="replace"))
                elif opcode == _OP_PING:
                    _ws_send_pong(sock, payload)
                elif opcode == _OP_CLOSE:
                    print("Server sent close frame.", flush=True)
                    break
        except Exception as exc:
            print(f"WebSocket error: {exc}", flush=True)
        finally:
            if sock:
                with suppress(OSError):
                    sock.close()
        store.set("srsran_exporter_ws_connected", 0)
        print(f"Reconnecting in {reconnect_delay}s ...", flush=True)
        time.sleep(reconnect_delay)


def _handle_ws_message(store: MetricStore, raw: str) -> None:
    store.inc("srsran_exporter_ws_messages_total")
    store.set("srsran_exporter_last_message_unixtime", time.time())
    with suppress(json.JSONDecodeError):
        message = json.loads(raw)
        # Skip command responses (e.g. {"cmd": "metrics_subscribe", ...})
        if isinstance(message, dict) and "cmd" not in message:
            process_message(store, message)
            return
    store.inc("srsran_exporter_ws_parse_errors_total")


def start_http_server(store: MetricStore, host: str, port: int) -> None:
    handler_cls = type("BoundMetricsHandler", (MetricsHandler,), {})
    handler_cls.store = store
    server = HTTPServer((host, port), handler_cls)
    server.serve_forever()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Expose srsRAN gNB JSON metrics as Prometheus metrics.",
        epilog=(
            "srsRAN must have remote_control enabled with metrics subscription.\n"
            "Example gnb.yaml:\n"
            "  remote_control:\n"
            "    enabled: true\n"
            "    bind_addr: 0.0.0.0\n"
            "    port: 8001\n"
            "  metrics:\n"
            "    enable_json: true\n"
        ),
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--ws-url",
        default="ws://127.0.0.1:8001",
        help="WebSocket URL of the srsRAN remote control server (default: ws://127.0.0.1:8001).",
    )
    parser.add_argument(
        "--reconnect-delay",
        type=float,
        default=5.0,
        help="Seconds to wait before reconnecting after a WebSocket disconnect (default: 5).",
    )
    parser.add_argument("--listen-host", default="127.0.0.1", help="HTTP listen host for Prometheus scraping.")
    parser.add_argument("--listen-port", type=int, default=9808, help="HTTP listen port for Prometheus scraping.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    store = MetricStore()

    receiver = threading.Thread(
        target=ws_receiver,
        args=(store, args.ws_url, args.reconnect_delay),
        daemon=True,
    )
    receiver.start()

    print(f"Serving Prometheus metrics on http://{args.listen_host}:{args.listen_port}/metrics", flush=True)
    start_http_server(store, args.listen_host, args.listen_port)


if __name__ == "__main__":
    main()
