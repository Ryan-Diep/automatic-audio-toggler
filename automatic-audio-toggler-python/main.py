import hid
import time
import subprocess
import json
import os
import sys

CONFIG_TEMPLATE = {
    "vendor_id": "0x1234",
    "product_id": "0x5678",
    "speaker_name": "Your Speaker Name Here",
    "headset_name": "Your Headset Name Here",
    "nircmd_path": "C:\\path\\to\\nircmd.exe"
}

def load_config():
    config_path = os.path.join(os.path.dirname(os.path.abspath(sys.argv[0])), "config.json")

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

def switch_audio(nircmd_path, device_name):
    subprocess.run([nircmd_path, "setdefaultsounddevice", device_name, "1"], shell=True)
    subprocess.run([nircmd_path, "setdefaultsounddevice", device_name, "2"], shell=True)

def main():
    cfg = load_config()

    target_path = None
    for device_info in hid.enumerate(cfg["vendor_id"], cfg["product_id"]):
        if device_info['usage_page'] == 65300:
            target_path = device_info['path']
            break

    if not target_path:
        sys.exit(0)

    try:
        hid_device = hid.device()
        hid_device.open_path(target_path)
        hid_device.set_nonblocking(1)
    except Exception:
        sys.exit(0)

    current_state = "unknown"

    try:
        while True:
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