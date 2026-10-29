# Automatic Audio Toggler

## Overview
Automatic Audio Toggler is an application that automatically switches your default Windows audio playback device based on whether a specific USB device (such as a wireless headset) is connected and active. 

It monitors a USB HID (Human Interface Device) channel for a configured Vendor ID (VID) and Product ID (PID). A connected USB receiver alone does not mean the wireless headset is on.

For the **Razer BlackShark V2 HyperSpeed wireless receiver (`1532:0565`)**, both implementations query the receiver's wireless link every 250 ms (four checks per second). A query waits at most 150 ms for its reply, returning as soon as the reply arrives. HID reads block while waiting instead of repeatedly checking for data, keeping idle CPU use low. Detection normally takes up to one polling interval plus the receiver's response and Windows audio-switching time. There are three switching rules:

1. Headset turns on or reconnects: select the headphones.
2. Manual output selections stick until the next power-on/reconnection, except that selecting an offline headset is corrected by rule 3.
3. Whenever the headset is confirmed off or asleep and headphones are selected: switch to the configured speakers (ASUS Monitor in the local config). Other outputs are left alone.

A missing or invalid reply leaves the output and last confirmed connection result unchanged; silence alone is not treated as power-off.

The app runs silently in the background without routine logging or a command prompt window. Failed status checks are retried at the normal polling interval. The status check detects headset power-off and automatic sleep even while its USB receiver remains plugged in.

The query uses report ID `0x02`, usage page `0xFF14` (65300), command `0x20`, and receiver domain `0x00`. Frame format and checksums are documented in the community [BlackShark V2 HyperSpeed protocol reference](https://github.com/bbesli/Synapse-for-Linux/blob/main/docs/PROTOCOL.md). Polling is enabled only for this VID/PID. Other headsets retain the original event-only detection, which can miss automatic shutdown. BlackShark V2 Pro and other Razer variants may use different protocols.

On the first valid startup check, an online headset is selected if the monitor is currently selected; other outputs are preserved. An offline headset selected as output is corrected at startup and on every successful offline check. After startup, a manual monitor selection while the headset stays on is preserved until the next power-on/reconnection. A failed power-on switch is not replayed over a later manual selection; offline correction is retried while headphones remain selected. The receiver reports wireless connectivity, so going out of range and reconnecting have the same effect as power-off and power-on. A complete off/on cycle between polls cannot be detected.

After changing the source, run the updated Python script or rebuild the executable; existing `.exe` files do not pick up source edits. For Go, build the whole package with `go build -ldflags="-H windowsgui" -o audio-toggler-go.exe .` inside its directory. For Python, build with `python -m PyInstaller --onefile --noconsole --name audio-toggler-python main.py`; the executable is created in `dist/`. These flags prevent a command prompt window when launching either executable. Run the Python source with `pythonw main.py` for the same background behavior without packaging it.

For Python, run `python main.py --status` in a terminal to query the link once without changing audio. Try it while the headset is on, then after it automatically turns off: it should print `connected` and `disconnected`, respectively. Query failures report `unknown` and exit with an error. The packaged background executable has no console for diagnostic output. Keep `audio-toggler-python.json` beside the executable; its `nircmd_path` must point to an existing NirCmd executable for audio switching.

## Implementations

### Python Version (Prototype)
The `automatic-audio-toggler-python` directory contains a prototype version of this tool. 

**Security Note:** This Python prototype is considered vulnerable and less robust because it relies on executing an external third-party utility called `nircmd` to perform the actual audio switching. 

## Setup Instructions (Python Prototype)

### 1. Install Nircmd
The Python prototype requires `nircmd.exe` to interact with Windows audio settings.
1. Download `nircmd` from the official NirSoft website.
2. Extract the downloaded ZIP file.
3. Save `nircmd.exe` to a known location on your computer (you will need this path for the configuration).

### 2. Install HIDAPI DLL
The Python script needs the `hidapi.dll` to interface with USB devices on Windows.
1. Download the pre-compiled `hidapi.dll` (ensure you get the architecture that matches your Python installation, typically x64) from the official `libusb/hidapi` GitHub repository releases.
2. Move the downloaded `hidapi.dll` directly into the `automatic-audio-toggler-python` folder.

### 3. Configuration File
Instead of an `.env` file, this project uses a `config.json` file for configuration, which is automatically generated when you first run the compiled binary or the Python script. If the `config.json` file does not exist, it will be created with placeholder values.

You will need to edit this `config.json` file to match your system's specifics.

#### How to find your VID and PID
1. Open Windows **Device Manager**.
2. Expand the **Human Interface Devices** or **Sound, video and game controllers** section.
3. Find your headset (or its USB receiver), right-click it, and select **Properties**.
4. Navigate to the **Details** tab.
5. Under the "Property" dropdown, select **Hardware Ids**.
6. Look at the values provided. You will see something like `USB\VID_1038&PID_12B6...`. 
7. Your Vendor ID (VID) is the 4-character code after `VID_` (e.g., `1038`).
8. Your Product ID (PID) is the 4-character code after `PID_` (e.g., `12B6`).
9. In the `config.json` file, update the `vendor_id` and `product_id` fields. These should be entered as hexadecimal strings (e.g., `"vendor_id": "0x1038"`).

#### How to find Headset and Speaker Names
1. Open the classic Windows **Sound Control Panel**. (Press `Win + R`, type `mmsys.cpl`, and hit Enter).
2. Look at the **Playback** tab.
3. Identify the exact names of your devices as they appear in bold text (e.g., "Speakers" or "Headset Earphone").
4. Enter these **exact names** into the `audio-toggler-go.json` or `audio-toggler-python.json` file for the `headset_name` and `speaker_name` fields.

#### Example `audio-toggler-python.json` file
```json
{
  "vendor_id": "0xYOUR_VID_HERE",
  "product_id": "0xYOUR_PID_HERE",
  "nircmd_path": "C:\\Path\\To\\Your\\nircmd.exe",
  "headset_name": "Your Headset Name",
  "speaker_name": "Your Speaker Name"
}
```
