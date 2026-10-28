import hid
import time
import subprocess
import json
import os
import sys
import argparse
from pycaw.pycaw import AudioUtilities
from headset_status import POLL_INTERVAL, query_link, supports_link_query

CONFIG_TEMPLATE = {
    "vendor_id": "0x1234",
    "product_id": "0x5678",
    "speaker_name": "Your Speaker Name Here",
    "headset_name": "Your Headset Name Here",
    "nircmd_path": "C:\\path\\to\\nircmd.exe"
}

def load_config():
    config_path = os.path.join(os.path.dirname(os.path.abspath(sys.argv[0])), "audio-toggler-python.json")

    if not os.path.exists(config_path):
        with open(config_path, "w") as f:
            json.dump(CONFIG_TEMPLATE, f, indent=2)
        sys.exit(0)

    with open(config_path, "r") as f:
        cfg = json.load(f)

    if cfg.get("headset_name") == "Your Headset Name Here" or cfg.get("speaker_name") == "Your Speaker Name Here":
        sys.exit(0)

    return {
        "vendor_id": int(cfg["vendor_id"], 16) if isinstance(cfg["vendor_id"], str) else cfg["vendor_id"],
        "product_id": int(cfg["product_id"], 16) if isinstance(cfg["product_id"], str) else cfg["product_id"],
        "nircmd_path": cfg["nircmd_path"],
        "headset_name": cfg["headset_name"],
        "speaker_name": cfg["speaker_name"]
    }

def get_default_device_name():
    try:
        device = AudioUtilities.GetSpeakers()
        if device is None:
            return None
        return device.FriendlyName
    except Exception:
        return None

def switch_audio(nircmd_path, device_name):
    subprocess.run([nircmd_path, "setdefaultsounddevice", device_name, "1"], check=True)
    subprocess.run([nircmd_path, "setdefaultsounddevice", device_name, "2"], check=True)

def monitor_link(hid_device, cfg):
    previous_connection = None
    sequence = 0
    while True:
        started = time.monotonic()
        try:
            connected = query_link(hid_device, sequence)
            # Timeouts leave the last confirmed connection result unchanged.
            previous = previous_connection
            previous_connection = connected
            if not connected:
                correct_startup_state(cfg)
            elif previous is False:
                # A new connection overrides manual output selection once.
                switch_audio(cfg["nircmd_path"], cfg["headset_name"])
            elif previous is None:
                # On startup, select an online headset only from the monitor.
                current = get_default_device_name()
                if current and cfg["speaker_name"].lower() in current.lower():
                    switch_audio(cfg["nircmd_path"], cfg["headset_name"])
        except (OSError, subprocess.CalledProcessError) as exc:
            print(f"Headset status/switch failed: {exc}", file=sys.stderr)
        sequence = (sequence + 1) & 0x1F
        time.sleep(max(0, POLL_INTERVAL - (time.monotonic() - started)))

def correct_startup_state(cfg):
    try:
        current = get_default_device_name()
        if current and cfg["headset_name"].lower() in current.lower():
            switch_audio(cfg["nircmd_path"], cfg["speaker_name"])
    except Exception:
        pass

def main():
    parser = argparse.ArgumentParser(description="Switch audio using headset connection status")
    parser.add_argument("--status", action="store_true", help="Query the wireless link once without switching audio")
    args = parser.parse_args()
    cfg = load_config()

    polling = supports_link_query(cfg["vendor_id"], cfg["product_id"])
    if args.status and not polling:
        parser.error("--status supports only the BlackShark V2 HyperSpeed receiver (1532:0565)")
    if not polling:
        correct_startup_state(cfg)

    target_path = None
    for device_info in hid.enumerate(cfg["vendor_id"], cfg["product_id"]):
        if device_info['usage_page'] == 65300:
            target_path = device_info['path']
            break

    if not target_path:
        print("Headset receiver control channel not found", file=sys.stderr)
        sys.exit(1)

    try:
        hid_device = hid.device()
        hid_device.open_path(target_path)
        hid_device.set_nonblocking(1)
    except Exception as exc:
        print(f"Cannot open headset receiver: {exc}", file=sys.stderr)
        sys.exit(1)

    current_state = "unknown"
    last_tick = time.monotonic()

    try:
        if args.status:
            try:
                print("connected" if query_link(hid_device, 0) else "disconnected")
            except OSError as exc:
                print(f"unknown: {exc}", file=sys.stderr)
                sys.exit(1)
            return
        if polling:
            monitor_link(hid_device, cfg)
            return
        while True:
            now = time.monotonic()
            if now - last_tick > 3:
                correct_startup_state(cfg)
                current_state = "unknown"
            last_tick = now

            data = hid_device.read(64)
            if data and len(data) >= 14:
                if data[10] == 0x20:
                    if data[13] == 0x1 and current_state != "on":
                        current_state = "on"
                        switch_audio(cfg["nircmd_path"], cfg["headset_name"])
                    elif data[13] == 0x0 and current_state != "off":
                        current_state = "off"
                        switch_audio(cfg["nircmd_path"], cfg["speaker_name"])
            time.sleep(0.05)
    except KeyboardInterrupt:
        pass
    finally:
        hid_device.close()

if __name__ == "__main__":
    main()
