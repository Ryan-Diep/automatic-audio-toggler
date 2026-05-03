# Automatic Audio Toggler

## Overview
Automatic Audio Toggler is an application that automatically switches your default Windows audio playback device based on whether a specific USB device (such as a wireless headset) is connected and active. 

It works by continuously monitoring USB HID (Human Interface Device) connections for a specific Vendor ID (VID) and Product ID (PID). When the target device is detected, it switches the audio output to your headset. When the device is disconnected or turned off, it switches the output back to your speakers.

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