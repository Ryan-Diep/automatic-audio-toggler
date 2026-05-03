package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sstallion/go-hid"
	"golang.org/x/sys/windows"
)

var (
	IID_IPolicyConfig  = ole.NewGUID("{f8679f50-850a-41cf-9c72-430f290290c8}")
	CLSID_PolicyConfig = ole.NewGUID("{870af99c-171d-4f9e-af0d-e63df40c2bc9}")
)

type IPolicyConfig struct {
	ole.IUnknown
}

type IPolicyConfigVtbl struct {
	ole.IUnknownVtbl
	GetMixFormat          uintptr
	GetDeviceFormat       uintptr
	ResetDeviceFormat     uintptr
	SetDeviceFormat       uintptr
	GetProcessingPeriod   uintptr
	SetProcessingPeriod   uintptr
	GetShareMode          uintptr
	SetShareMode          uintptr
	GetPropertyValue      uintptr
	SetPropertyValue      uintptr
	SetDefaultEndpoint    uintptr
	SetEndpointVisibility uintptr
}

func (pc *IPolicyConfig) VTable() *IPolicyConfigVtbl {
	return (*IPolicyConfigVtbl)(unsafe.Pointer(pc.RawVTable))
}

func (pc *IPolicyConfig) SetDefaultEndpoint(deviceID string, role wca.ERole) error {
	deviceIDPtr, err := windows.UTF16PtrFromString(deviceID)
	if err != nil {
		return err
	}
	hr, _, _ := syscall.SyscallN(
		pc.VTable().SetDefaultEndpoint,
		uintptr(unsafe.Pointer(pc)),
		uintptr(unsafe.Pointer(deviceIDPtr)),
		uintptr(role),
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

type HexUint16 uint16

func (h *HexUint16) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		val, err := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 16)
		if err != nil {
			return fmt.Errorf("invalid hex value: %s", s)
		}
		*h = HexUint16(val)
		return nil
	}

	var n uint16
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*h = HexUint16(n)
	return nil
}

type Config struct {
	VendorID    HexUint16 `json:"vendor_id"`
	ProductID   HexUint16 `json:"product_id"`
	SpeakerName string    `json:"speaker_name"`
	HeadsetName string    `json:"headset_name"`
}

func loadConfig() (Config, error) {
	exePath, err := os.Executable()
	if err != nil {
		return Config{}, err
	}
	configPath := filepath.Join(filepath.Dir(exePath), "audio-toggler-go.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		templateJSON := []byte(`{
  "vendor_id": "0x1234",
  "product_id": "0x5678",
  "speaker_name": "Your Speaker Name Here",
  "headset_name": "Your Headset Name Here"
}`)
		os.WriteFile(configPath, templateJSON, 0644)
		return Config{}, fmt.Errorf("config not set up")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	if cfg.HeadsetName == "Your Headset Name Here" || cfg.SpeakerName == "Your Speaker Name Here" {
		return Config{}, fmt.Errorf("config not set up")
	}

	return cfg, nil
}

func findDevicePath(vendorID, productID uint16) (string, error) {
	var targetPath string
	err := hid.Enumerate(vendorID, productID, func(info *hid.DeviceInfo) error {
		if info.UsagePage == 65300 {
			targetPath = info.Path
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("enumeration failed: %w", err)
	}
	if targetPath == "" {
		return "", fmt.Errorf("telemetry channel not found")
	}
	return targetPath, nil
}

func comInit() error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 0x00000001 {
			return fmt.Errorf("COM init failed: %w", err)
		}
	}
	return nil
}

func correctStartupState(cfg Config) error {
	if err := comInit(); err != nil {
		return err
	}
	defer ole.CoUninitialize()

	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		return err
	}
	defer enumerator.Release()

	var device *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &device); err != nil {
		return err
	}
	defer device.Release()

	var ps *wca.IPropertyStore
	if err := device.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return err
	}
	defer ps.Release()

	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return err
	}

	if strings.Contains(pv.String(), cfg.HeadsetName) {
		return setDefaultAudioDevice(cfg.SpeakerName)
	}

	return nil
}

func setDefaultAudioDevice(deviceName string) error {
	if err := comInit(); err != nil {
		return err
	}
	defer ole.CoUninitialize()

	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		return fmt.Errorf("failed to create enumerator: %w", err)
	}
	defer enumerator.Release()

	var collection *wca.IMMDeviceCollection
	if err := enumerator.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &collection); err != nil {
		return fmt.Errorf("failed to enumerate endpoints: %w", err)
	}
	defer collection.Release()

	var count uint32
	if err := collection.GetCount(&count); err != nil {
		return fmt.Errorf("failed to get device count: %w", err)
	}

	for i := uint32(0); i < count; i++ {
		var device *wca.IMMDevice
		if err := collection.Item(i, &device); err != nil {
			continue
		}
		defer device.Release()

		var ps *wca.IPropertyStore
		if err := device.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
			continue
		}
		defer ps.Release()

		var pv wca.PROPVARIANT
		if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
			continue
		}

		name := pv.String()
		if strings.Contains(name, deviceName) {
			var id string
			if err := device.GetId(&id); err != nil {
				return fmt.Errorf("failed to get device ID: %w", err)
			}
			if err := setDefaultDevice(id); err != nil {
				return fmt.Errorf("failed to set default device: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("device not found: %s", deviceName)
}

func setDefaultDevice(deviceID string) error {
	var pc *IPolicyConfig
	if err := wca.CoCreateInstance(
		CLSID_PolicyConfig,
		0,
		wca.CLSCTX_ALL,
		IID_IPolicyConfig,
		&pc,
	); err != nil {
		return fmt.Errorf("failed to create IPolicyConfig: %w", err)
	}
	defer pc.Release()

	if err := pc.SetDefaultEndpoint(deviceID, wca.EConsole); err != nil {
		return fmt.Errorf("failed to set console default: %w", err)
	}
	if err := pc.SetDefaultEndpoint(deviceID, wca.ECommunications); err != nil {
		return fmt.Errorf("failed to set comms default: %w", err)
	}
	return nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		os.Exit(0)
	}

	correctStartupState(cfg)

	path, err := findDevicePath(uint16(cfg.VendorID), uint16(cfg.ProductID))
	if err != nil {
		os.Exit(0)
	}

	dev, err := hid.OpenPath(path)
	if err != nil {
		os.Exit(0)
	}
	defer dev.Close()

	if err := dev.SetNonblock(true); err != nil {
		os.Exit(0)
	}

	stateChange := make(chan string, 1)

	go func() {
		for deviceName := range stateChange {
			setDefaultAudioDevice(deviceName)
		}
	}()

	buf := make([]byte, 64)
	currentState := "unknown"

	for {
		n, err := dev.Read(buf)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		if n > 0 && len(buf) >= 14 && buf[10] == 0x20 {
			switch {
			case buf[13] == 0x1 && currentState != "on":
				currentState = "on"
				select {
				case <-stateChange:
				default:
				}
				stateChange <- cfg.HeadsetName

			case buf[13] == 0x0 && currentState != "off":
				currentState = "off"
				select {
				case <-stateChange:
				default:
				}
				stateChange <- cfg.SpeakerName
			}
		}

		time.Sleep(50 * time.Millisecond)
	}
}
