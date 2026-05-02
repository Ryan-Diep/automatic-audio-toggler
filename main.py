import hid
import time
import subprocess
import os
from dotenv import load_dotenv

load_dotenv()
VENDOR_ID = int(os.getenv("VENDOR_ID"), 16)
PRODUCT_ID = int(os.getenv("PRODUCT_ID"), 16)

NIRCMD_PATH = os.getenv("NIRCMD_PATH")
HEADSET_NAME = os.getenv("HEADSET_NAME")
SPEAKER_NAME = os.getenv("SPEAKER_NAME")

def switch_audio(device_name):
    print(f"\n[>>>] Switching audio to: {device_name}")
    subprocess.run([NIRCMD_PATH, "setdefaultsounddevice", device_name, "1"], shell=True)
    subprocess.run([NIRCMD_PATH, "setdefaultsounddevice", device_name, "2"], shell=True)

def main():
    print("Initializing Razer Auto-Toggler...")
    
    target_path = None
    
    for d_info in hid.enumerate(VENDOR_ID, PRODUCT_ID):
        if d_info['usage_page'] == 65300:
            target_path = d_info['path']
            break
            
    if not target_path:
        print("[-] Could not find the Razer telemetry channel.")
        return

    try:
        h = hid.device()
        h.open_path(target_path)
        h.set_nonblocking(1) 
        print("[+] Connected! Listening for Power events...")
    except Exception as e:
        print(f"[-] Connection failed: {e}")
        return

    current_state = "unknown"

    try:
        while True:
            data = h.read(64)
            if data and len(data) >= 14:
                if data[10] == 0x20:
                    if data[13] == 0x1 and current_state != "on":
                        print("[+] Power ON detected!")
                        current_state = "on"
                        switch_audio(HEADSET_NAME)
                        
                    elif data[13] == 0x0 and current_state != "off":
                        print("[-] Power OFF detected!")
                        current_state = "off"
                        switch_audio(SPEAKER_NAME)

            time.sleep(0.05)

    except KeyboardInterrupt:
        print("\nExiting script...")
    finally:
        h.close()

if __name__ == "__main__":
    main()