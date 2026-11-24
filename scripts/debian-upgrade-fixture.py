"""Disposable CI fixture for the Phase 3 to Phase 4.1 Debian upgrade test."""
import json
import os
from pathlib import Path
import socket
import sqlite3
import subprocess
import sys
import threading
import time
from urllib.request import Request, urlopen

if os.environ.get("GITHUB_ACTIONS") != "true":
    raise SystemExit("Run only from the disposable Debian acceptance script")

BASE = "http://127.0.0.1:8080"
CONFIG = Path("/etc/udphttp-proxy/config.yaml")
DB = Path("/var/lib/udphttp-proxy/udphttp-proxy.db")
CHANNEL = {
    "id": "upgrade-check", "name": "Upgrade check", "enabled": True,
    "source_type": "unicast", "source_address": "127.0.0.1",
    "source_port": 15231, "interface": "127.0.0.1", "startup": "on_demand"
}

def api(path, method="GET", data=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["X-CSRF-Token"] = token
    with urlopen(Request(BASE + path, data=json.dumps(data).encode() if data is not None else None,
                         headers=headers, method=method), timeout=8) as response:
        return json.load(response)

def ready():
    for _ in range(40):
        try:
            if api("/health")["storage_ready"]:
                return
        except OSError:
            time.sleep(.25)
    raise AssertionError("daemon did not become ready")

def verify_playback(channel=CHANNEL, via="127.0.0.1", multicast=False):
    stop = threading.Event()
    def send():
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
            sock.bind((via, 0))
            if multicast:
                sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_IF, socket.inet_aton(via))
                sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_TTL, 0)
                sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_LOOP, 1)
            address = channel["source_address"] if multicast else via
            cc = 0
            while not stop.wait(.02):
                payload = bytearray()
                for _ in range(7):
                    payload.extend(bytes([0x47, 1, 0, 0x10 | cc]) + bytes(184))
                    cc = (cc + 1) % 16
                sock.sendto(payload, (address, channel["source_port"]))
    worker = threading.Thread(target=send)
    worker.start()
    try:
        with urlopen(f"http://127.0.0.1:7781/live/{channel['id']}.ts", timeout=10) as response:
            data = response.read(1316)
            assert response.headers.get_content_type() == "video/mp2t"
            assert len(data) == 1316 and all(data[i] == 0x47 for i in range(0, len(data), 188))
    finally:
        stop.set()
        worker.join()

ready()
if sys.argv[1] == "seed":
    token = api("/api/v1/csrf")["token"]
    view = api("/api/v1/settings")
    desired = view["settings"]
    desired["dashboard"]["name"] = "Debian upgrade preservation"
    desired["streaming"].update(allow_loopback_unicast=True,
                                allowed_unicast_sources=["127.0.0.1"],
                                allowed_client_networks=["127.0.0.0/8"])
    api("/api/v1/settings", "PUT", {"revision": view["revision"], "settings": desired, "confirm": True}, token)
    # Policy becomes active after restart. Use the check phase to create the channel.
    with CONFIG.open("a", encoding="utf-8") as file:
        file.write("\n# Debian conffile preservation fixture\n")
elif sys.argv[1] == "multicast":
    links = json.loads(subprocess.check_output(["ip", "-j", "-4", "address", "show", "up"], text=True))
    name, address = next(
        (link["ifname"], item["local"])
        for link in links if "MULTICAST" in link.get("flags", [])
        for item in link["addr_info"] if item["scope"] == "global"
    )
    channel = dict(CHANNEL, id="unit-multicast", name="Systemd multicast fixture",
                   source_type="multicast", source_address="239.255.42.1", source_port=15232, interface=name)
    token = api("/api/v1/csrf")["token"]
    view = api("/api/v1/settings")
    desired = view["settings"]
    desired["streaming"]["allowed_multicast_groups"] = [channel["source_address"]]
    api("/api/v1/settings", "PUT", {"revision": view["revision"], "settings": desired, "confirm": True}, token)
    subprocess.run(["systemctl", "restart", "udphttp-proxy"], check=True)
    ready()
    api("/api/v1/streams", "POST", channel, api("/api/v1/csrf")["token"])
    verify_playback(channel, address, multicast=True)
    print("Multicast playback passed inside the hardened systemd unit")
elif sys.argv[1] == "migration-failure":
    candidate, directory = sys.argv[2:]
    backup = Path(directory) / "migration-check.db"
    with sqlite3.connect(f"file:{DB}?mode=ro", uri=True) as original, sqlite3.connect(backup) as clone:
        original.backup(clone)
        clone.execute("CREATE TRIGGER fail_migration BEFORE UPDATE ON channels BEGIN SELECT RAISE(ABORT, 'injected migration failure'); END")
        rows = clone.execute("SELECT * FROM channels").fetchall()
    config = Path(directory) / "migration-check.yaml"
    config.write_text(json.dumps({"storage": {"path": str(backup)}}), encoding="utf-8")
    result = subprocess.run([candidate, "serve", "--config", str(config)], capture_output=True, text=True, timeout=10)
    assert result.returncode != 0 and "injected migration failure" in result.stderr, result.stderr
    with sqlite3.connect(backup) as clone:
        assert clone.execute("PRAGMA user_version").fetchone() == (3,)
        assert clone.execute("SELECT * FROM channels").fetchall() == rows
        clone.execute("DROP TRIGGER fail_migration")
    # The real packaged upgrade below proceeds with the untouched service DB.
    print("Injected schema migration failure retained schema 3 and original channel data")
else:
    schema = int(sys.argv[2])
    with sqlite3.connect(f"file:{DB}?mode=ro", uri=True) as connection:
        assert connection.execute("PRAGMA user_version").fetchone() == (schema,)
    view = api("/api/v1/settings")
    assert view["active"]["dashboard"]["name"] == "Debian upgrade preservation"
    assert view["active"]["network"]["http_port"] == 7781
    assert view["active"]["streaming"]["allowed_client_networks"] == ["127.0.0.0/8"]
    streams = api("/api/v1/streams")["streams"]
    if (schema == 3 or sys.argv[1] == "create") and not streams:
        api("/api/v1/streams", "POST", CHANNEL, api("/api/v1/csrf")["token"])
        streams = api("/api/v1/streams")["streams"]
    assert len(streams) == 1
    assert all(streams[0]["channel"][key] == value for key, value in CHANNEL.items())
    if schema == 4:
        assert streams[0]["channel"]["preferred_playback_port"] == "primary"
    verify_playback()
