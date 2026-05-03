package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/joho/godotenv"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/sstallion/go-hid"
	"golang.org/x/sys/windows"
)

// IPolicyConfig COM interface (undocumented Windows API)
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

type Config struct {
	VendorID    uint16
	ProductID   uint16
	SpeakerName string
	HeadsetName string
}

func loadConfig() Config {
	if err := godotenv.Load("../.env"); err != nil {
		log.Fatal("Error loading .env file")
	}

	vendorID, err := strconv.ParseUint(os.Getenv("VENDOR_ID"), 0, 16)
	if err != nil {
		log.Fatalf("Invalid VENDOR_ID: %v", err)
	}
	productID, err := strconv.ParseUint(os.Getenv("PRODUCT_ID"), 0, 16)
	if err != nil {
		log.Fatalf("Invalid PRODUCT_ID: %v", err)
	}

	return Config{
		VendorID:    uint16(vendorID),
		ProductID:   uint16(productID),
		SpeakerName: os.Getenv("SPEAKER_NAME"),
		HeadsetName: os.Getenv("HEADSET_NAME"),
	}
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

func setDefaultAudioDevice(deviceName string) error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 0x00000001 {
			return fmt.Errorf("COM init failed: %w", err)
		}
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
			fmt.Printf("[>>>] Default audio device set to: %s\n", name)
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
	cfg := loadConfig()

	path, err := findDevicePath(cfg.VendorID, cfg.ProductID)
	if err != nil {
		log.Fatalf("[-] %v", err)
	}

	dev, err := hid.OpenPath(path)
	if err != nil {
		log.Fatalf("Failed to open device: %v", err)
	}
	defer dev.Close()

	if err := dev.SetNonblock(true); err != nil {
		log.Fatalf("Failed to set non-blocking mode: %v", err)
	}

	fmt.Println("[+] Connected! Listening for power events...")

	stateChange := make(chan string, 1)

	go func() {
		for deviceName := range stateChange {
			if err := setDefaultAudioDevice(deviceName); err != nil {
				log.Printf("[!] Audio switch failed: %v", err)
			}
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
				fmt.Println("[+] Power ON detected!")
				currentState = "on"
				select {
				case <-stateChange:
				default:
				}
				stateChange <- cfg.HeadsetName

			case buf[13] == 0x0 && currentState != "off":
				fmt.Println("[-] Power OFF detected!")
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
